// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Cont represents a continuation-passing computation.
// Cont[R, A] computes a value of type A, with final result type R.
//
// The function receives a continuation k of type func(A) R, which represents
// "the rest of the computation". Applying k to a value of type A produces
// the final result of type R.
type Cont[R, A any] func(k func(A) R) R

// Return lifts a pure value into the continuation monad.
// The resulting computation immediately passes the value to its continuation.
func Return[R, A any](a A) Cont[R, A] {
	return func(k func(A) R) R {
		return k(a)
	}
}

// Eff is an effectful computation that produces a value of type A.
// This is the most common continuation type in effectful code.
type Eff[A any] = Cont[Resumed, A]

// Pure lifts a value into an effectful computation with no effects.
// Pure(a) is equivalent to Return[Resumed](a) with full type inference on A.
func Pure[A any](a A) Eff[A] {
	return Return[Resumed](a)
}

// Suspend creates a continuation from a CPS function.
// This is the primitive constructor for continuations that need direct
// access to the continuation.
func Suspend[R, A any](f func(func(A) R) R) Cont[R, A] {
	return Cont[R, A](f)
}
