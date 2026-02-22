// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import (
	"sync/atomic"
)

// Affine wraps a continuation with one-shot enforcement.
// The continuation can be resumed at most once; subsequent attempts
// to resume will panic (Resume) or return false (TryResume).
//
// Affine types model affine resource usage and are fundamental to
// algebraic effect handlers where continuations must not be duplicated.
type Affine[R, A any] struct {
	used   atomic.Uintptr
	resume func(A) R
}

// Once creates an affine continuation from a regular continuation.
// The returned Affine can be resumed at most once.
func Once[R, A any](k func(A) R) *Affine[R, A] {
	return &Affine[R, A]{resume: k}
}

// Resume invokes the continuation with the given value.
// Panics if the continuation has already been used.
func (a *Affine[R, A]) Resume(v A) R {
	if a.used.Add(1) != 1 {
		panic("kont: affine continuation resumed twice")
	}
	return a.resume(v)
}

// TryResume attempts to invoke the continuation.
// Returns (result, true) on success, or (zero, false) if already used.
func (a *Affine[R, A]) TryResume(v A) (R, bool) {
	if a.used.Add(1) != 1 {
		var zero R
		return zero, false
	}
	return a.resume(v), true
}

// Discard marks the continuation as used without invoking it.
// This is useful for explicitly dropping a continuation that will not be used.
func (a *Affine[R, A]) Discard() {
	a.used.Store(1)
}
