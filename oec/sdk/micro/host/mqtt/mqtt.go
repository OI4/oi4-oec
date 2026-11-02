// Package mqtt is the MQTT 3.1.1 client of the micro SDK host runner
// (ADR-0030 §7): the subset a device needs – CONNECT with Will and
// credentials, CONNACK, PUBLISH QoS 0 in both directions, SUBSCRIBE with
// QoS 0, PINGREQ and DISCONNECT. Standard library only; TLS is the
// caller's net.Conn (crypto/tls). Not a general MQTT library: no QoS 1/2
// publication, no persistent session, no retained messages (REQ-MQTT-015,
// REQ-MQTT-017, REQ-MQTT-018).
package mqtt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Packet types (MQTT 3.1.1 §2.2.1).
const (
	typeConnect     = 1
	typeConnack     = 2
	typePublish     = 3
	typePuback      = 4
	typeSubscribe   = 8
	typeSuback      = 9
	typePingreq     = 12
	typePingresp    = 13
	typeDisconnect  = 14
	maxRemainingLen = 268435455
)

// Options of a connection. The Will is sent with QoS 0 and retain false;
// the session is clean.
type Options struct {
	ClientID    string
	KeepAlive   uint16 // seconds; 0 disables the keep alive
	Username    *string
	Password    *string
	WillTopic   string // "" = no Will
	WillPayload []byte
	// MaxPacket bounds the size of a received packet (remaining length);
	// 0 = the protocol maximum.
	MaxPacket int
}

// ConnackError is a refused connection (return code ≠ 0).
type ConnackError byte

func (e ConnackError) Error() string {
	names := map[ConnackError]string{
		1: "unacceptable protocol version", 2: "identifier rejected", 3: "server unavailable",
		4: "bad user name or password", 5: "not authorized",
	}
	if n, ok := names[e]; ok {
		return fmt.Sprintf("CONNACK %d: %s", byte(e), n)
	}
	return fmt.Sprintf("CONNACK %d", byte(e))
}

// Client is an established connection. Publish, Subscribe, Disconnect
// and Close may be called from any goroutine; Run reads.
type Client struct {
	conn      net.Conn
	r         *bufio.Reader
	maxPacket int
	keepAlive time.Duration

	wmu     sync.Mutex // serializes writes and guards nextID, pending
	nextID  uint16
	pending map[uint16]string // SUBSCRIBE packet id → filter

	closeOnce sync.Once
	closed    chan struct{}
	pongMu    sync.Mutex
	pingOut   bool
}

// Connect sends CONNECT on conn and waits for the CONNACK within timeout.
// On error conn is closed.
func Connect(conn net.Conn, o Options, timeout time.Duration) (*Client, error) {
	c := &Client{
		conn:      conn,
		r:         bufio.NewReader(conn),
		maxPacket: o.MaxPacket,
		keepAlive: time.Duration(o.KeepAlive) * time.Second,
		pending:   map[uint16]string{},
		closed:    make(chan struct{}),
	}
	if c.maxPacket <= 0 || c.maxPacket > maxRemainingLen {
		c.maxPacket = maxRemainingLen
	}
	fail := func(err error) (*Client, error) {
		conn.Close()
		return nil, err
	}
	pkt, err := EncodeConnect(o)
	if err != nil {
		return fail(err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(pkt); err != nil {
		return fail(fmt.Errorf("CONNECT: %w", err))
	}
	typ, _, body, err := ReadPacket(c.r, c.maxPacket)
	if err != nil {
		return fail(fmt.Errorf("CONNACK: %w", err))
	}
	if typ != typeConnack || len(body) != 2 {
		return fail(fmt.Errorf("CONNACK expected, got packet type %d (%d octets)", typ, len(body)))
	}
	if body[1] != 0 {
		return fail(ConnackError(body[1]))
	}
	_ = conn.SetDeadline(time.Time{})
	if c.keepAlive > 0 {
		go c.pinger()
	}
	return c, nil
}

// EncodeConnect is the CONNECT packet of o.
func EncodeConnect(o Options) ([]byte, error) {
	for _, n := range []int{len(o.ClientID), len(o.WillTopic), len(o.WillPayload)} {
		if n > 0xffff {
			return nil, errors.New("CONNECT field longer than 65535 octets")
		}
	}
	var flags byte = 0x02 // clean session
	payload := appendString(nil, o.ClientID)
	if o.WillTopic != "" {
		flags |= 0x04 // Will flag, QoS 0, retain false
		payload = appendString(payload, o.WillTopic)
		payload = appendString(payload, string(o.WillPayload))
	}
	if o.Username != nil {
		flags |= 0x80
		payload = appendString(payload, *o.Username)
	}
	if o.Password != nil {
		if o.Username == nil {
			return nil, errors.New("a password needs a user name (MQTT 3.1.1 §3.1.2.9)")
		}
		flags |= 0x40
		payload = appendString(payload, *o.Password)
	}
	vh := []byte{0, 4, 'M', 'Q', 'T', 'T', 4, flags, byte(o.KeepAlive >> 8), byte(o.KeepAlive)}
	return packet(typeConnect<<4, append(vh, payload...))
}

// EncodePublish is a PUBLISH with QoS 0 and retain false.
func EncodePublish(topic string, payload []byte) ([]byte, error) {
	if topic == "" || len(topic) > 0xffff {
		return nil, fmt.Errorf("topic length %d", len(topic))
	}
	return packet(typePublish<<4, append(appendString(nil, topic), payload...))
}

// Publish sends a PUBLISH with QoS 0 and retain false (REQ-MQTT-017,
// REQ-MQTT-018). A write error closes the connection.
func (c *Client) Publish(topic string, payload []byte) error {
	pkt, err := EncodePublish(topic, payload)
	if err != nil {
		return err
	}
	return c.write(pkt)
}

// Subscribe sends a SUBSCRIBE of filter with QoS 0; Run ends with an
// error on a refusal in the SUBACK.
func (c *Client) Subscribe(filter string) error {
	if filter == "" || len(filter) > 0xffff {
		return fmt.Errorf("filter length %d", len(filter))
	}
	c.wmu.Lock()
	c.nextID++
	if c.nextID == 0 {
		c.nextID = 1
	}
	id := c.nextID
	c.pending[id] = filter
	c.wmu.Unlock()
	body := append([]byte{byte(id >> 8), byte(id)}, appendString(nil, filter)...)
	pkt, err := packet(typeSubscribe<<4|0x02, append(body, 0))
	if err != nil {
		return err
	}
	return c.write(pkt)
}

// Disconnect sends DISCONNECT and closes the connection (no Will).
func (c *Client) Disconnect() error {
	err := c.write([]byte{typeDisconnect << 4, 0})
	c.Close()
	return err
}

// Close closes the connection without DISCONNECT (the broker publishes
// the Will).
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.conn.Close()
	})
}

