// Command oecm-host runs the micro SDK core (oec/sdk/micro/src) as a smart
// device on a computer (ADR-0030 §7): the core is compiled by cgo, the PAL
// is Go (crypto/tls and the MQTT 3.1.1 client of package mqtt). It is the
// device under test of D7 in the gate and a quick way to put a micro SDK
// device on oec/dev. The broker configuration is read from the layout of
// the container storage below OEC_ROOT (default "/"): broker.json, the CA,
// the credentials or a client certificate, the hostname (the default
// serial number).
//
//	oecm-host [-level 1|2|3] [-state DIR] [-serial S] [-model M] …
//
// From Level 2 the device is a simulated temperature sensor (demo.c):
// Data "Temperature" with a writable Setpoint and the Config context
// "Sensor". Level 3 adds the ReferenceDesignation and NewDataSetWriterId;
// their records are files in the state directory (default
// OEC_ROOT/var/lib/oecm-host).
//
// SIGTERM or SIGINT publishes the Close and ends the program (exit 0).
package main

/*
#cgo CFLAGS: -std=c17 -I${SRCDIR}/../../../include -I${SRCDIR}/../../../src
#include <stdlib.h>
#include "oecm.h"

extern const oecm_pal oecm_host_pal;
void oecm_host_demo(oecm_config *cfg, oecm_device *dev);
unsigned oecm_host_demo_interval(void);
void oecm_host_demo_tick(void);
*/
import "C"

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/OI4/oec/sdk/micro/host/mqtt"
)

type eventKind int

const (
	evConnected eventKind = iota
	evFailed
	evLost
	evMessage
)

// event is a report of a network goroutine to the loop; gen tells which
// connection attempt it belongs to.
type event struct {
	gen     uint64
	kind    eventKind
	client  *mqtt.Client
	err     error
	topic   string
	payload []byte
}

// runner is the state of the one device. Only the loop goroutine (and the
// PAL functions it calls through the core) touch it, except events.
type runner struct {
	st        *storage
	log       *log.Logger
	began     time.Time
	dev       *C.oecm_device
	events    chan event
	gen       uint64
	cancel    context.CancelFunc
	state     string // directory of the persisted records
	client    *mqtt.Client
	published int
}

var current *runner

// txSize is the transmit buffer of the core: room for the largest
// publication of the demo (the Config with its definition).
const txSize = 8192

// refdesSize is the buffer of the ReferenceDesignation (Level 3); the core
// limits it to half the transmit buffer and the room of an answer.
const refdesSize = 2048

// drop ends the current connection attempt or connection: later events of
// it are ignored. A connected client is ended with DISCONNECT (no Will).
func (r *runner) drop() error {
	r.gen++
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	var err error
	if r.client != nil {
		err = r.client.Disconnect()
		r.client = nil
	}
	return err
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv("OEC_ROOT")))
}

// cstr is a C copy of s that lives as long as the program (the core
// borrows its configuration for the device lifetime).
func cstr(s string) *C.char {
	if s == "" {
		return nil
	}
	return C.CString(s)
}

