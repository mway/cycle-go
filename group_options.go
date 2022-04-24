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

// GroupOptions configure a Group.
type GroupOptions struct {
	Tasks []*Task
}

// NewGroupOptions creates a new GroupOptions.
func NewGroupOptions() GroupOptions {
	return GroupOptions{}
}

// With merges the given options into a copy of the current GroupOptions and
// returns the result.
func (o GroupOptions) With(opts ...GroupOption) GroupOptions {
	for _, opt := range opts {
		opt.apply(&o)
	}
	return o
}

func (o GroupOptions) apply(opts *GroupOptions) {
	opts.Tasks = o.Tasks
}

// A GroupOption configures a Group.
type GroupOption interface {
	apply(*GroupOptions)
}

type groupOptionFunc func(*GroupOptions)

func (f groupOptionFunc) apply(o *GroupOptions) {
	f(o)
}

// WithTasks configures a GroupOption to use the given tasks.
func WithTasks(tasks ...*Task) GroupOption {
	return groupOptionFunc(func(o *GroupOptions) {
		o.Tasks = append([]*Task(nil), tasks...)
	})
}
