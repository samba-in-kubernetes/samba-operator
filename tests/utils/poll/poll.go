package poll

import (
	"context"
	"errors"
	"time"
)

var errProberCondUnset = errors.New("Prober Cond field is unset")

// Probe interfaces are used to repeatedly test for a condition.
type Probe interface {
	Condition() (bool, error)
}

// ProbeInterval interfaces specify both a condition as well as
// a time interval between probes.
type ProbeInterval interface {
	Probe
	Interval() time.Duration
}

// Completer interfaces are used to gather and/or report status
// when probing is complete.
type Completer interface {
	Completed(error) error
}

// Prober can be used to construct simple probes without creating
// a full fledged type.
type Prober struct {
	Cond          func() (bool, error)
	OnComplete    func(error) error
	RetryInterval time.Duration
}

// Condition returns true when the probe is successful, false if
// it not. It returns error on a non-recoverable failure.
func (p *Prober) Condition() (bool, error) {
	if p.Cond == nil {
		return false, errProberCondUnset
	}
	return p.Cond()
}

// Interval of time between probes.
func (p *Prober) Interval() time.Duration {
	if p.RetryInterval == 0 {
		return 200 * time.Millisecond
	}
	return p.RetryInterval
}

// Completed probing and can report/transform the exit status.
func (p *Prober) Completed(e error) error {
	if p.OnComplete == nil {
		return e
	}
	return p.OnComplete(e)
}

func noOpCompleted(e error) error {
	return e
}

// TryUntil the probe condition is true or the context deadline is
// exceeded. Probe can also be a ProbeInterval to specify the
// interval between probes. Probe can also be a Completer in order
// to update the return (error) after probing is done.
func TryUntil(ctx context.Context, p Probe) error {
	var (
		pi        ProbeInterval
		completed func(error) error
	)
	if x, ok := p.(ProbeInterval); ok {
		pi = x
	} else {
		pi = &Prober{Cond: p.Condition}
	}
	completed = noOpCompleted
	if x, ok := p.(Completer); ok {
		completed = x.Completed
	}

	for {
		c, err := pi.Condition()
		if err != nil {
			return completed(err)
		}
		if c {
			return completed(nil)
		}
		if err := ctx.Err(); err != nil {
			return completed(err)
		}
		time.Sleep(pi.Interval())
	}
}
