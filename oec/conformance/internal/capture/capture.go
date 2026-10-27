// Package capture is the recording format of D7 monitor (DESIGN.md §3.2,
// ADR-0025 amendment C-052): a JSONL file with a header line, one line per
// received message and an optional trailer line. A record is a superset of
// the message object of D7 validate ({"topic", "payload"}), so validate reads
// a capture as a message file. The replay uses the recorded times only, so
// it is deterministic.
//
//	{"format":"oec-capture","version":1,"tool":"oec-check",...,"started":"..."}
//	{"topic":"Oi4v2/...","payload":{...},"received":"...","qos":0,"retain":false}
//	{"topic":"Oi4v2/...","payload_base64":"...","received":"...","qos":1,"retain":false}
//	{"ended":"...","messages":2}
package capture

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf8"
)

// Format and Version identify the header line.
const (
	Format  = "oec-capture"
	Version = 1
)

// Header is the first line of a capture.
type Header struct {
	Format       string `json:"format"`
	Version      int    `json:"version"`
	Tool         string `json:"tool,omitempty"`
	ToolVersion  string `json:"tool_version,omitempty"`
	Schemas      string `json:"schemas,omitempty"`
	Baseline     string `json:"baseline,omitempty"`
	Broker       string `json:"broker,omitempty"`
	Insecure     bool   `json:"insecure,omitempty"` // broker certificate not verified (--insecure)
	Subscription string `json:"subscription,omitempty"`
	SubscribeQoS int    `json:"subscribe_qos"`
	// MaxPacketSize of the Message Bus (broker.json, RIG 4.1.1), 0 if unknown.
	MaxPacketSize int       `json:"max_packet_size,omitempty"`
	Started       time.Time `json:"started"`
	Note          string    `json:"note,omitempty"`
}

// Record is one received message.
type Record struct {
	Line     int // line in the capture (1-based), set by the reader
	Topic    string
	Payload  []byte // the payload as received (JSON payloads compacted)
	Size     int    // byte length of the payload as received
	Received time.Time
	QoS      byte
	Retain   bool
}

// Trailer is the optional last line of a capture.
type Trailer struct {
	Ended    time.Time `json:"ended"`
	Messages int       `json:"messages"`
	Error    string    `json:"error,omitempty"` // why the recording stopped early (e.g. connection lost)
}

// Capture is a complete capture.
type Capture struct {
	Header  Header
	Records []Record
	Trailer *Trailer
}

// End returns the end of the observation: the trailer time, else the time
// of the last record, else the start.
func (c *Capture) End() time.Time {
	if c.Trailer != nil {
		return c.Trailer.Ended
	}
	if n := len(c.Records); n > 0 {
		return c.Records[n-1].Received
	}
	return c.Header.Started
}

