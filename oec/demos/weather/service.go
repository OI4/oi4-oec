package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	oec "github.com/OI4/oec/sdk/go"
)

// service runs the stations: one poll goroutine owns them and is the only
// one that calls the SDK after the start (assets, Data, events). The
// OnSet handlers run on the SDK thread and must not block: they check a
// proposal, hand it over and wake the poll goroutine (A-344).
type service struct {
	app  *oec.App
	host string
	out  io.Writer

	mu    sync.Mutex
	next  *settings                    // accepted application Config, not yet applied
	moved map[*weatherStation]Location // accepted station Configs, not yet applied
	wake  chan struct{}

	// Owned by the poll goroutine (and by start before it runs).
	cur      settings
	provider Provider
	stations []*weatherStation
}

// weatherStation is a weather station: an asset represented on Level 3
// like the application (REQ-CMP-045), with its Config and Data.
type weatherStation struct {
	index  int
	loc    Location
	asset  *oec.Asset
	config *oec.Config
	data   *oec.DataSet
	// Whether the warnings are on (A-345).
	frost, storm bool
}

func newService(app *oec.App, host string, out io.Writer) *service {
	return &service{app: app, host: host, out: out, moved: map[*weatherStation]Location{}, wake: make(chan struct{}, 1)}
}

func (s *service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// start adds the application Config and the stations of the stored or
// default settings.
func (s *service) start() error {
	def := appConfig()
	cfg, err := s.app.This().AddConfig(def)
	if err != nil {
		return err
	}
	cur, err := parseSettings(read(cfg, def))
	if err != nil {
		fmt.Fprintln(os.Stderr, "oec-weather: stored Config:", err, "– default used")
	}
	if err := cfg.OnSet(func(proposed oec.ConfigValues) error {
		next, err := parseSettings(proposed)
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.next = &next
		s.mu.Unlock()
		s.signal()
		return nil
	}); err != nil {
		return err
	}
	if s.provider, err = newProvider(cur); err != nil {
		return err
	}
	s.cur = cur
	s.logf("config provider=%s stations=%d pollInterval=%s", cur.Provider, cur.Stations, cur.PollInterval)
	return s.resize()
}

// newProvider is the provider of the settings.
func newProvider(set settings) (Provider, error) {
	switch set.Provider {
	case "simulation":
		return Simulation{Seed: set.Seed}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", set.Provider)
}

// read are the current values of a Config ("" = unset).
func read(cfg *oec.Config, def oec.ConfigDef) oec.ConfigValues {
	v := oec.ConfigValues{}
	for _, g := range def.Groups {
		v[g.Name] = map[string]string{}
		for _, o := range g.Objects {
			x, _, _ := cfg.Get(g.Name, o.Name)
			v[g.Name][o.Name] = x
		}
	}
	return v
}

// stationProduct is the MAM of a station (A-341).
var stationProduct = oec.Product{
	ManufacturerURI:  "oi4.org",
	Model:            "OEC Weather Station",
	ProductCode:      "OEC-WEATHER-STATION",
	Manufacturer:     "Open Industry 4.0 Alliance",
	Description:      "Weather station of the OEC demo weather service",
	SoftwareRevision: version,
}

// addStation adds station i: asset with MAM and Health, its Config with
// the stored or default location, its Data Oi4Pv with Metadata.
func (s *service) addStation(i int) (*weatherStation, error) {
	asset, err := s.app.AddAsset(oec.AssetOptions{Product: stationProduct, SerialNumber: stationSerial(s.host, i)})
	if err != nil {
		return nil, err
	}
	st := &weatherStation{index: i, asset: asset}
	fail := func(err error) (*weatherStation, error) {
		_ = asset.Remove()
		return nil, err
	}
	if err := asset.SetHealth(oec.Normal, 100); err != nil {
		return fail(err)
	}
	def := stationConfig(i)
	if st.config, err = asset.AddConfig(def); err != nil {
		return fail(err)
	}
	if st.loc, err = parseLocation(i, read(st.config, def)); err != nil {
		fmt.Fprintf(os.Stderr, "oec-weather: stored Config of station %d: %v – default used\n", i, err)
	}
	if err := st.config.OnSet(func(proposed oec.ConfigValues) error {
		loc, err := parseLocation(i, proposed)
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.moved[st] = loc
		s.mu.Unlock()
		s.signal()
		return nil
	}); err != nil {
		return fail(err)
	}
	// Published on a change of at least the Precision of a field;
	// consumers may change Mode and Interval by Set PublicationList.
	if st.data, err = asset.AddData(oec.DataDef{Tag: oec.OI4PV, Fields: fields(), Writable: oec.WritableModeAndInterval}); err != nil {
		return fail(err)
	}
	s.logf("station %d %s at %g, %g, %g m added", i, st.loc.Name, st.loc.Latitude, st.loc.Longitude, st.loc.Altitude)
	return st, nil
}

// resize adds or removes stations to the configured number. A removed
// station leaves the PublicationList (REQ-RES-PUBLIST-017); MG defines no
// Close message for a represented asset (A-341).
func (s *service) resize() error {
	for len(s.stations) > s.cur.Stations {
		last := s.stations[len(s.stations)-1]
		if err := last.asset.Remove(); err != nil {
			return err
		}
		s.stations = s.stations[:len(s.stations)-1]
		s.logf("station %d %s removed", last.index, last.loc.Name)
	}
	for len(s.stations) < s.cur.Stations {
		st, err := s.addStation(len(s.stations) + 1)
		if err != nil {
			return err
		}
		s.stations = append(s.stations, st)
	}
	return nil
}

// apply applies the accepted Configs.
func (s *service) apply() error {
	s.mu.Lock()
	next, moved := s.next, s.moved
	s.next, s.moved = nil, map[*weatherStation]Location{}
	s.mu.Unlock()
	if next != nil {
		p, err := newProvider(*next)
		if err != nil {
			return err
		}
		s.cur, s.provider = *next, p
		s.logf("config provider=%s stations=%d pollInterval=%s", next.Provider, next.Stations, next.PollInterval)
		if err := s.resize(); err != nil {
			return err
		}
	}
	// In the order of the stations; a removed station is skipped.
	for _, st := range s.stations {
		if loc, ok := moved[st]; ok {
			st.loc = loc
			s.logf("station %d %s at %g, %g, %g m", st.index, loc.Name, loc.Latitude, loc.Longitude, loc.Altitude)
		}
	}
	return nil
}

// run is the poll goroutine: it observes every station each poll
// interval and right after an accepted Set Config. The first round waits
// for the connection, so the first Data and warnings are not dropped.
func (s *service) run(ctx context.Context) error {
	online := time.NewTicker(20 * time.Millisecond)
	for s.app.State() != oec.Online {
		select {
		case <-ctx.Done():
			online.Stop()
			return nil
		case <-online.C:
		}
	}
	online.Stop()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		case <-s.wake:
		}
		if err := s.apply(); err != nil {
			return err
		}
		if err := s.poll(ctx); err != nil {
			return err
		}
		timer.Reset(s.cur.PollInterval)
	}
}