// Run reads until the connection ends and calls msg for every received
// PUBLISH (topic and payload are owned by msg). It returns the reason;
// net.ErrClosed after Close or Disconnect.
func (c *Client) Run(msg func(topic string, payload []byte)) error {
	defer c.Close()
	for {
		typ, flags, body, err := ReadPacket(c.r, c.maxPacket)
		if err != nil {
			select {
			case <-c.closed:
				return net.ErrClosed
			default:
			}
			return err
		}
		switch typ {
		case typePublish:
			topic, payload, id, err := ParsePublish(flags, body)
			if err != nil {
				return err
			}
			if flags>>1&3 == 1 {
				if err := c.write([]byte{typePuback << 4, 2, byte(id >> 8), byte(id)}); err != nil {
					return err
				}
			}
			msg(topic, payload)
		case typeSuback:
			if len(body) < 3 {
				return fmt.Errorf("SUBACK of %d octets", len(body))
			}
			id := uint16(body[0])<<8 | uint16(body[1])
			c.wmu.Lock()
			filter, ok := c.pending[id]
			delete(c.pending, id)
			c.wmu.Unlock()
			if !ok {
				return fmt.Errorf("SUBACK for unknown packet id %d", id)
			}
			if body[2] == 0x80 {
				return fmt.Errorf("SUBSCRIBE %q refused", filter)
			}
		case typePingresp:
			c.pongMu.Lock()
			c.pingOut = false
			c.pongMu.Unlock()
		default:
			return fmt.Errorf("unexpected packet type %d", typ)
		}
	}
}

// pinger sends a PINGREQ every half keep alive; a PINGREQ still without
// answer at the next one ends the connection (MQTT 3.1.1 §3.1.2.10).
func (c *Client) pinger() {
	t := time.NewTicker(c.keepAlive / 2)
	defer t.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-t.C:
		}
		c.pongMu.Lock()
		lost := c.pingOut
		c.pingOut = true
		c.pongMu.Unlock()
		if lost {
			c.Close()
			return
		}
		if c.write([]byte{typePingreq << 4, 0}) != nil {
			return
		}
	}
}

func (c *Client) write(pkt []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.conn.Write(pkt); err != nil {
		c.Close()
		return err
	}
	return nil
}

// ReadPacket reads one packet with a remaining length of at most max:
// type, flags and body.
func ReadPacket(r *bufio.Reader, max int) (byte, byte, []byte, error) {
	h, err := r.ReadByte()
	if err != nil {
		return 0, 0, nil, err
	}
	n, err := readRemaining(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if n > max {
		return 0, 0, nil, fmt.Errorf("packet of %d octets exceeds %d", n, max)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, 0, nil, err
	}
	return h >> 4, h & 0x0f, body, nil
}

// ParsePublish splits a PUBLISH body: topic, payload and (QoS > 0) the
// packet id.
func ParsePublish(flags byte, body []byte) (string, []byte, uint16, error) {
	if len(body) < 2 {
		return "", nil, 0, errors.New("PUBLISH without topic")
	}
	n := int(body[0])<<8 | int(body[1])
	if n == 0 || len(body) < 2+n {
		return "", nil, 0, errors.New("PUBLISH topic length")
	}
	topic := string(body[2 : 2+n])
	rest := body[2+n:]
	var id uint16
	switch flags >> 1 & 3 {
	case 0:
	case 1, 2:
		if len(rest) < 2 {
			return "", nil, 0, errors.New("PUBLISH without packet id")
		}
		id = uint16(rest[0])<<8 | uint16(rest[1])
		rest = rest[2:]
	default:
		return "", nil, 0, errors.New("PUBLISH with QoS 3")
	}
	return topic, rest, id, nil
}

func readRemaining(r io.ByteReader) (int, error) {
	n, mult := 0, 1
	for i := 0; i < 4; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		n += int(b&0x7f) * mult
		if b&0x80 == 0 {
			return n, nil
		}
		mult *= 128
	}
	return 0, errors.New("malformed remaining length")
}

func packet(header byte, body []byte) ([]byte, error) {
	if len(body) > maxRemainingLen {
		return nil, fmt.Errorf("packet of %d octets", len(body))
	}
	out := []byte{header}
	for n := len(body); ; {
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
	return append(out, body...), nil
}

func appendString(b []byte, s string) []byte {
	return append(append(b, byte(len(s)>>8), byte(len(s))), s...)
}
