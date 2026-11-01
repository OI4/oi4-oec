package oec

/*
#include <stdlib.h>
#include "oec.h"
*/
import "C"

import (
	"encoding/json"
	"strconv"
	"sync"
	"unsafe"
)

// ConfigType is the Type of a configuration object (REQ-RES-CONFIG-006);
// values travel as strings (REQ-RES-CONFIG-007).
type ConfigType int

// The Types of configuration objects.
const (
	ConfigBoolean ConfigType = iota + 1
	ConfigByteString
	ConfigDateTime
	ConfigNumber
	ConfigString
)

var configTypeNames = [...]string{"", "Boolean", "ByteString", "DateTime", "Number", "String"}

func (t ConfigType) String() string {
	if t > 0 && int(t) < len(configTypeNames) {
		return configTypeNames[t]
	}
	return "ConfigType(" + strconv.Itoa(int(t)) + ")"
}

// ConfigObject defines one configuration object. Names follow
// ^[A-Za-z_][A-Za-z0-9_-]*$ (REQ-RES-CONFIG-004).
type ConfigObject struct {
	Name string
	Type ConfigType
	// Default is the default and initial value in the string form of the
	// bus (REQ-RES-CONFIG-007); "" = unset.
	Default string
	// Label is the display name (Name, en-US); "" = Name.
	Label       string
	Description string // en-US
	Unit        string // REQ-RES-CONFIG-009
	Mandatory   bool   // REQ-RES-CONFIG-011
	// Sensitive values are masked as ******** in every Pub
	// (REQ-RES-CONFIG-024); the mask in a Set keeps the value
	// (REQ-RES-CONFIG-025).
	Sensitive bool
	// Validation (REQ-RES-CONFIG-008): Min and Max for ConfigNumber
	// (e.g. new(10.0)), MaxLength in characters (0 = none), Pattern (an
	// unanchored regular expression in the syntax of the Rust regex crate,
	// RE2-like), OneOf the allowed values.
	Min, Max  *float64
	MaxLength uint32
	Pattern   string
	OneOf     []string
}

// NumberObject is a ConfigNumber object with its default.
func NumberObject(name string, def float64) ConfigObject {
	return ConfigObject{Name: name, Type: ConfigNumber, Default: strconv.FormatFloat(def, 'f', -1, 64)}
}

// StringObject is a ConfigString object with its default.
func StringObject(name, def string) ConfigObject {
	return ConfigObject{Name: name, Type: ConfigString, Default: def}
}

// BooleanObject is a ConfigBoolean object with its default.
func BooleanObject(name string, def bool) ConfigObject {
	return ConfigObject{Name: name, Type: ConfigBoolean, Default: strconv.FormatBool(def)}
}

// ConfigGroup is a group of configuration objects (REQ-RES-CONFIG-013);
// the group of common objects is named "common" (REQ-RES-CONFIG-014).
type ConfigGroup struct {
	Name    string
	Label   string // display name (en-US); "" = Name
	Objects []ConfigObject
}

// ConfigDef defines a Config DataSet. An asset may have several
// (REQ-RES-CONFIG-001).
type ConfigDef struct {
	// Context is the Filter of the topic and the WriterGroupName
	// (REQ-RES-CONFIG-015, REQ-TOP-022).
	Context string
	Label   string // Context.Name (en-US); "" = Context
	Groups  []ConfigGroup
}

type localizedText struct {
	Locale string `json:"Locale"`
	Text   string `json:"Text"`
}

func enUS(text, fallback string) localizedText {
	if text == "" {
		text = fallback
	}
	return localizedText{"en-US", text}
}

type validationJSON struct {
	Length  uint32   `json:"Length,omitempty"`
	Min     *float64 `json:"Min,omitempty"`
	Max     *float64 `json:"Max,omitempty"`
	Pattern string   `json:"Pattern,omitempty"`
	Values  []string `json:"Values,omitempty"`
}

type configObjectJSON struct {
	Type         string          `json:"Type"`
	Value        string          `json:"Value"`
	DefaultValue string          `json:"DefaultValue"`
	Unit         string          `json:"Unit,omitempty"`
	Mandatory    bool            `json:"Mandatory,omitempty"`
	Sensitive    bool            `json:"Sensitive,omitempty"`
	Name         localizedText   `json:"Name"`
	Description  *localizedText  `json:"Description,omitempty"`
	Validation   *validationJSON `json:"Validation,omitempty"`
}

// definition is the Config DataSet as published (MG 8.3.3), the
// definition_json of oec_asset_add_config; the SDK checks it.
func (d *ConfigDef) definition() ([]byte, error) {
	invalid := func(msg string) error { return &Error{Status: StatusInvalidArgument, Message: msg} }
	payload := map[string]any{"Context": map[string]any{"Name": enUS(d.Label, d.Context)}}
	for _, g := range d.Groups {
		if _, dup := payload[g.Name]; dup {
			return nil, invalid("Config " + d.Context + ": group " + strconv.Quote(g.Name) + " twice")
		}
		group := map[string]any{"Name": enUS(g.Label, g.Name)}
		for _, o := range g.Objects {
			if _, dup := group[o.Name]; dup || o.Name == "Description" {
				return nil, invalid("Config " + d.Context + ": object " + g.Name + "/" + o.Name + " twice or reserved")
			}
			if o.Type <= 0 || int(o.Type) >= len(configTypeNames) {
				return nil, invalid("Config " + d.Context + ": object " + g.Name + "/" + o.Name + ": " + o.Type.String())
			}
			j := configObjectJSON{Type: o.Type.String(), Value: o.Default, DefaultValue: o.Default,
				Unit: o.Unit, Mandatory: o.Mandatory, Sensitive: o.Sensitive, Name: enUS(o.Label, o.Name)}
			if o.Description != "" {
				j.Description = &localizedText{"en-US", o.Description}
			}
			if o.Min != nil || o.Max != nil || o.MaxLength != 0 || o.Pattern != "" || o.OneOf != nil {
				j.Validation = &validationJSON{Length: o.MaxLength, Min: o.Min, Max: o.Max, Pattern: o.Pattern, Values: o.OneOf}
			}
			group[o.Name] = j
		}
		payload[g.Name] = group
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, invalid("Config " + d.Context + ": " + err.Error())
	}
	return b, nil
}

