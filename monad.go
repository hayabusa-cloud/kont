// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Monad operations for continuations.
//
// Minimal definition: Return (unit) and Bind are necessary and sufficient.
// Map and Then are derived operations kept as optimizations to avoid
// intermediate closure allocations.

// Bind sequences two continuations (monadic bind).
// It runs m, then passes the result to f to get a new continuation.
func Bind[R, A, B any](m Cont[R, A], f func(A) Cont[R, B]) Cont[R, B] {
	return func(k func(B) R) R {
		return m(func(a A) R {
			return f(a)(k)
		})
	}
}

// Map applies a pure function to the result of a continuation.
//
// Allocation note: Map is equivalent to Bind(m, compose(Return, f)) but
// avoids the intermediate Return closure, making it the preferred choice
// when the transformation is pure (does not produce effects).
func Map[R, A, B any](m Cont[R, A], f func(A) B) Cont[R, B] {
	return func(k func(B) R) R {
		return m(func(a A) R {
			return k(f(a))
		})
	}
}

// Then sequences two continuations, discarding the first result.
// This is more efficient than Bind when the second computation
// does not depend on the first result.
//
// Allocation note: Then avoids the closure capture of a transformation
// function that would occur with Bind(m, func(_ A) { return n }).
func Then[R, A, B any](m Cont[R, A], n Cont[R, B]) Cont[R, B] {
	return func(k func(B) R) R {
		return m(func(_ A) R {
			return n(k)
		})
	}
}
