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

package cycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mway.dev/chrono"
	"go.mway.dev/chrono/clock"
	"go.mway.dev/cycle"
	"go.mway.dev/math"
	"go.uber.org/atomic"
)

func TestNewTask(t *testing.T) {
	cases := []struct {
		name        string
		opts        []cycle.TaskOption
		expectError bool
	}{
		{
			name:        "no opts",
			opts:        nil,
			expectError: true,
		},
		{
			name:        "no clock",
			opts:        newTaskOptions(cycle.WithClock(nil)),
			expectError: true,
		},
		{
			name:        "no func",
			opts:        newTaskOptions(cycle.WithFunc(nil)),
			expectError: true,
		},
		{
			name:        "invalid jitter",
			opts:        newTaskOptions(cycle.WithJitter(-1)),
			expectError: true,
		},
		{
			name:        "valid opts",
			opts:        newTaskOptions(),
			expectError: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			task, err := cycle.NewTask(tt.opts...)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, task)
			} else {
				require.NoError(t, err)
				require.NotNil(t, task)
			}
		})
	}
}

func TestTaskPreStartState(t *testing.T) {
	task, _ := newTask(t)
	require.False(t, task.IsRunning())
	require.EqualValues(t, 0, task.Cycles())
	require.ErrorIs(t, task.Stop(context.Background()), cycle.ErrTaskNotRunning)
}

func TestTaskStartStopState(t *testing.T) {
	const rate = time.Second

	task, clk := newTask(t, cycle.WithRate(rate))
	require.NoError(t, task.Start(context.Background()))
	require.True(t, task.IsRunning())
	require.EqualValues(t, 0, task.Cycles())

	require.Never(t, func() bool { return task.Cycles() > 2 }, rate>>4, rate>>8)
	clk.Add(rate)
	require.Eventually(t, func() bool { return task.Cycles() > 2 }, rate, rate>>8)
	require.NoError(t, task.Stop(context.Background()))
	require.False(t, task.IsRunning())
}

func TestTaskDoubleStartStop(t *testing.T) {
	task, _ := newTask(t)
	require.NoError(t, task.Start(context.Background()))
	require.ErrorIs(t, task.Start(context.Background()), cycle.ErrTaskAlreadyStarted)
	require.NoError(t, task.Stop(context.Background()))
	require.ErrorIs(t, task.Stop(context.Background()), cycle.ErrTaskNotRunning)
}

func TestTaskOnStartHookError(t *testing.T) {
	var (
		err     = errors.New("hook error")
		task, _ = newTask(t, cycle.WithHooks(cycle.TaskHook{
			OnStart: func(context.Context) error {
				return err
			},
		}))
	)

	require.ErrorIs(t, task.Start(context.Background()), err)
}