func run(args []string, root string) int {
	logger := log.New(os.Stderr, "oecm-host: ", log.LstdFlags|log.Lmicroseconds)
	fs := flag.NewFlagSet("oecm-host", flag.ContinueOnError)
	level := fs.Int("level", 1, "Messaging level of the device (1, 2 or 3)")
	state := fs.String("state", "", "directory of the persisted records (Level 3); default: OEC_ROOT/var/lib/oecm-host")
	il := fs.String("il", "", "an existing Identification Link; default: the Oi4Identifier from the MAM")
	manufacturer := fs.String("manufacturer", "OI4", "MAM Manufacturer")
	manufacturerURI := fs.String("manufacturer-uri", "oi4.org", "MAM ManufacturerUri")
	model := fs.String("model", "OEC Micro Host", "MAM Model")
	productCode := fs.String("product-code", "OECM-HOST", "MAM ProductCode")
	serial := fs.String("serial", "", "MAM SerialNumber; default: the hostname of the storage")
	software := fs.String("software-revision", "0.1.0", "MAM SoftwareRevision")
	clientID := fs.String("client-id", "", "MQTT ClientId; default: derived from the Identification Link")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *level < 1 || *level > 3 {
		logger.Printf("-level %d: 1, 2 or 3", *level)
		return 2
	}
	if root == "" {
		root = "/"
	}
	if *state == "" {
		*state = filepath.Join(root, "var/lib/oecm-host")
	}
	st, err := loadStorage(root)
	if err != nil {
		logger.Print(err)
		return 1
	}
	if *serial == "" {
		*serial = st.name
	}

	runtime.LockOSThread() // one task drives the core, as on a device
	cfg := (*C.oecm_config)(C.calloc(1, C.size_t(unsafe.Sizeof(C.oecm_config{}))))
	cfg.level = C.oecm_level(*level)
	cfg.service_type = C.OECM_ST_OT_CONNECTOR // a directly connected smart device (SF-102)
	cfg.identification_link = cstr(*il)
	cfg.client_id = cstr(*clientID)
	cfg.max_packet_size = C.uint32_t(st.maxPacket)
	m := &cfg.mam
	m.manufacturer = cstr(*manufacturer)
	m.manufacturer_uri = cstr(*manufacturerURI)
	m.model = cstr(*model)
	m.product_code = cstr(*productCode)
	m.software_revision = cstr(*software)
	m.hardware_revision = cstr("host")
	m.device_class = cstr("Simulation")
	m.device_manual = cstr("https://oi4.org")
	m.serial_number = cstr(*serial)
	m.revision_counter = -1

	r := &runner{
		st:     st,
		log:    logger,
		began:  time.Now(),
		dev:    (*C.oecm_device)(C.calloc(1, C.size_t(unsafe.Sizeof(C.oecm_device{})))),
		events: make(chan event, 256),
		state:  *state,
	}
	current = r
	if *level >= 2 {
		C.oecm_host_demo(cfg, r.dev)
	}
	if *level >= 3 {
		cfg.refdes_buf = (*C.char)(C.malloc(refdesSize))
		cfg.refdes_cap = refdesSize
	}
	tx := (*C.uint8_t)(C.malloc(txSize))
	if e := C.oecm_init(r.dev, cfg, &C.oecm_host_pal, tx, txSize); e != C.OECM_OK {
		logger.Printf("oecm_init: %s", C.GoString(C.oecm_err_str(e)))
		return 1
	}
	logger.Printf("device %s, broker %s, %s", C.GoString(C.oecm_identification_link(r.dev)), st.addr, st.auth)
	if e := C.oecm_start(r.dev); e != C.OECM_OK {
		logger.Printf("oecm_start: %s", C.GoString(C.oecm_err_str(e)))
		return 1
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	timer := time.NewTimer(0)
	step := time.NewTimer(time.Duration(C.oecm_host_demo_interval()) * time.Second)
	if *level < 2 {
		step.Stop()
	}
	for {
		select {
		case sig := <-stop:
			e := C.oecm_stop(r.dev)
			logger.Printf("%v: stopped (%s), %d publications", sig, C.GoString(C.oecm_err_str(e)), r.published)
			if e != C.OECM_OK {
				return 1
			}
			return 0
		case ev := <-r.events:
			r.handle(ev)
		case <-timer.C:
		case <-step.C:
			C.oecm_host_demo_tick()
			step.Reset(time.Duration(C.oecm_host_demo_interval()) * time.Second)
		}
		if e := C.oecm_poll(r.dev); e != C.OECM_OK {
			logger.Printf("oecm_poll: %s", C.GoString(C.oecm_err_str(e)))
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(time.Duration(C.oecm_poll_interval_ms(r.dev)) * time.Millisecond)
	}
}

func (r *runner) handle(ev event) {
	if ev.gen != r.gen {
		if ev.kind == evConnected {
			_ = ev.client.Disconnect() // a late connection of a dropped attempt
		}
		return
	}
	switch ev.kind {
	case evConnected:
		r.client = ev.client
		r.cancel = nil
		r.log.Printf("connected")
		C.oecm_on_connected(r.dev)
	case evFailed:
		r.cancel = nil
		r.log.Printf("connect: %v", ev.err)
		C.oecm_on_disconnected(r.dev)
	case evLost:
		r.client = nil
		r.log.Printf("connection lost: %v", ev.err)
		C.oecm_on_disconnected(r.dev)
	case evMessage:
		topic := C.CString(ev.topic)
		payload := C.CBytes(ev.payload)
		C.oecm_on_message(r.dev, topic, (*C.uint8_t)(payload), C.size_t(len(ev.payload)))
		C.free(unsafe.Pointer(topic))
		C.free(payload)
	default:
		panic(fmt.Sprintf("event kind %d", ev.kind))
	}
}
