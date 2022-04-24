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

package rand_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mway.dev/cycle/internal/rand"
	"go.mway.dev/math"
)

func TestDurationN(t *testing.T) {
	cases := []struct {
		input  time.Duration
		expect time.Duration
	}{
		{
			input:  0,
			expect: 0,
		},
		{
			input:  -1,
			expect: 0,
		},
		{
			input:  1,
			expect: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.input.String(), func(t *testing.T) {
			absinput := float64(math.Abs(tt.input))
			require.InDelta(
				t,
				absinput/2.0,
				rand.DurationN(tt.input),
				absinput/2.0,
			)
		})
	}
}

func TestDurationBetween(t *testing.T) {
	cases := []struct {
		min         time.Duration
		max         time.Duration
		expectRange []time.Duration
	}{
		{
			min:         0,
			max:         0,
			expectRange: []time.Duration{0, 0},
		},
		{
			min:         -1,
			max:         -1,
			expectRange: []time.Duration{0, 0},
		},
		{
			min:         time.Second,
			max:         0,
			expectRange: []time.Duration{0, time.Second},
		},
		{
			min:         0,
			max:         time.Second,
			expectRange: []time.Duration{0, time.Second},
		},
		{
			min:         time.Second,
			max:         time.Second,
			expectRange: []time.Duration{time.Second, time.Second},
		},
		{
			min:         time.Second,
			max:         10 * time.Second,
			expectRange: []time.Duration{time.Second, 10 * time.Second},
		},
	}

	for _, tt := range cases {
		name := fmt.Sprintf("%v-%v", tt.min, tt.max)
		t.Run(name, func(t *testing.T) {
			actual := rand.DurationBetween(tt.min, tt.max)
			require.True(t, tt.expectRange[0] <= actual && actual <= tt.expectRange[1])
		})
	}
}
