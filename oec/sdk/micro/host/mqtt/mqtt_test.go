package mqtt

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"testing"
	"time"
)

func ptr(s string) *string { return &s }

func TestEncodeConnect(t *testing.T) {
	got, err := EncodeConnect(Options{
		ClientID: "c1", KeepAlive: 60, Username: ptr("u"), Password: ptr("p:w"),
		WillTopic: "w/t", WillPayload: []byte("bye"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x10, 32,
		0, 4, 'M', 'Q', 'T', 'T', 4, 0xc6, 0, 60, // user, password, Will (QoS 0, no retain), clean session
		0, 2, 'c', '1', 0, 3, 'w', '/', 't', 0, 3, 'b', 'y', 'e', 0, 1, 'u', 0, 3, 'p', ':', 'w'}
	if !bytes.Equal(got, want) {
		t.Fatalf("CONNECT\n got % x\nwant % x", got, want)
	}
	got, _ = EncodeConnect(Options{ClientID: "c"})
	if !bytes.Equal(got, []byte{0x10, 13, 0, 4, 'M', 'Q', 'T', 'T', 4, 0x02, 0, 0, 0, 1, 'c'}) {
		t.Fatalf("minimal CONNECT % x", got)
	}
	if _, err := EncodeConnect(Options{ClientID: "c", Password: ptr("p")}); err == nil {
		t.Fatal("password without user name accepted")
	}
}

func TestRemainingLength(t *testing.T) {
	for _, n := range []int{0, 1, 127, 128, 16383, 16384, 2097151, 2097152} {
		pkt, err := packet(0x30, make([]byte, n))
		if err != nil {
			t.Fatal(err)
		}
		typ, flags, body, err := ReadPacket(bufio.NewReader(bytes.NewReader(pkt)), maxRemainingLen)
		if err != nil || typ != 3 || flags != 0 || len(body) != n {
			t.Fatalf("%d: %d %d %d %v", n, typ, flags, len(body), err)
		}
		octets := 1
		for m := n; m >= 128; m /= 128 {
			octets++
		}
		if len(pkt)-n-1 != octets {
			t.Fatalf("%d: %d length octets, want %d", n, len(pkt)-n-1, octets)
		}
	}
	if _, _, _, err := ReadPacket(bufio.NewReader(bytes.NewReader([]byte{0x30, 0xff, 0xff, 0xff, 0xff, 0x01})), maxRemainingLen); err == nil {
		t.Fatal("five length octets accepted")
	}
	if _, _, _, err := ReadPacket(bufio.NewReader(bytes.NewReader([]byte{0x30, 5, 0, 1, 'a', 'b', 'c'})), 4); err == nil {
		t.Fatal("packet above the limit accepted")
	}
}

func TestParsePublish(t *testing.T) {
	topic, payload, id, err := ParsePublish(0, []byte{0, 1, 'a', 'x', 'y'})
	if err != nil || topic != "a" || string(payload) != "xy" || id != 0 {
		t.Fatalf("QoS 0: %q %q %d %v", topic, payload, id, err)
	}
	topic, payload, id, err = ParsePublish(0x02, []byte{0, 1, 'a', 0x12, 0x34, 'z'})
	if err != nil || topic != "a" || string(payload) != "z" || id != 0x1234 {
		t.Fatalf("QoS 1: %q %q %d %v", topic, payload, id, err)
	}
	for name, c := range map[string]struct {
		flags byte
		body  []byte
	}{
		"empty":       {0, nil},
		"short topic": {0, []byte{0, 5, 'a'}},
		"empty topic": {0, []byte{0, 0, 'x'}},
		"no id":       {0x02, []byte{0, 1, 'a', 1}},
		"QoS 3":       {0x06, []byte{0, 1, 'a', 0, 1}},
	} {
		if _, _, _, err := ParsePublish(c.flags, c.body); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// server is the broker side of a net.Pipe.
type server struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func pipe(t *testing.T) (net.Conn, *server) {
	c, s := net.Pipe()
	t.Cleanup(func() { c.Close(); s.Close() })
	_ = s.SetDeadline(time.Now().Add(10 * time.Second))
	return c, &server{t: t, conn: s, r: bufio.NewReader(s)}
}

func (s *server) expect(typ byte) (byte, []byte) {
	s.t.Helper()
	got, flags, body, err := ReadPacket(s.r, maxRemainingLen)
	if err != nil || got != typ {
		s.t.Errorf("server: want type %d, got %d (%v)", typ, got, err)
	}
	return flags, body
}

func (s *server) send(b ...byte) {
	if _, err := s.conn.Write(b); err != nil {
		s.t.Errorf("server: %v", err)
	}
}

func connect(t *testing.T, o Options, s *server, c net.Conn, rc byte) (*Client, error) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.expect(typeConnect)
		s.send(typeConnack<<4, 2, 0, rc)
	}()
	cl, err := Connect(c, o, 5*time.Second)
	<-done
	return cl, err
}

func TestConnectRefused(t *testing.T) {
	c, s := pipe(t)
	_, err := connect(t, Options{ClientID: "c"}, s, c, 5)
	var refused ConnackError
	if !errors.As(err, &refused) || refused != 5 || err.Error() != "CONNACK 5: not authorized" {
		t.Fatalf("got %v", err)
	}
}

func TestSession(t *testing.T) {
	c, s := pipe(t)
	cl, err := connect(t, Options{ClientID: "c"}, s, c, 0)
	if err != nil {
		t.Fatal(err)
	}
	type msg struct {
		topic   string
		payload string
	}
	got := make(chan msg, 4)
	ran := make(chan error, 1)
	go func() {
		ran <- cl.Run(func(topic string, payload []byte) { got <- msg{topic, string(payload)} })
	}()

	go func() {
		if err := cl.Publish("a/b", []byte("hi")); err != nil {
			t.Error(err)
		}
	}()
	if flags, body := s.expect(typePublish); flags != 0 || !bytes.Equal(body, []byte{0, 3, 'a', '/', 'b', 'h', 'i'}) {
		t.Fatalf("PUBLISH flags %x body % x", flags, body)
	}

	go func() {
		if err := cl.Subscribe("x/#"); err != nil {
			t.Error(err)
		}
	}()
	flags, body := s.expect(typeSubscribe)
	if flags != 0x02 || !bytes.Equal(body, []byte{0, 1, 0, 3, 'x', '/', '#', 0}) {
		t.Fatalf("SUBSCRIBE flags %x body % x", flags, body)
	}
	s.send(typeSuback<<4, 3, 0, 1, 0)
	s.send(typePublish<<4, 5, 0, 1, 'x', '1', '2')
	s.send(typePublish<<4|0x02, 7, 0, 1, 'y', 0, 9, '3', '4')
	if _, body := s.expect(typePuback); !bytes.Equal(body, []byte{0, 9}) {
		t.Fatalf("PUBACK % x", body)
	}
	s.send(typePingresp<<4, 0)
	for _, want := range []msg{{"x", "12"}, {"y", "34"}} {
		if m := <-got; m != want {
			t.Fatalf("got %v, want %v", m, want)
		}
	}

	go func() { _ = cl.Disconnect() }()
	s.expect(typeDisconnect)
	if err := <-ran; !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Run after Disconnect: %v", err)
	}
	if err := cl.Publish("a", nil); err == nil {
		t.Fatal("Publish after Disconnect succeeded")
	}
}

func TestRunErrors(t *testing.T) {
	for name, b := range map[string][]byte{
		"SUBACK refused":    {typeSuback << 4, 3, 0, 1, 0x80},
		"SUBACK unknown":    {typeSuback << 4, 3, 0, 7, 0},
		"oversized":         {typePublish << 4, 9, 0, 1, 'x', 1, 2, 3, 4, 5, 6},
		"unexpected type":   {typeConnack << 4, 2, 0, 0},
		"malformed PUBLISH": {typePublish << 4, 1, 0},
	} {
		t.Run(name, func(t *testing.T) {
			c, s := pipe(t)
			cl, err := connect(t, Options{ClientID: "c", MaxPacket: 8}, s, c, 0)
			if err != nil {
				t.Fatal(err)
			}
			if name == "SUBACK refused" {
				go func() { _ = cl.Subscribe("f") }()
				s.expect(typeSubscribe)
			}
			ran := make(chan error, 1)
			go func() { ran <- cl.Run(func(string, []byte) {}) }()
			s.send(b...)
			if err := <-ran; err == nil || errors.Is(err, net.ErrClosed) {
				t.Fatalf("Run: %v", err)
			}
		})
	}
}

func TestKeepAlive(t *testing.T) {
	c, s := pipe(t)
	cl, err := connect(t, Options{ClientID: "c", KeepAlive: 1}, s, c, 0)
	if err != nil {
		t.Fatal(err)
	}
	ran := make(chan error, 1)
	go func() { ran <- cl.Run(func(string, []byte) {}) }()
	began := time.Now()
	s.expect(typePingreq) // after 0.5 s, not answered
	select {
	case err := <-ran:
		if d := time.Since(began); d < 800*time.Millisecond {
			t.Fatalf("closed after %v", d)
		}
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no close without PINGRESP")
	}
}
