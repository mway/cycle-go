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

// Package rand provides internal rand utilities.
package rand

import (
	"math/rand"
	"time"
)

// DurationN returns a random duration in the range (0,d].
func DurationN(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}

	return DurationBetween(0, d)
}

// DurationBetween returns a random duration in the range (min,max].
func DurationBetween(min time.Duration, max time.Duration) time.Duration {
	if min < 0 {
		min = 0
	}

	if max < 0 {
		max = 0
	}

	if min == max {
		return min
	} else if max < min {
		min, max = max, min
	}

	return time.Duration(rand.Int63n(int64(max-min))) + min
}
