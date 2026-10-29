package probe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// Subscription is the topic filter of a test run: D7 watches the whole
// namespace, like monitor, to correlate answers and to feed the passive
// checks (DESIGN.md §3.3).
const Subscription = "Oi4v2/#"

// V1Namespace is the namespace element of version 1 (MG 7.1). With
// T7-ADR-02 selected, D7 also subscribes to V1Namespace/# (best effort) to
// see answers in the wrong version; these messages go to the runner only,
// not to the monitor or the recording (both judge version 2).
const V1Namespace = "Oi4"

// Session is the outcome of one test run on a bus.
type Session struct {
	Probe   Result
	Started time.Time
	Ended   time.Time
	Cause   error // the run was cut short (connection lost, interrupted)
}

// RunBus connects to the broker, subscribes to Subscription and runs the
// tests. Every received message is handed to observe (monitor engine,
// recording; nil: none) and then to the runner, from one goroutine in the
// order of reception. An error of observe or of the connection setup is
// returned as error; a lost connection ends the run with Session.Cause.
func RunBus(ctx context.Context, cfg mqttwire.Config, opt Options, reqs verdict.Requirements, observe func(mqttwire.Message) error) (Session, error) {
	s := Session{Started: time.Now().UTC()}
	if opt.TesterIL == "" {
		opt.TesterIL = TesterIL()
	}
	if opt.Peer && opt.PeerIL == "" {
		opt.PeerIL = PeerIL()
	}
	// The announced components (DESIGN.md §3.4, §3.5): D7 itself on its
	// connection, the peer on its own; both register their Will.
	var comps []*component
	var tester, peer *component
	if opt.Announce {
		tester = newComponent(RoleTester, opt.TesterIL)
		will, err := tester.will()
		if err != nil {
			return s, err
		}
		cfg.Will = will
		comps = append(comps, tester)
	}
	dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	cl, err := mqttwire.Dial(dctx, cfg)
	cancel()
	if err != nil {
		return s, err
	}
	defer cl.Close()
	publish := func(c *mqttwire.Client) func(context.Context, string, []byte) error {
		return func(ctx context.Context, topic string, payload []byte) error {
			return c.Publish(ctx, mqttwire.Message{Topic: topic, Payload: payload})
		}
	}
	if tester != nil {
		tester.pub = publish(cl)
	}
	var peerLost <-chan error
	var pcl *mqttwire.Client
	if opt.Peer {
		peer = newComponent(RolePeer, opt.PeerIL)
		pcfg := cfg
		pcfg.ClientID = mqttwire.ClientID("oec-check-peer-")
		if pcfg.Will, err = peer.will(); err != nil {
			return s, err
		}
		dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		pcl, err = mqttwire.Dial(dctx, pcfg)
		cancel()
		if err != nil {
			return s, fmt.Errorf("peer: %w", err)
		}
		defer pcl.Close()
		peer.pub, peerLost = publish(pcl), pcl.Lost()
		comps = append(comps, peer)
	}

	rctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	msgs := make(chan mqttwire.Message, 4096)
	if err := cl.Subscribe(rctx, Subscription, 1, func(m mqttwire.Message) { msgs <- m }); err != nil {
		return s, err
	}
	var v1Err error
	if Selected(opt.Tests, "T7-ADR-02") {
		v1Err = cl.Subscribe(rctx, V1Namespace+"/#", 1, func(m mqttwire.Message) { msgs <- m })
	}
	for _, c := range comps {
		if err := c.birth(rctx); err != nil {
			return s, fmt.Errorf("%s: birth: %w", c.role, err)
		}
	}
	// The CUT connects through the proxy once D7 sees the bus.
	if opt.Proxy != nil {
		opt.Proxy.Serve()
	}
	if opt.Subscribed != nil {
		if err := opt.Subscribed(rctx); err != nil {
			return s, err
		}
	}
	runner := New(opt, reqs, func(ctx context.Context, topic string, payload []byte) error {
		return cl.Publish(ctx, mqttwire.Message{Topic: topic, Payload: payload})
	})
	runner.v1Refused = v1Err
	runner.present = func(ctx context.Context) error {
		for _, c := range comps {
			if err := c.health(ctx, "NORMAL_0", 100); err != nil {
				return fmt.Errorf("%s: health: %w", c.role, err)
			}
		}
		return nil
	}
	cctx, stopComps := context.WithCancel(rctx)
	defer stopComps()
	var running sync.WaitGroup
	for _, c := range comps {
		running.Add(1)
		go func() {
			defer running.Done()
			c.run(cctx)
		}()
	}

	var observeErr error
	handle := func(m mqttwire.Message) {
		if observeErr != nil {
			return
		}
		if observe != nil && !strings.HasPrefix(m.Topic, V1Namespace+"/") {
			if err := observe(m); err != nil {
				observeErr = err
				stop(err)
				return
			}
		}
		msg := Message{Topic: m.Topic, Payload: m.Payload, Received: m.Received, Retain: m.Retain}
		runner.Deliver(msg)
		for _, c := range comps {
			if c.owns(m.Topic) {
				c.deliver(msg)
			}
		}
	}
	done := make(chan struct{})
	consumed := make(chan struct{})
	go func() {
		defer close(consumed)
		for {
			select {
			case m := <-msgs:
				handle(m)
			case err := <-cl.Lost():
				stop(fmt.Errorf("connection lost: %v", err))
			case err := <-peerLost:
				stop(fmt.Errorf("peer connection lost: %v", err))
			case <-done:
				for {
					select {
					case m := <-msgs:
						handle(m)
					default:
						return
					}
				}
			}
		}
	}()

	res, runErr := runner.Run(rctx)
	// The announced components leave with their Close message (MG 6.3.4);
	// the monitor sees it before the subscription ends.
	stopComps()
	running.Wait()
	if len(comps) > 0 && rctx.Err() == nil {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		for _, c := range comps {
			_ = c.close(closeCtx)
		}
		cancel()
		if pcl != nil {
			pcl.Close()
		}
		_ = sleep(ctx, runner.opt.Grace)
	}
	for _, c := range comps {
		res.Announced = append(res.Announced, c.report())
	}
	close(done)
	<-consumed
	s.Probe, s.Ended = res, time.Now().UTC()
	if observeErr != nil {
		return s, observeErr
	}
	if runErr != nil {
		if c := context.Cause(rctx); c != nil && !errors.Is(c, context.Canceled) {
			s.Cause = c
		} else if ctx.Err() != nil {
			s.Cause = errors.New("interrupted")
		} else {
			return s, runErr
		}
	}
	return s, nil
}
