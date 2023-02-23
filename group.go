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
	"sync"

	"go.uber.org/atomic"
	"go.uber.org/multierr"
)

var (
	// ErrGroupAlreadyStarted is returned by Group.Start if called once a Group
	// has already been started.
	ErrGroupAlreadyStarted = errors.New("group already started")
	// ErrGroupNotRunning is returned by Group.Stop if called when a Group is
	// not running.
	ErrGroupNotRunning = errors.New("group not running")
)

// A Group controls multiple Tasks simultaneously.
type Group struct {
	tasks map[string][]*Task
	mu    sync.Mutex
	state atomic.Uint32
}

// NewGroup creates a new Group with the given options.
func NewGroup(opts ...GroupOption) (*Group, error) {
	var (
		options = NewGroupOptions().With(opts...)
		tasks   = make(map[string][]*Task)
	)

	for _, task := range options.Tasks {
		tmp := tasks[task.name]
		tmp = append(tmp, task)
		tasks[task.name] = tmp
	}

	return &Group{
		tasks: tasks,
	}, nil
}

// Add creates a new Task with the given options and adds it to the Group. If
// the group is already running, the resulting task is started immediately;
// otherwise, it will be started when Group.Start is called.
func (g *Group) Add(opts ...TaskOption) error {
	task, err := NewTask(opts...)
	if err != nil {
		return err
	}

	if g.state.Load() == _stateRunning {
		if err = task.Start(context.Background()); err != nil {
			return err
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	tmp := g.tasks[task.name]
	tmp = append(tmp, task)
	g.tasks[task.name] = tmp

	return nil
}

// Start starts all tasks in the Group. Once called, additional tasks added via
// Add will be started immediately.
func (g *Group) Start(ctx context.Context) error {
	if !g.state.CompareAndSwap(_stateInitialized, _stateRunning) {
		return ErrGroupAlreadyStarted
	}

	return g.do(ctx, true)
}

// Stop stops all tasks in the Group.
func (g *Group) Stop(ctx context.Context) error {
	if !g.state.CompareAndSwap(_stateRunning, _stateStopped) {
		return ErrGroupNotRunning
	}

	return g.do(ctx, false)
}

func (g *Group) do(ctx context.Context, start bool) error {
	var num int
	for _, tasks := range g.tasks {
		num += len(tasks)
	}

	var (
		errch = make(chan error, num)
		wg    sync.WaitGroup
	)

	for _, tasks := range g.tasks {
		for _, task := range tasks {
			task := task
			wg.Add(1)
			go func() {
				defer wg.Done()
				if start {
					errch <- task.Start(ctx)
				} else {
					errch <- task.Stop(ctx)
				}
			}()
		}
	}

	wg.Wait()
	close(errch)

	var err error
	for e := range errch {
		err = multierr.Append(err, e)
	}

	return err
}
