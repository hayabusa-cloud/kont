// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync/atomic"

// Affine wraps a continuation that can be resumed at most once.
// Share the pointer returned by [Once]. Copying an unused Affine lets both
// copies invoke the same callback.
//
// Resume, TryResume, and Discard may compete. The winner clears the stored
// callback. Later Resume calls panic and TryResume calls return false.
// The total number of attempts on one handle must fit in a uintptr.
type Affine[R, A any] struct {
	attempts atomic.Uintptr
	resume   func(A) R
}

// Once creates an affine continuation from a regular continuation.
// The returned Affine can be resumed at most once.
func Once[R, A any](k func(A) R) *Affine[R, A] {
	return &Affine[R, A]{resume: k}
}

// Resume invokes the continuation with the given value.
// Panics if the continuation has already been used.
func (a *Affine[R, A]) Resume(v A) R {
	if !claimOnce(&a.attempts) {
		panic("kont: affine continuation resumed twice")
	}
	k := a.resume
	a.resume = nil
	return k(v)
}

// TryResume attempts to invoke the continuation.
// Returns (result, true) on success, or (zero, false) if already used.
func (a *Affine[R, A]) TryResume(v A) (R, bool) {
	if !claimOnce(&a.attempts) {
		var zero R
		return zero, false
	}
	k := a.resume
	a.resume = nil
	return k(v), true
}

// Discard releases the callback without invoking it. Repeated calls have no effect.
func (a *Affine[R, A]) Discard() {
	if claimOnce(&a.attempts) {
		a.resume = nil
	}
}

// Keeping the atomic add outside the generic methods lets Go 1.27 inline it in callers.
func claimOnce(attempts *atomic.Uintptr) bool {
	return attempts.Add(1) == 1
}
