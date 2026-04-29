// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestStepIndexPrevAndWeaken(t *testing.T) {
	n := kont.StepIndex(3)
	if n.IsZero() {
		t.Fatal("positive index must not report zero")
	}
	prev, ok := n.Prev()
	if !ok || prev != 2 {
		t.Fatalf("Prev: got (%d, %v), want (2, true)", prev, ok)
	}
	if got := n.MustPrev(); got != 2 {
		t.Fatalf("MustPrev: got %d, want 2", got)
	}
	if !kont.StepIndex(0).IsZero() {
		t.Fatal("zero index must report zero")
	}
	if _, ok := kont.StepIndex(0).Prev(); ok {
		t.Fatal("zero index must not have a predecessor")
	}
	if !n.Allows(0) || !n.Allows(3) || n.Allows(4) {
		t.Fatalf("unexpected weakening relation for %d", n)
	}
	if got, ok := n.Weaken(1); !ok || got != 1 {
		t.Fatalf("Weaken: got (%d, %v), want (1, true)", got, ok)
	}
	if _, ok := n.Weaken(4); ok {
		t.Fatal("cannot weaken to a larger index")
	}
}

func TestStepIndexAllowsLaws(t *testing.T) {
	strong := kont.StepIndex(8)
	mid := kont.StepIndex(5)
	weak := kont.StepIndex(2)

	if !strong.Allows(strong) {
		t.Fatal("step-index weakening should be reflexive")
	}
	if !strong.Allows(mid) || !mid.Allows(weak) || !strong.Allows(weak) {
		t.Fatal("step-index weakening should be transitive")
	}
	if !strong.Allows(0) || kont.StepIndex(0).Allows(1) {
		t.Fatal("zero should be the weakest index")
	}
}

func TestStepIndexWeakenLaws(t *testing.T) {
	strong := kont.StepIndex(9)
	mid, ok := strong.Weaken(6)
	if !ok || mid != 6 {
		t.Fatalf("first weakening: got (%d, %v), want (6, true)", mid, ok)
	}
	weak, ok := mid.Weaken(3)
	if !ok || weak != 3 {
		t.Fatalf("second weakening: got (%d, %v), want (3, true)", weak, ok)
	}
	direct, ok := strong.Weaken(weak)
	if !ok || direct != weak {
		t.Fatalf("direct weakening: got (%d, %v), want (%d, true)", direct, ok, weak)
	}
}

func TestStepIndexPrevIsValidWeakening(t *testing.T) {
	n := kont.StepIndex(7)
	prev, ok := n.Prev()
	if !ok {
		t.Fatal("positive index should have a predecessor")
	}
	if prev != 6 || !n.Allows(prev) || prev.Allows(n) {
		t.Fatalf("unexpected predecessor relation: n=%d prev=%d", n, prev)
	}
}

func TestStepIndexMustPrevPanicsAtZero(t *testing.T) {
	defer func() {
		if r := recover(); r != "kont: step index exhausted" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_ = kont.StepIndex(0).MustPrev()
}