// AddConfig adds a Config DataSet; the SDK validates the definition
// (REQ-RES-CONFIG-004, REQ-RES-CONFIG-006 … 008) and restores the
// persisted values of the same Context.
func (s *Asset) AddConfig(def ConfigDef) (*Config, error) {
	b, err := def.definition()
	if err != nil {
		return nil, err
	}
	return s.AddConfigJSON(def.Context, b)
}

// AddConfigJSON adds a Config DataSet given as JSON in the form of its
// Payload (MG 8.3.3): an optional Context and the groups with their
// objects, each with Type, Value (the initial value) and Name and
// optionally DefaultValue, Unit, Mandatory, Sensitive, Description and
// Validation.
func (s *Asset) AddConfigJSON(context string, definition []byte) (*Config, error) {
	if err := noNUL("config", context, string(definition)); err != nil {
		return nil, err
	}
	ctx, def := cString(context), C.CString(string(definition))
	defer free(ctx)
	defer free(def)
	var out *C.oec_config
	err := s.with(func(c *C.oec_asset) C.oec_status { return C.oec_asset_add_config(c, ctx, def, &out) })
	if err != nil {
		return nil, err
	}
	return &Config{app: s.app, c: out, context: context}, nil
}

// Config is a Config DataSet of an asset; owned by its App.
type Config struct {
	app     *App
	c       *C.oec_config
	context string

	mu     sync.Mutex
	target *setTarget
}

func (c *Config) with(f func(*C.oec_config) C.oec_status) error {
	return c.app.with(func(*C.oec_app) C.oec_status { return f(c.c) })
}

// Get is the current value of one object in clear text; ok is false if it
// is unset or unknown (REQ-RES-CONFIG-010).
func (c *Config) Get(group, name string) (value string, ok bool, err error) {
	if err := noNUL("config", group, name); err != nil {
		return "", false, err
	}
	g, n := cString(group), cString(name)
	defer free(g)
	defer free(n)
	var v *C.char
	if err := c.with(func(cfg *C.oec_config) C.oec_status { return C.oec_config_get(cfg, g, n, &v) }); err != nil {
		return "", false, err
	}
	if v == nil {
		return "", false, nil
	}
	defer C.oec_string_free(v)
	return C.GoString(v), true, nil
}

// Set changes a value from the application side (e.g. a local UI) with
// the checks of a Set Config; "" unsets it. A change is persisted and
// published; no Status event, no call of the OnSet handler.
func (c *Config) Set(group, name, value string) error {
	if err := noNUL("config", group, name, value); err != nil {
		return err
	}
	g, n, v := cString(group), cString(name), cString(value)
	defer free(g)
	defer free(n)
	defer free(v)
	return c.with(func(cfg *C.oec_config) C.oec_status { return C.oec_config_set(cfg, g, n, v) })
}

// ConfigValues are the values of a Config by group and name in their
// string form (REQ-RES-CONFIG-007), unset ones as "". Sensitive values are
// in clear text: do not log them.
type ConfigValues map[string]map[string]string

// Value is the value of group/name; ok is false if it is unset or
// unknown.
func (v ConfigValues) Value(group, name string) (string, bool) {
	s, ok := v[group][name]
	return s, ok && s != ""
}

// Number is the value of a ConfigNumber object.
func (v ConfigValues) Number(group, name string) (float64, bool) {
	s, ok := v.Value(group, name)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// Bool is the value of a ConfigBoolean object.
func (v ConfigValues) Bool(group, name string) (bool, bool) {
	s, ok := v.Value(group, name)
	if !ok {
		return false, false
	}
	b, err := strconv.ParseBool(s)
	return b, err == nil
}

// OnSet decides about a Set Config (Level 2 on): fn gets the validated,
// complete proposal; nil accepts and the SDK applies and persists all
// values, a StatusCode (e.g. BadOutOfRange) or another error rejects all
// (REQ-RES-CONFIG-023, REQ-RES-EVENT-005). Without a handler the values
// are applied. A further call replaces fn. fn runs on the SDK thread as
// the handler of DataSet.OnSet.
func (c *Config) OnSet(fn func(proposed ConfigValues) error) error {
	if fn == nil {
		return &Error{Status: StatusInvalidArgument, Message: "OnSet: fn is nil"}
	}
	wrapped := func(s string) error {
		var v ConfigValues
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return BadInternalError
		}
		return fn(v)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.target != nil {
		return c.app.do(func(*C.oec_app) error {
			c.target.replace(wrapped)
			return nil
		})
	}
	t := &setTarget{what: "Config " + c.context, fn: wrapped}
	err := c.app.do(func(*C.oec_app) error {
		return c.app.onSet(t, func(f C.oec_set_fn, ud unsafe.Pointer) C.oec_status {
			return C.oec_config_on_set(c.c, f, ud)
		})
	})
	if err == nil {
		c.target = t
	}
	return err
}
