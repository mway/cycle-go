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
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.mway.dev/chrono"
	"go.mway.dev/chrono/clock"
	"go.mway.dev/cycle/internal/rand"
	xmath "go.mway.dev/math"
	"go.uber.org/atomic"
	"go.uber.org/multierr"
)

const (
	_stateInitialized = iota
	_stateRunning
	_stateStopped
)

var (
	// ErrTaskAlreadyStarted is returned by Task.Start if called after the Task
	// has started.
	ErrTaskAlreadyStarted = errors.New("task already started")
	// ErrTaskNotRunning is returned by Task.Stop if called when the Task is
	// not running.
	ErrTaskNotRunning = errors.New("task not running")
	// ErrStartHook is returned by Task.Start if a given TaskHook.OnStart
	// returned an error.
	ErrStartHook = errors.New("start hook error")
	// ErrStopHook is returned by Task.Stop if any TaskHook.OnStop returned an
	// error.
	ErrStopHook = errors.New("stop hook error")
)

// A TaskFunc is a function that can be executed by a Task. If a TaskFunc
// returns an error, the Task will stop.
type TaskFunc = func(context.Context) error

// A Task is a periodic task executed on a predefined interval. Unlike simple
// sleep loops, a Task will adjust its sleep duration dynamically based on the
// latency of its TaskFunc such that all calls are evenly spaced. If a cycle
// latencie exceeds the Task's configured rate, the subsequent cycle will be
// executed immediately. Once a Task has been stopped, it may not be started
// again: tasks are intended to be long-running operations.
//
// Tasks are designed to provide consistent cycle lengths, controlling for
// noise in each cycle, however they may also be configured to use jitter.
// See TaskOptions for more information.
type Task struct {
	ctx       context.Context
	cancel    context.CancelFunc
	clock     clock.Clock
	body      TaskFunc
	hooks     []TaskHook
	name      string
	state     atomic.Uint32
	wg        sync.WaitGroup
	rate      time.Duration
	cycles    atomic.Uint64
	err       error
	errmu     sync.Mutex
	jitterMin time.Duration
	jitterMax time.Duration
}

// NewTask creates a new Task based on the given options.
func NewTask(opts ...TaskOption) (*Task, error) {
	options := DefaultTaskOptions()
	for _, opt := range opts {
		opt.apply(&options)
	}

	if err := options.Validate(); err != nil {
		return nil, prefixedError(options.Name, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Task{
		ctx:       ctx,
		cancel:    cancel,
		clock:     options.Clock,
		body:      options.Func,
		hooks:     options.Hooks,
		name:      options.Name,
		rate:      options.Rate,
		jitterMin: options.JitterMin,
		jitterMax: options.JitterMax,
	}, nil
}

// Run creates a new Task with the given rate and TaskFunc, and starts it
// immediately.
func Run(d time.Duration, f TaskFunc) (*Task, error) {
	task, err := NewTask(WithFunc(f), WithRate(d))
	if err != nil {
		return nil, err
	}

	// It's not possible for Start to return an error when called by Run.
	//nolint:errcheck
	task.Start(context.Background())

	return task, nil
}

// Cycles returns the number of cycles that this Task has executed.
func (t *Task) Cycles() uint64 {
	return t.cycles.Load()
}

// Err returns the last error that the Task encountered while running.
func (t *Task) Err() error {
	t.errmu.Lock()
	defer t.errmu.Unlock()
	return t.err
}

// IsRunning returns whether or not the task is currently running.
func (t *Task) IsRunning() bool {
	return t.state.Load() == _stateRunning
}

// Start starts the task. Start will return an error if any configured OnStart
// hook returns an error, or if Start has previously been called.
//
// If the Task's TaskFunc returns an error, the Task will stop.
func (t *Task) Start(ctx context.Context) error {
	if !t.state.CAS(_stateInitialized, _stateRunning) {
		return ErrTaskAlreadyStarted
	}

	if err := t.runStartHooks(ctx); err != nil {
		return err
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.run()
	}()

	return nil
}

// Stop stops the task. Stop will return an error if any configured OnStop hook
// returns an error, or if Stop has previously been called. Note that the Task
// will still be stopped even if the OnStop hooks error.
func (t *Task) Stop(ctx context.Context) error {
	if !t.state.CAS(_stateRunning, _stateStopped) {
		return ErrTaskNotRunning
	}

	err := t.runStopHooks(ctx)

	t.cancel()
	t.wg.Wait()

	return err
}

func (t *Task) jitter(max time.Duration) time.Duration {
	return rand.DurationBetween(
		xmath.Min(t.jitterMin, max),
		xmath.Min(t.jitterMax, max),
	)
}

func (t *Task) run() {
	timer := t.clock.NewTimer(time.Duration(math.MaxInt64))
	defer func() {
		// Explicitly manager our state in the event we're exiting due to a
		// TaskFunc error.
		t.state.Store(_stateStopped)

		if !timer.Stop() {
			select {
			case <-timer.C():
			default:
			}
		}
	}()

	// Run once immediately, without jitter.
	t.cycles.Inc()
	elapsed, err := t.runBody(t.rate)
	if err != nil {
		t.storeError(err)
		return
	}

	for {
		t.cycles.Inc()
		timer.Reset(xmath.Max(0, t.rate-elapsed))

		select {
		case <-timer.C():
			if elapsed, err = t.runBody(elapsed); err != nil {
				t.storeError(err)
				return
			}
		case <-t.ctx.Done():
			t.storeError(err)
			return
		}
	}
}

func (t *Task) runBody(
	prev time.Duration,
) (elapsed time.Duration, err error) {
	start := chrono.Nanotime()
	defer func() {
		elapsed = time.Duration(chrono.Nanotime() - start)
		if r := recover(); r != nil {
			err = multierr.Combine(err, fmt.Errorf("panic: %v", r))
		}
	}()

	if jitter := t.jitter(t.rate - prev); prev < t.rate && jitter > 0 {
		t.clock.Sleep(jitter)
	}

	err = t.body(context.TODO())
	return
}

func (t *Task) runStartHooks(ctx context.Context) error {
	for _, hook := range t.hooks {
		if hook.OnStart != nil {
			if err := hook.OnStart(ctx); err != nil {
				return multierr.Append(ErrStartHook, err)
			}
		}
	}

	err := ctx.Err()
	if err != nil {
		err = multierr.Append(ErrStartHook, err)
	}
	return err
}

func (t *Task) runStopHooks(ctx context.Context) error {
	var err error
	for i := len(t.hooks) - 1; i >= 0; i-- {
		hook := t.hooks[i]
		if hook.OnStop != nil {
			err = multierr.Append(err, hook.OnStop(ctx))
		}
	}

	err = multierr.Append(err, ctx.Err())
	if err != nil {
		err = multierr.Combine(ErrStopHook, err)
	}

	return err
}

func (t *Task) storeError(err error) {
	t.errmu.Lock()
	defer t.errmu.Unlock()
	t.err = err
}

// A TaskHook provides hooks to be executed as part of Task.Start and
// Task.Stop. Hooks are cumulative: for example, hooks may be added through
// by more than one TaskOption.
//
// OnStart hooks are executed in the order in which they were provided. OnStop
// hooks are executed in reverse order.
type TaskHook struct {
	// OnStart is executed as part of Task.Start.
	OnStart func(context.Context) error
	// OnStop is executed as part of Task.Stop.
	OnStop func(context.Context) error
}