// poll observes every station, publishes its Data and its warnings.
func (s *service) poll(ctx context.Context) error {
	now := time.Now().UTC()
	for _, st := range s.stations {
		o, err := s.provider.Observe(ctx, st.loc, now)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintf(os.Stderr, "oec-weather: station %d: %v\n", st.index, err)
			continue
		}
		if err := st.data.Update(dataValues(o)); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		// Events are dropped offline (ADR-0027): a warning is evaluated
		// online only, so a change is reported after a reconnect.
		if s.app.State() != oec.Online {
			continue
		}
		s.warn(st, "Frost", o.Temperature <= s.cur.FrostThreshold, &st.frost,
			map[string]float64{"Temperature": o.Temperature, "FrostThreshold": s.cur.FrostThreshold})
		s.warn(st, "Storm", o.WindSpeed >= s.cur.StormThreshold, &st.storm,
			map[string]float64{"WindSpeed": o.WindSpeed, "StormThreshold": s.cur.StormThreshold})
	}
	s.logf("poll %d stations", len(s.stations))
	return nil
}

// warn raises the Generic event of a warning when it begins (Number 1)
// and when it ends (Number 0) (A-345).
func (s *service) warn(st *weatherStation, level string, on bool, state *bool, details map[string]float64) {
	if on == *state {
		return
	}
	*state = on
	number, what := uint32(0), "ends"
	if on {
		number, what = 1, "begins"
	}
	err := st.asset.EmitEvent(oec.Event{Level: level, Number: number,
		Description: fmt.Sprintf("%s %s at %s", level, what, st.loc.Name), Details: details})
	if err != nil && !errors.Is(err, oec.ErrClosed) {
		fmt.Fprintf(os.Stderr, "oec-weather: event: %v\n", err)
	}
	s.logf("event station %d %s %s", st.index, level, what)
}

func (s *service) logf(format string, args ...any) {
	fmt.Fprintf(s.out, format+"\n", args...)
}
