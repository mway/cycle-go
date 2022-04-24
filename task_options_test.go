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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mway.dev/cycle"
)

func TestTaskOptionsWith(t *testing.T) {
	var (
		base = cycle.DefaultTaskOptions().With(cycle.WithHooks(
			cycle.TaskHook{
				OnStart: func(context.Context) error { return errors.New("onstart") },
			},
		))
		expect = cycle.TaskOptions{
			Clock: base.Clock,
			Func:  func(context.Context) error { return errors.New("foo") },
			Hooks: []cycle.TaskHook{
				{
					OnStart: base.Hooks[0].OnStart,
				},
				{
					OnStop: func(context.Context) error { return errors.New("onstop") },
				},
			},
			Rate:      -1,
			JitterMin: time.Millisecond,
			JitterMax: time.Second,
			Name:      "test task",
		}
		withA = cycle.TaskOptions{
			Clock: base.Clock,
			Func:  expect.Func,
			Hooks: []cycle.TaskHook{
				{
					OnStop: expect.Hooks[1].OnStop,
				},
			},
			Rate:      -1,
			JitterMin: time.Millisecond,
			JitterMax: time.Second,
			Name:      "test task",
		}
		withB = []cycle.TaskOption{
			cycle.WithFunc(expect.Func),
			cycle.WithRate(time.Hour),
			cycle.WithFreeSpin(),
			cycle.WithJitterRange(expect.JitterMin, expect.JitterMax),
			cycle.WithHooks(cycle.TaskHook{
				OnStop: expect.Hooks[1].OnStop,
			}),
			cycle.WithName(expect.Name),
		}
	)

	actuals := []cycle.TaskOptions{
		base.With(withA),
		base.With(withB...),
	}

	for _, actual := range actuals {
		require.Equal(t, expect.Clock, actual.Clock)
		require.Equal(t, addr(expect.Func), addr(actual.Func))
		require.Equal(t, len(expect.Hooks), len(actual.Hooks))
		for i := range expect.Hooks {
			require.Equal(t, addr(expect.Hooks[i].OnStart), addr(actual.Hooks[i].OnStart))
			require.Equal(t, addr(expect.Hooks[i].OnStop), addr(actual.Hooks[i].OnStop))
		}
		require.Equal(t, expect.Rate, actual.Rate)
		require.Equal(t, expect.JitterMin, actual.JitterMin)
		require.Equal(t, expect.JitterMax, actual.JitterMax)
		require.Equal(t, expect.Name, actual.Name)
	}
}

func addr(x interface{}) string {
	return fmt.Sprintf("%p", x)
}
