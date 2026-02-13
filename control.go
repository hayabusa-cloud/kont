// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Delimited control operators provide composable control flow.
// Shift/Reset follow Danvy & Filinski's formulation (1990).

// Shift captures the current continuation up to the nearest Reset.
// The function f receives the captured continuation k, which can be
// invoked zero or more times.
//
// Example:
//
//	Reset(Bind(Shift(func(k func(int) int) int {
//	    return k(k(3))  // Apply continuation twice
//	}), func(x int) Cont[int, int] {
//	    return Return[int](x * 2)
//	}))
//	// Result: 12 (3 * 2 * 2)
func Shift[R, A any](f func(k func(A) R) R) Cont[R, A] {
	return Cont[R, A](f)
}

// Reset establishes a delimiter for Shift.
// Continuations captured by Shift stop at the nearest enclosing Reset.
func Reset[R, A any](m Cont[A, A]) Cont[R, A] {
	return Return[R, A](Run(m))
}
