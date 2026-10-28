package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MQTT control packet types (MQTT 3.1.1 §2.2.1, MQTT 5.0 §2.1.2).
const (
	typeConnect    = 1
	typeConnack    = 2
	typePublish    = 3
	typeDisconnect = 14
)

// maxRemaining is the largest Remaining Length of MQTT (four bytes).
const maxRemaining = 268435455

// frame is one raw MQTT control packet: the fixed header byte and the
// variable header plus payload.
type frame struct {
	header byte
	body   []byte
}

func (f frame) kind() byte { return f.header >> 4 }

// bytes encodes the frame unchanged for the relay.
func (f frame) bytes() []byte {
	out := []byte{f.header}
	n := len(f.body)
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			break
		}
	}
	return append(out, f.body...)
}

// readFrame reads one control packet. The Remaining Length is at most four
// bytes; a longer one is a protocol error.
func readFrame(r io.Reader) (frame, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return frame{}, err
	}
	f := frame{header: b[0]}
	n, mul := 0, 1
	for i := 0; ; i++ {
		if i == 4 {
			return frame{}, errors.New("malformed Remaining Length")
		}
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return frame{}, err
		}
		n += int(b[0]&0x7f) * mul
		mul *= 128
		if b[0]&0x80 == 0 {
			break
		}
	}
	f.body = make([]byte, n)
	if _, err := io.ReadFull(r, f.body); err != nil {
		return frame{}, err
	}
	return f, nil
}

// reader decodes the fields of a packet body.
type reader struct {
	b   []byte
	err error
}

func (r *reader) fail(what string) {
	if r.err == nil {
		r.err = fmt.Errorf("malformed packet: %s", what)
	}
}

func (r *reader) byte1(what string) byte {
	if r.err != nil || len(r.b) < 1 {
		r.fail(what)
		return 0
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v
}

func (r *reader) uint16(what string) uint16 {
	if r.err != nil || len(r.b) < 2 {
		r.fail(what)
		return 0
	}
	v := binary.BigEndian.Uint16(r.b)
	r.b = r.b[2:]
	return v
}

// binary reads two-byte length prefixed data (strings and binary data).
func (r *reader) binary(what string) []byte {
	n := int(r.uint16(what))
	if r.err != nil || len(r.b) < n {
		r.fail(what)
		return nil
	}
	v := r.b[:n:n]
	r.b = r.b[n:]
	return v
}

// properties skips the properties of MQTT 5.0 (Variable Byte Integer
// length).
func (r *reader) properties(what string) {
	n, mul := 0, 1
	for i := 0; ; i++ {
		if i == 4 {
			r.fail(what)
			return
		}
		b := r.byte1(what)
		if r.err != nil {
			return
		}
		n += int(b&0x7f) * mul
		mul *= 128
		if b&0x80 == 0 {
			break
		}
	}
	if len(r.b) < n {
		r.fail(what)
		return
	}
	r.b = r.b[n:]
}

// Connect is the CONNECT packet of a client as the proxy saw it. The
// password is never kept, only whether it was sent.
type Connect struct {
	ProtocolName string `json:"protocol_name"`
	Level        byte   `json:"protocol_level"` // 3: MQTT 3.1, 4: MQTT 3.1.1, 5: MQTT 5.0
	CleanSession bool   `json:"clean_session"`  // Clean Start in MQTT 5.0
	KeepAlive    uint16 `json:"keep_alive"`     // seconds
	ClientID     string `json:"client_id"`
	Will         *Will  `json:"will,omitempty"`
	Username     string `json:"username,omitempty"`
	HasUsername  bool   `json:"has_username"`
	HasPassword  bool   `json:"has_password"`
}

// Will is the Will message registered in a CONNECT.
type Will struct {
	Topic   string `json:"topic"`
	Payload []byte `json:"-"`
	QoS     byte   `json:"qos"`
	Retain  bool   `json:"retain"`
}

// parseConnect decodes a CONNECT of MQTT 3.1 (MQIsdp, level 3), 3.1.1
// (level 4) or 5.0 (level 5; the properties are skipped).
func parseConnect(f frame) (*Connect, error) {
	if f.kind() != typeConnect {
		return nil, fmt.Errorf("the first packet is of type %d, not CONNECT", f.kind())
	}
	if f.header&0x0f != 0 {
		return nil, errors.New("malformed packet: CONNECT with reserved flags set")
	}
	r := &reader{b: f.body}
	c := &Connect{ProtocolName: string(r.binary("protocol name"))}
	c.Level = r.byte1("protocol level")
	flags := r.byte1("connect flags")
	c.KeepAlive = r.uint16("keep alive")
	if r.err != nil {
		return nil, r.err
	}
	switch {
	case c.ProtocolName == "MQIsdp" && c.Level == 3, c.ProtocolName == "MQTT" && (c.Level == 4 || c.Level == 5):
	default:
		return nil, fmt.Errorf("unknown protocol %q level %d", c.ProtocolName, c.Level)
	}
	if flags&0x01 != 0 {
		return nil, errors.New("malformed packet: CONNECT with the reserved flag set")
	}
	if c.Level == 5 {
		r.properties("connect properties")
	}
	c.CleanSession = flags&0x02 != 0
	c.ClientID = string(r.binary("client identifier"))
	if flags&0x04 != 0 {
		w := &Will{QoS: flags >> 3 & 0x03, Retain: flags&0x20 != 0}
		if c.Level == 5 {
			r.properties("will properties")
		}
		w.Topic = string(r.binary("will topic"))
		w.Payload = append([]byte(nil), r.binary("will payload")...)
		c.Will = w
	} else if flags&0x38 != 0 {
		return nil, errors.New("malformed packet: Will QoS or Will Retain without Will flag")
	}
	if c.HasUsername = flags&0x80 != 0; c.HasUsername {
		c.Username = string(r.binary("user name"))
	}
	if c.HasPassword = flags&0x40 != 0; c.HasPassword {
		r.binary("password")
	}
	if r.err != nil {
		return nil, r.err
	}
	return c, nil
}

// Publish is one PUBLISH of the client.
type Publish struct {
	Topic   string `json:"topic"`
	Payload []byte `json:"-"`
	QoS     byte   `json:"qos"`
	Retain  bool   `json:"retain"`
	Dup     bool   `json:"dup,omitempty"`
}

// parsePublish decodes a PUBLISH; level is the protocol level of the
// connection (properties in MQTT 5.0).
func parsePublish(f frame, level byte) (Publish, error) {
	p := Publish{QoS: f.header >> 1 & 0x03, Retain: f.header&0x01 != 0, Dup: f.header&0x08 != 0}
	if p.QoS == 3 {
		return p, errors.New("malformed packet: PUBLISH with QoS 3")
	}
	r := &reader{b: f.body}
	p.Topic = string(r.binary("topic name"))
	if p.QoS > 0 {
		r.uint16("packet identifier")
	}
	if level == 5 {
		r.properties("publish properties")
	}
	if r.err != nil {
		return p, r.err
	}
	p.Payload = append([]byte(nil), r.b...)
	return p, nil
}

// connackCode returns the return code (MQTT 3.1.1) or reason code (MQTT
// 5.0) of a CONNACK.
func connackCode(f frame) (int, error) {
	if f.kind() != typeConnack || len(f.body) < 2 {
		return 0, fmt.Errorf("the first packet of the broker is of type %d, not CONNACK", f.kind())
	}
	return int(f.body[1]), nil
}
