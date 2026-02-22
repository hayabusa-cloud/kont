// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// identity is the identity continuation for Run.
// Named generic function produces a static function value per type instantiation,
// avoiding the heap allocation that anonymous closures incur.
func identity[A any](a A) A { return a }

// Run executes a continuation with the identity continuation.
// The result type must match the value type (R = A).
func Run[A any](m Cont[A, A]) A {
	return m(identity[A])
}

// RunWith executes a continuation with a custom final continuation.
func RunWith[R, A any](m Cont[R, A], k func(A) R) R {
	return m(k)
}