type recordJSON struct {
	Topic         string          `json:"topic"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	PayloadBase64 *string         `json:"payload_base64,omitempty"`
	Size          *int            `json:"size,omitempty"`
	Received      time.Time       `json:"received"`
	QoS           byte            `json:"qos"`
	Retain        bool            `json:"retain"`
}

// IsCapture reports whether data starts with a capture header line.
func IsCapture(data []byte) bool {
	line, _, _ := bytes.Cut(bytes.TrimLeft(data, " \t\r\n"), []byte("\n"))
	var probe struct {
		Format string `json:"format"`
	}
	return json.Unmarshal(line, &probe) == nil && probe.Format == Format
}

// Read parses a capture. Unknown keys, a missing header, times that go
// backwards and anything after the trailer are errors.
func Read(r io.Reader) (*Capture, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	c := &Capture{}
	n, header := 0, false
	var last time.Time
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if c.Trailer != nil {
			return nil, fmt.Errorf("line %d: data after the trailer", n)
		}
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(line, &keys); err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		switch {
		case !header:
			if _, ok := keys["format"]; !ok {
				return nil, fmt.Errorf("line %d: no capture header (format %q)", n, Format)
			}
			if err := strict(line, &c.Header); err != nil {
				return nil, fmt.Errorf("line %d: header: %w", n, err)
			}
			if c.Header.Format != Format || c.Header.Version != Version {
				return nil, fmt.Errorf("line %d: unsupported capture %s version %d", n, c.Header.Format, c.Header.Version)
			}
			if c.Header.Started.IsZero() {
				return nil, fmt.Errorf("line %d: header without started", n)
			}
			header, last = true, c.Header.Started
		case keys["ended"] != nil:
			var t Trailer
			if err := strict(line, &t); err != nil {
				return nil, fmt.Errorf("line %d: trailer: %w", n, err)
			}
			if t.Ended.Before(last) {
				return nil, fmt.Errorf("line %d: ended %s before the last record", n, t.Ended.Format(time.RFC3339Nano))
			}
			if t.Messages != len(c.Records) {
				return nil, fmt.Errorf("line %d: trailer counts %d messages, the capture has %d", n, t.Messages, len(c.Records))
			}
			c.Trailer = &t
		default:
			rec, err := decodeRecord(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", n, err)
			}
			if rec.Received.Before(last) {
				return nil, fmt.Errorf("line %d: received %s goes back in time", n, rec.Received.Format(time.RFC3339Nano))
			}
			rec.Line, last = n, rec.Received
			c.Records = append(c.Records, rec)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !header {
		return nil, errors.New("empty capture")
	}
	return c, nil
}

func strict(line []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func decodeRecord(line []byte) (Record, error) {
	var j recordJSON
	if err := strict(line, &j); err != nil {
		return Record{}, err
	}
	r := Record{Topic: j.Topic, Received: j.Received, QoS: j.QoS, Retain: j.Retain}
	switch {
	case j.Topic == "":
		return r, errors.New("record without topic")
	case j.Received.IsZero():
		return r, errors.New("record without received")
	case j.QoS > 2:
		return r, fmt.Errorf("qos %d", j.QoS)
	case j.Payload != nil && j.PayloadBase64 != nil:
		return r, errors.New("payload and payload_base64 together")
	case j.PayloadBase64 != nil:
		b, err := base64.StdEncoding.DecodeString(*j.PayloadBase64)
		if err != nil {
			return r, fmt.Errorf("payload_base64: %w", err)
		}
		r.Payload = b
	case j.Payload != nil:
		r.Payload = []byte(j.Payload)
	default:
		r.Payload = []byte{}
	}
	r.Size = len(r.Payload)
	if j.Size != nil {
		if *j.Size < 0 {
			return r, fmt.Errorf("size %d", *j.Size)
		}
		r.Size = *j.Size
	}
	return r, nil
}

// encodeRecord gives the JSON line of a record: a JSON payload is kept as
// JSON (compacted; the original length goes to size if it differs), any
// other payload is base64.
func encodeRecord(r Record) ([]byte, error) {
	j := recordJSON{Topic: r.Topic, Received: r.Received.UTC(), QoS: r.QoS, Retain: r.Retain}
	var buf bytes.Buffer
	switch {
	case len(r.Payload) == 0:
	case utf8.Valid(r.Payload) && json.Valid(r.Payload) && json.Compact(&buf, r.Payload) == nil:
		j.Payload = buf.Bytes()
		if buf.Len() != len(r.Payload) {
			size := len(r.Payload)
			j.Size = &size
		}
	default:
		s := base64.StdEncoding.EncodeToString(r.Payload)
		j.PayloadBase64 = &s
	}
	return json.Marshal(j)
}

// Writer records a capture line by line; every line is flushed, so an
// interrupted recording stays readable.
type Writer struct {
	w     *bufio.Writer
	count int
	last  time.Time
}

// NewWriter writes the header.
func NewWriter(w io.Writer, h Header) (*Writer, error) {
	h.Format, h.Version = Format, Version
	h.Started = h.Started.UTC()
	cw := &Writer{w: bufio.NewWriter(w), last: h.Started}
	return cw, cw.line(h)
}

func (cw *Writer) line(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	cw.w.Write(b)
	cw.w.WriteByte('\n')
	return cw.w.Flush()
}

// Write appends a record. A time before the previous one is raised to it, so
// the capture stays ordered.
func (cw *Writer) Write(r Record) error {
	if r.Received.Before(cw.last) {
		r.Received = cw.last
	}
	cw.last = r.Received
	b, err := encodeRecord(r)
	if err != nil {
		return err
	}
	cw.w.Write(b)
	cw.w.WriteByte('\n')
	if err := cw.w.Flush(); err != nil {
		return err
	}
	cw.count++
	return nil
}

// Close writes the trailer.
func (cw *Writer) Close(ended time.Time, cause error) error {
	if ended.Before(cw.last) {
		ended = cw.last
	}
	t := Trailer{Ended: ended.UTC(), Messages: cw.count}
	if cause != nil {
		t.Error = cause.Error()
	}
	return cw.line(t)
}
