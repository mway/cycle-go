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
	"go.mway.dev/chrono/clock"
	"go.mway.dev/cycle"
	"go.uber.org/atomic"
)

func TestGroup(t *testing.T) {
	clk := clock.NewFakeClock()

	discardClock := func(e *cycle.Task, _ *clock.FakeClock) *cycle.Task {
		return e
	}

	var (
		waits = []chan struct{}{
			make(chan struct{}),
			make(chan struct{}),
			make(chan struct{}),
		}
		closeWait = func(ch chan struct{}) {
			select {
			case <-ch:
			default:
				close(ch)
			}
		}
		waitMax = func(ch chan struct{}, d time.Duration) {
			select {
			case <-ch:
			case <-time.After(d):
			}
		}
	)

	group, err := cycle.NewGroup(cycle.WithTasks(
		discardClock(newTask(
			t,
			cycle.WithClock(clk),
			cycle.WithRate(time.Millisecond),
			cycle.WithFunc(func(context.Context) error {
				closeWait(waits[0])
				return nil
			}),
		)),
		discardClock(newTask(
			t,
			cycle.WithClock(clk),
			cycle.WithRate(time.Millisecond),
			cycle.WithFunc(func(context.Context) error {
				closeWait(waits[1])
				return nil
			}),
		)),
		discardClock(newTask(
			t,
			cycle.WithClock(clk),
			cycle.WithRate(time.Millisecond),
			cycle.WithFunc(func(context.Context) error {
				closeWait(waits[2])
				return nil
			}),
		)),
	))
	require.NoError(t, err)
	require.NoError(t, group.Start(context.Background()))

	clk.Add(time.Second)

	waitMax(waits[0], time.Second)
	waitMax(waits[1], time.Second)
	waitMax(waits[2], time.Second)

	require.NoError(t, group.Stop(context.Background()))
}

func TestGroupDoubleStartStop(t *testing.T) {
	group := newGroup(t)
	require.NoError(t, group.Start(context.Background()))
	require.ErrorIs(t, group.Start(context.Background()), cycle.ErrGroupAlreadyStarted)
	require.NoError(t, group.Stop(context.Background()))
	require.ErrorIs(t, group.Stop(context.Background()), cycle.ErrGroupNotRunning)
}

func TestGroupAdd(t *testing.T) {
	var (
		group   = newGroup(t)
		calledA = atomic.NewBool(false)
		calledB = atomic.NewBool(false)
		funcA   = func(context.Context) error {
			calledA.Store(true)
			return nil
		}
		funcB = func(context.Context) error {
			calledB.Store(true)
			return nil
		}
	)

	require.NoError(t, group.Add(
		cycle.WithFunc(funcA),
		cycle.WithRate(time.Millisecond),
	))
	require.NoError(t, group.Start(context.Background()))
	require.NoError(t, group.Add(
		cycle.WithFunc(funcB),
		cycle.WithRate(time.Millisecond),
	))
	require.Eventually(t, calledA.Load, time.Second, time.Second>>8)
	require.Eventually(t, calledB.Load, time.Second, time.Second>>8)
	require.NoError(t, group.Stop(context.Background()))
}

func TestGroupAddWithErrors(t *testing.T) {
	group := newGroup(t)
	require.Error(t, group.Add())
	require.Error(t, group.Add(cycle.WithFunc(func(context.Context) error { return nil })))
	require.NoError(t, group.Start(context.Background()))
	require.Error(t, group.Add(
		cycle.WithFunc(func(context.Context) error { return nil }),
		cycle.WithRate(time.Millisecond),
		cycle.WithHooks(cycle.TaskHook{
			OnStart: func(context.Context) error {
				return errors.New("error")
			},
		}),
	))
	require.NoError(t, group.Stop(context.Background()))
}

func newGroup(t *testing.T, opts ...cycle.GroupOption) *cycle.Group {
	group, err := cycle.NewGroup(opts...)
	require.NoError(t, err)
	return group
}
