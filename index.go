// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// StepIndex is the finite approximation level used by step-indexed
// interpretations of kont computations.
//
// StepIndex does not change the behavior of [Step], [StepExpr], or
// [Suspension]. It is an explicit fuel witness for callers that interpret a
// finite prefix of the one-effect-at-a-time stepping boundary.
type StepIndex uint64

// IsZero reports whether n has no remaining step credit.
func (n StepIndex) IsZero() bool { return n == 0 }

// Prev consumes one step of credit.
// The second result is false when n is already zero.
func (n StepIndex) Prev() (StepIndex, bool) {
	if n == 0 {
		return 0, false
	}
	return n - 1, true
}

// MustPrev consumes one step of credit and panics when n is zero.
func (n StepIndex) MustPrev() StepIndex {
	prev, ok := n.Prev()
	if !ok {
		panic("kont: step index exhausted")
	}
	return prev
}

// Allows reports whether n may be weakened to target.
func (n StepIndex) Allows(target StepIndex) bool { return target <= n }

// Weaken returns target when target is a valid weakening of n.
func (n StepIndex) Weaken(target StepIndex) (StepIndex, bool) {
	if !n.Allows(target) {
		return 0, false
	}
	return target, true
}
