// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Run executes a continuation with the identity continuation.
// The result type must match the value type (R = A).
func Run[A any](m Cont[A, A]) A {
	return m(func(a A) A { return a })
}

// RunWith executes a continuation with a custom final handler.
func RunWith[R, A any](m Cont[R, A], k func(A) R) R {
	return m(k)
}
