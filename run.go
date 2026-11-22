// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// identity is the identity continuation for Run.
// Named generic function produces a static function value per type instantiation,
// avoiding the heap allocation that anonymous closures incur.
func identity[A any](a A) A { return a }

// Run applies m to the identity continuation and returns its answer.
// Use [Handle] or [Step] to interpret pending effects.
func Run[A any](m Cont[A, A]) A {
	return m(identity[A])
}

// RunWith applies m to the final continuation k and returns m's answer.
// The body of m may call k zero or more times. Use [Handle] or [Step] to
// interpret pending effects.
func RunWith[R, A any](m Cont[R, A], k func(A) R) R {
	return m(k)
}
