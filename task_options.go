// Copyright (c) 2022 Matt Way
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE THE SOFTWARE.

package cycle

import (
	"errors"
	"time"

	"go.mway.dev/chrono/clock"
	"go.uber.org/multierr"
)

var (
	// ErrNoClock indicates that no valid clock was provided.
	ErrNoClock = errors.New("no clock provided")
	// ErrInvalidJitter indicates that a given jitter value is invalid.
	ErrInvalidJitter = errors.New("invalid jitter")
	// ErrNoTaskFunc indicates that no valid TaskFunc was provided.
	ErrNoTaskFunc = errors.New("no task function provided")
	// ErrNoTaskRate indicates that a given task rate is invalid.
	ErrNoTaskRate = errors.New("no task rate provided")

	_ TaskOption = (*TaskOptions)(nil)
)

// TaskOptions configure a Task.
type TaskOptions struct {
	// Clock is used within a Task for measuring time. This field is required.
	Clock clock.Clock
	// Func is the function executed by a Task. This field is required.
	Func TaskFunc
	// Hooks configure Task.Start and Task.Stop behavior.
	Hooks []TaskHook
	// Rate is the target cycle rate for a Task. This field is required.
	Rate time.Duration
	// JitterMax is the maximum jitter permitted for a Task. This value will
	// be dynamically clamped based on runtime cycle latency.
	JitterMax time.Duration
	// JitterMin is the minimum jitter permitted for a Task.
	JitterMin time.Duration
	// Name is the name of a Task. Task names do not need to be unique.
	Name string
}

// DefaultTaskOptions creates a new TaskOptions with some sane defaults. The
// resulting TaskOptions is not valid verbatim; see TaskOptions for information
// on required fields.
func DefaultTaskOptions() TaskOptions {
	//nolint:errcheck
	clk, _ := clock.NewClock(clock.WithNanotimeFunc(clock.DefaultNanotimeFunc()))

	return TaskOptions{
		Clock: clk,
	}
}

// Validate returns an error if the current TaskOptions is invalid.
func (o TaskOptions) Validate() (err error) {
	if o.Clock == nil {
		err = multierr.Append(err, ErrNoClock)
	}

	if o.Func == nil {
		err = multierr.Append(err, ErrNoTaskFunc)
	}

	if o.Rate == 0 {
		err = multierr.Append(err, ErrNoTaskRate)
	}

	if o.JitterMin < 0 || o.JitterMax < 0 {
		err = multierr.Append(err, ErrInvalidJitter)
	}

	return
}

// With copies the current TaskOptions and applies the given options to it,
// returning the result.
func (o TaskOptions) With(opts ...TaskOption) TaskOptions {
	if o.Hooks != nil {
		o.Hooks = append([]TaskHook(nil), o.Hooks...)
	}

	for _, opt := range opts {
		opt.apply(&o)
	}

	return o
}

func (o TaskOptions) apply(opts *TaskOptions) {
	if o.Clock != nil {
		opts.Clock = o.Clock
	}

	if o.Func != nil {
		opts.Func = o.Func
	}

	if o.Hooks != nil {
		opts.Hooks = append(opts.Hooks, o.Hooks...)
	}

	if o.Rate != 0 {
		opts.Rate = o.Rate
	}

	if o.JitterMin != 0 || o.JitterMax != 0 {
		opts.JitterMin = o.JitterMin
		opts.JitterMax = o.JitterMax
	}

	if len(o.Name) > 0 {
		opts.Name = o.Name
	}
}

// A TaskOption configures a Task.
type TaskOption interface {
	apply(*TaskOptions)
}

type optionFunc func(*TaskOptions)

func (f optionFunc) apply(o *TaskOptions) {
	f(o)
}

// WithClock configures a Task to use the given clock.
func WithClock(clk clock.Clock) TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.Clock = clk
	})
}

// WithFunc configures a Task to use the given function as its TaskFunc.
func WithFunc(fn TaskFunc) TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.Func = fn
	})
}

// WithName configures a Task to use the given name.
func WithName(name string) TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.Name = name
	})
}

// WithHooks configures a Task to use the given hooks.
func WithHooks(hooks ...TaskHook) TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.Hooks = append(o.Hooks, hooks...)
	})
}

// WithFreeSpin configures a Task to be free-spinning: it will not wait between
// cycles.
func WithFreeSpin() TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.Rate = -1
	})
}

// WithRate configures a Task to use the given rate.
func WithRate(rate time.Duration) TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.Rate = rate
	})
}

// WithJitter configures a Task to use [0,d) as its jitter range.
// bound.
func WithJitter(d time.Duration) TaskOption {
	return WithJitterRange(0, d)
}

// WithJitterRange configures a Task to use [min,max) as its jitter range.
func WithJitterRange(min time.Duration, max time.Duration) TaskOption {
	return optionFunc(func(o *TaskOptions) {
		o.JitterMin = min
		o.JitterMax = max
	})
}