func TestTaskOnStartContextError(t *testing.T) {
	t.Run("with hook error", func(t *testing.T) {
		var (
			hookErr = errors.New("hook error")
			task, _ = newTask(t, cycle.WithHooks(cycle.TaskHook{
				OnStart: func(context.Context) error {
					return hookErr
				},
			}))
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// OnStart hooks bail early if there's an error, so the context error is
		// not checked in those cases.
		require.ErrorIs(t, task.Start(ctx), context.Canceled)
	})

	t.Run("without hook error", func(t *testing.T) {
		task, _ := newTask(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		require.ErrorIs(t, task.Start(ctx), context.Canceled)
	})
}

func TestTaskOnStopHookError(t *testing.T) {
	var (
		hookErr = errors.New("hook error")
		task, _ = newTask(t, cycle.WithHooks(cycle.TaskHook{
			OnStop: func(context.Context) error {
				return hookErr
			},
		}))
	)

	require.NoError(t, task.Start(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, task.Stop(ctx), context.Canceled)
}

func TestTaskHooks(t *testing.T) {
	var (
		expectStart = []string{"first", "second", "third"}
		actualStart []string
		expectStop  = []string{"third", "second", "first"}
		actualStop  []string
		task, _     = newTask(
			t,
			cycle.WithHooks(
				cycle.TaskHook{
					OnStart: cycle.FuncToTask(func() { actualStart = append(actualStart, "first") }),
					OnStop:  cycle.FuncToTask(func() { actualStop = append(actualStop, "first") }),
				},
				cycle.TaskHook{
					OnStart: cycle.FuncToTask(func() { actualStart = append(actualStart, "second") }),
					OnStop:  cycle.FuncToTask(func() { actualStop = append(actualStop, "second") }),
				},
			),
			cycle.WithHooks(
				cycle.TaskHook{
					OnStart: cycle.FuncToTask(func() { actualStart = append(actualStart, "third") }),
					OnStop:  cycle.FuncToTask(func() { actualStop = append(actualStop, "third") }),
				},
			),
		)
	)

	require.NoError(t, task.Start(context.Background()))
	require.NoError(t, task.Stop(context.Background()))

	require.Equal(t, expectStart, actualStart)
	require.Equal(t, expectStop, actualStop)
}

func TestTaskHooks_OnlyStopIfStarted(t *testing.T) {
	var (
		err     = errors.New("start failed")
		stops   int
		task, _ = newTask(
			t,
			cycle.WithHooks(
				cycle.TaskHook{
					OnStart: cycle.FuncToTask(func() {
						// nop
					}),
					OnStop: cycle.FuncToTask(func() {
						stops++
					}),
				},
				cycle.TaskHook{
					OnStart: nil,
					OnStop: cycle.FuncToTask(func() {
						stops++
					}),
				},
				cycle.TaskHook{
					OnStart: func(context.Context) error {
						return err
					},
					OnStop: cycle.FuncToTask(func() {
						require.FailNow(
							t,
							"called stop hook without successful start hook",
						)
					}),
				},
			),
		)
	)

	require.ErrorIs(t, task.Start(context.Background()), err)
	require.Equal(t, 2, stops)
}

func TestTaskFuncInitialError(t *testing.T) {
	var (
		err       = errors.New("initial error")
		task, clk = newTask(t, cycle.WithFunc(cycle.ErrFuncToTask(func() error {
			return err
		})))
	)

	require.NoError(t, task.Start(context.Background()))
	clk.Add(time.Second)

	require.Eventually(t, func() bool { return task.Cycles() > 0 }, time.Second, time.Second>>8)
	require.ErrorIs(t, task.Err(), err)
	require.False(t, task.IsRunning())
}

func TestTaskFuncEventualError(t *testing.T) {
	var (
		err       = errors.New("eventual error")
		shouldErr = atomic.NewBool(false)
		task, clk = newTask(t, cycle.WithFunc(func(context.Context) error {
			if shouldErr.Load() {
				return err
			}
			return nil
		}))
	)

	require.NoError(t, task.Start(context.Background()))
	clk.Add(time.Second)

	require.Eventually(t, func() bool { return task.Cycles() > 0 }, time.Second, time.Second>>8)
	require.NoError(t, task.Err())
	require.True(t, task.IsRunning())

	shouldErr.Store(true)
	clk.Add(time.Second)

	require.Eventually(t, func() bool { return task.Cycles() > 1 }, time.Second, time.Second>>8)
	require.ErrorIs(t, task.Err(), err)
	require.False(t, task.IsRunning())
}

func TestTaskFuncPanic(t *testing.T) {
	task, clk := newTask(t, cycle.WithFunc(func(context.Context) error {
		panic("oops")
	}))

	require.NoError(t, task.Start(context.Background()))
	clk.Add(time.Second)

	require.Eventually(t, func() bool { return task.Cycles() > 0 }, time.Second, time.Second>>8)
	require.ErrorContains(t, task.Err(), "panic:")
	require.False(t, task.IsRunning())
}

func TestTask(t *testing.T) {
	const (
		cycles = 100
		rate   = time.Second
	)

	task, clk := newTask(
		t,
		cycle.WithFunc(func(context.Context) error { return nil }),
		cycle.WithRate(rate),
	)
	require.False(t, task.IsRunning())
	require.NoError(t, task.Start(context.Background()))
	require.True(t, task.IsRunning())

	for i := uint64(0); i < cycles; i++ {
		i := i
		clk.Add(rate)
		require.Eventually(t, func() bool { return task.Cycles() > i }, rate, rate>>8)
	}

	require.True(t, task.Cycles() >= 100, "observed %d calls", task.Cycles())
	require.True(t, task.IsRunning())
	require.NoError(t, task.Stop(context.Background()))
	require.False(t, task.IsRunning())
}

func TestRun(t *testing.T) {
	task, err := cycle.Run(time.Microsecond, func(context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.Cycles() > 10 }, time.Second, time.Second>>8)
	require.NoError(t, task.Stop(context.Background()))
}

func TestJitter(t *testing.T) {
	const rate = time.Millisecond

	var (
		last    int64
		samples []int64
		clk     = clock.NewMonotonicClock()
		task, _ = newTask(
			t,
			cycle.WithClock(clk),
			cycle.WithFunc(func(context.Context) error {
				now := chrono.Nanotime()
				if last > 0 {
					samples = append(samples, now-last)
				}
				last = now
				return nil
			}),
			cycle.WithJitterRange(rate, rate),
			cycle.WithRate(rate),
		)
	)

	require.NoError(t, task.Start(context.Background()))
	require.Eventually(t, func() bool { return task.Cycles() >= 100 }, time.Second, time.Second>>8)
	require.NoError(t, task.Stop(context.Background()))

	avg := math.Mean(samples...)
	require.InEpsilon(t, 1500*time.Microsecond, avg, float64(500*time.Microsecond))
}

func TestTaskFreeSpin(t *testing.T) {
	task, _ := newTask(
		t,
		cycle.WithClock(clock.NewMonotonicClock()),
		cycle.WithFreeSpin(),
		cycle.WithFunc(func(context.Context) error { return nil }),
	)

	require.NoError(t, task.Start(context.Background()))
	require.Eventually(t, func() bool { return task.Cycles() >= 1000 }, time.Second, time.Second>>8)
	require.NoError(t, task.Stop(context.Background()))
}

func TestRunInvalidFunc(t *testing.T) {
	task, err := cycle.Run(time.Microsecond, nil)
	require.Error(t, err)
	require.Nil(t, task)
}

func newTaskOptions(opts ...cycle.TaskOption) []cycle.TaskOption {
	return append(
		[]cycle.TaskOption{
			cycle.WithFunc(func(context.Context) error { return nil }),
			cycle.WithRate(time.Second),
			cycle.WithName("test-task"),
		},
		opts...,
	)
}

func newTask(
	t *testing.T,
	opts ...cycle.TaskOption,
) (*cycle.Task, *clock.FakeClock) {
	t.Helper()

	clk := clock.NewFakeClock()
	opts = newTaskOptions(
		append(
			[]cycle.TaskOption{
				cycle.WithClock(clk),
			},
			opts...,
		)...,
	)

	x, err := cycle.NewTask(opts...)
	require.NoError(t, err)

	return x, clk
}
