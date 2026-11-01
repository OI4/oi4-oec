package oec

/*
#include <stdlib.h>
#include "oec.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"
)

// AssetOptions describes an asset the application represents
// (REQ-CMP-045), e.g. a field device without an MQTT client of its own.
type AssetOptions struct {
	// Product is the MAM of the asset.
	Product Product
	// SerialNumber completes the Oi4Identifier of the asset (REQ-IL-005).
	SerialNumber string
	// IdentificationLink is the Identification Link of the asset instead
	// of the Oi4Identifier (checked, REQ-IL-008).
	IdentificationLink string
}

// AddAsset adds a represented asset: its MAM and Health are published and
// served (REQ-CMP-045). Before or while the app runs.
func (a *App) AddAsset(o AssetOptions) (*Asset, error) {
	if err := noNUL("asset", o.SerialNumber, o.IdentificationLink); err != nil {
		return nil, err
	}
	var out *C.oec_asset
	err := a.do(func(app *C.oec_app) error {
		product, err := o.Product.newProduct()
		if err != nil {
			return err
		}
		defer C.oec_product_free(product)
		il, serial := optString(o.IdentificationLink), optString(o.SerialNumber)
		defer free(il)
		defer free(serial)
		return lastError(C.oec_app_add_asset(app, il, product, serial, &out))
	})
	if err != nil {
		return nil, err
	}
	return &Asset{app: a, c: out}, nil
}

// Remove removes a represented asset with its DataSets
// (REQ-RES-PUBLIST-017); later calls on it fail (SetHealth has no
// effect). Not for App.This.
func (s *Asset) Remove() error {
	return s.with(func(c *C.oec_asset) C.oec_status { return C.oec_asset_remove(c) })
}

// Event is a Generic event of an asset (REQ-RES-EVENT-009).
type Event struct {
	// Level is the free topic level of Pub/Event/<Source>/Generic/<Level>,
	// e.g. "Maintenance"; at most 16 per asset.
	Level  string
	Number uint32
	// Description is optional.
	Description string
	// Details is optional; it is encoded with encoding/json and must
	// become a JSON object.
	Details any
}

// EmitEvent publishes a Generic event of the asset; offline it is
// dropped.
func (s *Asset) EmitEvent(e Event) error {
	if err := noNUL("event", e.Level, e.Description); err != nil {
		return err
	}
	var details []byte
	if e.Details != nil {
		var err error
		if details, err = json.Marshal(e.Details); err != nil {
			return &Error{Status: StatusInvalidArgument, Message: "event details: " + err.Error()}
		}
	}
	level, desc, det := cString(e.Level), optString(e.Description), optBytes(details)
	defer free(level)
	defer free(desc)
	defer free(det)
	return s.with(func(c *C.oec_asset) C.oec_status {
		return C.oec_asset_emit_event(c, level, C.uint32_t(e.Number), desc, det)
	})
}

// BuiltinType is the OPC UA BuiltinType of a DataSet field
// (REQ-RES-META-003).
type BuiltinType int

// The BuiltinTypes of DataSet fields.
const (
	TypeBoolean BuiltinType = iota + 1
	TypeSByte
	TypeByte
	TypeInt16
	TypeUInt16
	TypeInt32
	TypeUInt32
	TypeInt64
	TypeUInt64
	TypeFloat
	TypeDouble
	TypeString
	TypeDateTime
	TypeGUID
	TypeByteString
	TypeLocalizedText
)

var builtinTypeNames = [...]string{"", "Boolean", "SByte", "Byte", "Int16", "UInt16", "Int32",
	"UInt32", "Int64", "UInt64", "Float", "Double", "String", "DateTime", "Guid", "ByteString",
	"LocalizedText"}

func (t BuiltinType) String() string {
	if t > 0 && int(t) < len(builtinTypeNames) {
		return builtinTypeNames[t]
	}
	return fmt.Sprintf("BuiltinType(%d)", int(t))
}

// Range is the range of a numeric value (property EURange).
type Range struct{ Low, High float64 }

// Field is one field of a DataSet; it becomes a FieldMetaData of the
// Metadata (REQ-RES-META-003). Names are unique within the DataSet
// (REQ-MSG-038).
type Field struct {
	Name string
	Type BuiltinType
	// Unit is the display name of the engineering unit; numeric fields
	// only.
	Unit string
	// Description, e.g. the measuring point of Pv (REQ-RES-DATA-005).
	Description string
	// Range of a numeric value; nil = none.
	Range *Range
	// Precision is the change of the value that triggers a publication
	// (Precisions of the PublicationList, REQ-RES-PUBLIST-013); 0 = any
	// change.
	Precision float64
}

// Mode is the publication Mode of a PublicationList entry
// (REQ-RES-PUBLIST-005): flags that are combined with |.
type Mode uint

// The Mode flags. The zero Mode is the default ModeOnRequest|ModeFilter;
// ModeOff stands alone for OFF_0 (not published).
const (
	ModeOnRequest   Mode = C.OEC_MODE_ON_REQUEST
	ModeApplication Mode = C.OEC_MODE_APPLICATION
	ModeSource      Mode = C.OEC_MODE_SOURCE
	ModeFilter      Mode = C.OEC_MODE_FILTER
	ModeOff         Mode = 1 << 8
)

func (m Mode) c() (C.uint, error) {
	switch {
	case m == 0:
		return C.uint(ModeOnRequest | ModeFilter), nil
	case m == ModeOff:
		return 0, nil
	case m&^(ModeOnRequest|ModeApplication|ModeSource|ModeFilter) != 0:
		return 0, &Error{Status: StatusInvalidArgument, Message: fmt.Sprintf("Mode %#x: ModeOff stands alone, other bits are no Mode", uint(m))}
	}
	return C.uint(m), nil
}

// Writable is what a Set PublicationList may change of the entry of a
// DataSet (REQ-RES-PUBLIST-016); the rest is answered with BadNotWritable.
type Writable int

// The Writable values.
const (
	WritableNothing         Writable = C.OEC_WRITABLE_NOTHING
	WritableMode            Writable = C.OEC_WRITABLE_MODE
	WritableInterval        Writable = C.OEC_WRITABLE_INTERVAL
	WritableModeAndInterval Writable = C.OEC_WRITABLE_MODE_AND_INTERVAL
)

// OI4PV is the Tag of the standardized process value DataSet: its first
// field is Pv, further fields are Sv1 … Sv255 (REQ-RES-DATA-002 … 005).
const OI4PV = "Oi4Pv"

// DataDef defines a Data DataSet with its Metadata and its
// PublicationList entry.
type DataDef struct {
	// Tag is the Filter of the topic and the WriterGroupName
	// (REQ-TOP-019 … 022, REQ-MSG-024).
	Tag    string
	Fields []Field
	// Mode of the PublicationList entry; zero = ModeOnRequest|ModeFilter.
	Mode Mode
	// Interval publishes cyclically, in whole milliseconds, whether the
	// values changed or not; 0 = on change (REQ-RES-PUBLIST-013/014).
	Interval time.Duration
	// Writable is what a Set PublicationList may change.
	Writable Writable
}

type rangeJSON struct {
	Low  float64 `json:"Low"`
	High float64 `json:"High"`
}

type fieldJSON struct {
	Name        string     `json:"Name"`
	Type        string     `json:"Type"`
	Unit        string     `json:"Unit,omitempty"`
	Description string     `json:"Description,omitempty"`
	Range       *rangeJSON `json:"Range,omitempty"`
	Precision   *float64   `json:"Precision,omitempty"`
}

// fieldsJSON is the fields_json of oec_asset_add_data.
func (d *DataDef) fieldsJSON() ([]byte, error) {
	fields := make([]fieldJSON, 0, len(d.Fields))
	for _, f := range d.Fields {
		if f.Type <= 0 || int(f.Type) >= len(builtinTypeNames) {
			return nil, &Error{Status: StatusInvalidArgument, Message: fmt.Sprintf("field %q: %v", f.Name, f.Type)}
		}
		j := fieldJSON{Name: f.Name, Type: f.Type.String(), Unit: f.Unit, Description: f.Description}
		if f.Range != nil {
			j.Range = &rangeJSON{f.Range.Low, f.Range.High}
		}
		if f.Precision != 0 {
			j.Precision = new(f.Precision)
		}
		fields = append(fields, j)
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return nil, &Error{Status: StatusInvalidArgument, Message: "fields: " + err.Error()}
	}
	return b, nil
}

// AddData adds a Data DataSet with its Metadata (REQ-RES-META-003) and
// its PublicationList entry. It is published once every field has a
// value (Update).
func (s *Asset) AddData(def DataDef) (*DataSet, error) {
	if err := noNUL("tag", def.Tag); err != nil {
		return nil, err
	}
	mode, err := def.Mode.c()
	if err != nil {
		return nil, err
	}
	if def.Interval < 0 || def.Interval%time.Millisecond != 0 || def.Interval.Milliseconds() > math.MaxUint32 {
		return nil, &Error{Status: StatusInvalidArgument, Message: fmt.Sprintf("Interval %v is no whole number of milliseconds within 32 bits", def.Interval)}
	}
	fields, err := def.fieldsJSON()
	if err != nil {
		return nil, err
	}
	tag, fj := cString(def.Tag), optBytes(fields)
	defer free(tag)
	defer free(fj)
	var out *C.oec_dataset
	err = s.with(func(c *C.oec_asset) C.oec_status {
		return C.oec_asset_add_data(c, tag, fj, mode, C.uint32_t(def.Interval.Milliseconds()),
			C.oec_writable(def.Writable), &out)
	})
	if err != nil {
		return nil, err
	}
	return &DataSet{app: s.app, c: out, tag: def.Tag}, nil
}

// DataSet is a Data DataSet of an asset; owned by its App.
type DataSet struct {
	app *App
	c   *C.oec_dataset
	tag string

	mu     sync.Mutex
	target *setTarget
}

func (d *DataSet) with(f func(*C.oec_dataset) C.oec_status) error {
	return d.app.with(func(*C.oec_app) C.oec_status { return f(d.c) })
}

// Update sets field values, all or none: values is encoded with
// encoding/json into a JSON object of field values in their OPC UA JSON
// form (REQ-MSG-031), e.g. map[string]any{"Pv": 21.5} or a struct with
// json tags. Non-blocking; when it is published follows the
// PublicationList entry.
func (d *DataSet) Update(values any) error {
	b, err := json.Marshal(values)
	if err != nil {
		return &Error{Status: StatusInvalidArgument, Message: "values: " + err.Error()}
	}
	v := optBytes(b)
	defer free(v)
	return d.with(func(c *C.oec_dataset) C.oec_status { return C.oec_dataset_update(c, v) })
}

// OnSet makes the DataSet writable (Set Data, Level 2 on,
// REQ-RES-DATA-011): fn gets the checked request, a JSON object with
// every field, and decides: nil accepts, a StatusCode (e.g.
// BadOutOfRange) rejects with that code, any other error with Bad. The SDK
// answers with the Status event and publishes the new state
// (REQ-RES-DATA-012, REQ-RES-EVENT-005). A further call replaces fn.
//
// fn runs on the SDK thread: it must not block; a call that waits for the
// bus fails with ErrInvalidState there (start a goroutine for it). A
// panic in fn is recovered and answered with BadInternalError.
func (d *DataSet) OnSet(fn func(values json.RawMessage) error) error {
	if fn == nil {
		return &Error{Status: StatusInvalidArgument, Message: "OnSet: fn is nil"}
	}
	wrapped := func(s string) error { return fn(json.RawMessage(s)) }
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.target != nil {
		return d.app.do(func(*C.oec_app) error {
			d.target.replace(wrapped)
			return nil
		})
	}
	t := &setTarget{what: "DataSet " + d.tag, fn: wrapped}
	err := d.app.do(func(*C.oec_app) error {
		return d.app.onSet(t, func(f C.oec_set_fn, ud unsafe.Pointer) C.oec_status {
			return C.oec_dataset_on_set(d.c, f, ud)
		})
	})
	if err == nil {
		d.target = t
	}
	return err
}

// Remove removes the DataSet (REQ-RES-PUBLIST-017); later calls on it
// fail.
func (d *DataSet) Remove() error {
	return d.with(func(c *C.oec_dataset) C.oec_status { return C.oec_dataset_remove(c) })
}
