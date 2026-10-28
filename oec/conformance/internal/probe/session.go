package probe

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	cl, err := mqttwire.Dial(dctx, cfg)
	cancel()
	if err != nil {
		return s, err
	}
	defer cl.Close()

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
	if opt.Subscribed != nil {
		if err := opt.Subscribed(rctx); err != nil {
			return s, err
		}
	}
	runner := New(opt, reqs, func(ctx context.Context, topic string, payload []byte) error {
		return cl.Publish(ctx, mqttwire.Message{Topic: topic, Payload: payload})
	})
	runner.v1Refused = v1Err

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
		runner.Deliver(Message{Topic: m.Topic, Payload: m.Payload, Received: m.Received})
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
