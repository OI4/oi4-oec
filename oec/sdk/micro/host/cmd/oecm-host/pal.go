package main

/*
#include "oecm.h"
*/
import "C"

import (
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/OI4/oec/sdk/micro/host/mqtt"
)

// The PAL in Go (ADR-0030 §7). The core calls these functions only from
// the loop goroutine (inside oecm_* calls), so they use the runner's
// state without locks; network goroutines report to the loop through
// r.events.

const dialTimeout = 20 * time.Second

//export oecmHostConnect
func oecmHostConnect(o *C.oecm_connect) C.oecm_err {
	r := current
	r.drop()
	opts := mqtt.Options{
		ClientID:    C.GoString(o.client_id),
		KeepAlive:   uint16(o.keep_alive_s),
		Username:    r.st.username,
		Password:    r.st.password,
		WillTopic:   C.GoString(o.will_topic),
		WillPayload: C.GoBytes(unsafe.Pointer(o.will_payload), C.int(o.will_len)),
		// A received packet may carry a payload of MaxPacketSize plus the
		// topic and the header (SF-105).
		MaxPacket: r.st.maxPacket + 65544,
	}
	gen := r.gen
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	r.cancel = cancel
	go func() {
		defer cancel()
		d := tls.Dialer{NetDialer: &net.Dialer{}, Config: r.st.tls}
		conn, err := d.DialContext(ctx, "tcp", r.st.addr)
		if err != nil {
			r.events <- event{gen: gen, kind: evFailed, err: err}
			return
		}
		c, err := mqtt.Connect(conn, opts, dialTimeout)
		if err != nil {
			r.events <- event{gen: gen, kind: evFailed, err: err}
			return
		}
		r.events <- event{gen: gen, kind: evConnected, client: c}
		err = c.Run(func(topic string, payload []byte) {
			r.events <- event{gen: gen, kind: evMessage, topic: topic, payload: payload}
		})
		r.events <- event{gen: gen, kind: evLost, err: err}
	}()
	return C.OECM_OK
}

//export oecmHostPublish
func oecmHostPublish(topic *C.char, payload *C.uint8_t, n C.size_t) C.oecm_err {
	r := current
	if r.client == nil {
		return C.OECM_E_PAL
	}
	t := C.GoString(topic)
	if err := r.client.Publish(t, C.GoBytes(unsafe.Pointer(payload), C.int(n))); err != nil {
		r.log.Printf("publish %s: %v", t, err)
		return C.OECM_E_PAL
	}
	r.published++
	return C.OECM_OK
}

//export oecmHostSubscribe
func oecmHostSubscribe(filter *C.char) C.oecm_err {
	r := current
	if r.client == nil || r.client.Subscribe(C.GoString(filter)) != nil {
		return C.OECM_E_PAL
	}
	return C.OECM_OK
}

//export oecmHostDisconnect
func oecmHostDisconnect() C.oecm_err {
	r := current
	connected := r.client != nil
	if err := r.drop(); err != nil {
		r.log.Printf("disconnect: %v", err)
		return C.OECM_E_PAL
	}
	if connected {
		r.log.Printf("disconnected")
	}
	return C.OECM_OK
}

//export oecmHostUnixMs
func oecmHostUnixMs() C.int64_t {
	return C.int64_t(time.Now().UnixMilli())
}

//export oecmHostMonoMs
func oecmHostMonoMs() C.uint32_t {
	return C.uint32_t(uint64(time.Since(current.began).Milliseconds()))
}

// The persisted records of Level 3 (oecm_pal persist_*): one file per key
// in the state directory, replaced by rename.

//export oecmHostLoad
func oecmHostLoad(key *C.char, buf *C.uint8_t, capacity C.size_t, n *C.size_t) C.oecm_err {
	r := current
	*n = 0
	raw, err := os.ReadFile(filepath.Join(r.state, C.GoString(key)))
	if errors.Is(err, fs.ErrNotExist) {
		return C.OECM_OK
	}
	if err != nil {
		r.log.Printf("persist load: %v", err)
		return C.OECM_E_PAL
	}
	if len(raw) > int(capacity) {
		r.log.Printf("persist load %s: %d octets, room for %d", C.GoString(key), len(raw), capacity)
		return C.OECM_E_NOSPACE
	}
	if len(raw) > 0 {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(buf)), len(raw)), raw)
	}
	*n = C.size_t(len(raw))
	return C.OECM_OK
}

//export oecmHostSave
func oecmHostSave(key *C.char, buf *C.uint8_t, n C.size_t) C.oecm_err {
	r := current
	path := filepath.Join(r.state, C.GoString(key))
	if err := save(path, C.GoBytes(unsafe.Pointer(buf), C.int(n))); err != nil {
		r.log.Printf("persist save: %v", err)
		return C.OECM_E_PAL
	}
	return C.OECM_OK
}

// save writes the record (len 0: removes it).
func save(path string, data []byte) error {
	if len(data) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
