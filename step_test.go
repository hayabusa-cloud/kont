// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"code.hybscloud.com/kont"
)

// --- Step (Cont path) ---

func TestStepPure(t *testing.T) {
	m := kont.Pure(42)
	result, susp := kont.Step(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure computation")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestStepExprPureNilInterface(t *testing.T) {
	result, susp := kont.StepExpr(kont.ExprReturn[any](nil))
	if susp != nil {
		t.Fatal("expected nil suspension for pure computation")
	}
	if result != nil {
		t.Fatalf("got %v, want nil", result)
	}
}

func TestStepSingleEffect(t *testing.T) {
	m := kont.Perform(kont.Get[int]{})
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	result, susp := susp.Resume(99)
	if susp != nil {
		t.Fatal("expected nil suspension after resume")
	}
	if result != 99 {
		t.Fatalf("got %d, want 99", result)
	}
}

func TestStepChainedEffects(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+10), Get))
	m := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
	})

	state := 5

	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected first suspension (Get)")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	_, susp = susp.Resume(state)

	if susp == nil {
		t.Fatal("expected second suspension (Put)")
	}
	if put, ok := susp.Op().(kont.Put[int]); !ok {
		t.Fatalf("expected Put[int], got %T", susp.Op())
	} else {
		state = put.Value
	}
	_, susp = susp.Resume(struct{}{})

	if susp == nil {
		t.Fatal("expected third suspension (Get)")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	result, susp := susp.Resume(state)

	if susp != nil {
		t.Fatal("expected nil suspension after final resume")
	}
	if result != 15 {
		t.Fatalf("got %d, want 15", result)
	}
}

func TestStepAffinePanic(t *testing.T) {
	m := kont.Perform(kont.Get[int]{})
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	susp.Resume(1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on double resume")
		}
		if r != "kont: suspension resumed twice" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	susp.Resume(2)
}

func TestStepDiscard(t *testing.T) {
	m := kont.Perform(kont.Get[int]{})
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	susp.Discard()

	// After discard, TryResume must fail
	_, _, ok := susp.TryResume(0)
	if ok {
		t.Fatal("expected TryResume to fail after Discard")
	}
}

func TestStepDiscardAfterResumeIsConsumed(t *testing.T) {
	m := kont.Perform(kont.Get[int]{})
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, next := susp.Resume(42)
	if next != nil {
		t.Fatal("expected completion")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
	susp.Discard()
}

func TestStepContOldSuspensionCannotResumeAfterNextStep(t *testing.T) {
	m := kont.Bind(kont.Perform(kont.Get[int]{}), func(s int) kont.Eff[int] {
		return kont.Bind(kont.Perform(kont.Get[int]{}), func(next int) kont.Eff[int] {
			return kont.Pure(s + next)
		})
	})
	_, first := kont.Step(m)
	if first == nil {
		t.Fatal("expected first suspension")
	}
	_, second := first.Resume(1)
	if second == nil {
		t.Fatal("expected second suspension")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on resuming consumed suspension")
		}
		if r != "kont: suspension resumed twice" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	first.Resume(2)
}

func TestStepExprDiscard(t *testing.T) {
	m := kont.ExprPerform(kont.Get[int]{})
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}

	susp.Discard()

	// After discard, TryResume must fail on the Expr path as well.
	_, _, ok := susp.TryResume(0)
	if ok {
		t.Fatal("expected TryResume to fail after Expr-path Discard")
	}
}

func TestStepExprDiscardReleasesPooledContinuationFrames(t *testing.T) {
	ef := kont.AcquireEffectFrame()
	ef.Operation = kont.Get[int]{}
	ef.Resume = func(v kont.Erased) kont.Erased { return v }

	bf := kont.AcquireBindFrame()
	bf.F = func(v kont.Erased) kont.Expr[kont.Erased] {
		return kont.ExprReturn[kont.Erased](v)
	}
	bf.Next = kont.ReturnFrame{}
	ef.Next = bf

	_, susp := kont.StepExpr(kont.ExprSuspend[int](ef))
	if susp == nil {
		t.Fatal("expected suspension")
	}
	susp.Discard()

	if ef.Operation != nil || ef.Resume != nil || ef.Next != nil {
		t.Fatalf("discard did not release effect frame: %#v", ef)
	}
	if bf.F != nil || bf.Next != nil {
		t.Fatalf("discard did not release continuation frame: %#v", bf)
	}
}

func TestStepExprDiscardReleasesNestedPooledContinuationFrames(t *testing.T) {
	ef := kont.AcquireEffectFrame()
	ef.Operation = kont.Get[int]{}
	ef.Resume = func(v kont.Erased) kont.Erased { return v }

	bf := kont.AcquireBindFrame()
	bf.F = func(v kont.Erased) kont.Expr[kont.Erased] {
		return kont.ExprReturn[kont.Erased](v)
	}
	bf.Next = kont.ReturnFrame{}

	mf := &kont.MapFrame[kont.Erased, kont.Erased]{
		F:    func(v kont.Erased) kont.Erased { return v },
		Next: kont.ReturnFrame{},
	}

	secondBF := kont.AcquireBindFrame()
	secondBF.F = func(v kont.Erased) kont.Expr[kont.Erased] {
		return kont.ExprReturn[kont.Erased](v)
	}
	secondBF.Next = kont.ReturnFrame{}

	tf := kont.AcquireThenFrame()
	tf.Second = kont.Expr[kont.Erased]{Frame: secondBF}
	tf.Next = kont.ReturnFrame{}

	nextEF := kont.AcquireEffectFrame()
	nextEF.Operation = kont.Put[int]{Value: 1}
	nextEF.Resume = func(v kont.Erased) kont.Erased { return v }
	nextEF.Next = kont.ReturnFrame{}

	uf := kont.AcquireUnwindFrame()
	uf.Data1 = 1
	uf.Unwind = func(kont.Erased, kont.Erased, kont.Erased, kont.Erased) (kont.Erased, kont.Frame) {
		return nil, kont.ReturnFrame{}
	}

	ef.Next = kont.ChainFrames(
		kont.ChainFrames(bf, mf),
		kont.ChainFrames(tf, kont.ChainFrames(nextEF, uf)),
	)

	_, susp := kont.StepExpr(kont.ExprSuspend[int](ef))
	if susp == nil {
		t.Fatal("expected suspension")
	}
	susp.Discard()

	if bf.F != nil || bf.Next != nil {
		t.Fatalf("discard did not release first bind frame: %#v", bf)
	}
	if secondBF.F != nil || secondBF.Next != nil {
		t.Fatalf("discard did not release nested bind frame: %#v", secondBF)
	}
	if tf.Second.Frame != nil || tf.Next != nil {
		t.Fatalf("discard did not release then frame: %#v", tf)
	}
	if nextEF.Operation != nil || nextEF.Resume != nil || nextEF.Next != nil {
		t.Fatalf("discard did not release nested effect frame: %#v", nextEF)
	}
	if uf.Data1 != nil || uf.Unwind != nil {
		t.Fatalf("discard did not release unwind frame: %#v", uf)
	}
}

func TestStepExprDiscardStopsAtCustomContinuationFrame(t *testing.T) {
	ef := kont.AcquireEffectFrame()
	ef.Operation = kont.Get[int]{}
	ef.Resume = func(v kont.Erased) kont.Erased { return v }
	ef.Next = &NoUnwindFrame{}

	_, susp := kont.StepExpr(kont.ExprSuspend[int](ef))
	if susp == nil {
		t.Fatal("expected suspension")
	}
	susp.Discard()

	if ef.Operation != nil || ef.Resume != nil || ef.Next != nil {
		t.Fatalf("discard did not release effect frame: %#v", ef)
	}
}

func TestStepExprNestedChainSuspendsAndResumes(t *testing.T) {
	// Build a nested chained frame structure to cover the chainedFrame flattening path
	// in evalFrames[stepProcessor].
	//
	// effect (resume with int) → map (*2) → map (+0)
	ef := &kont.EffectFrame[kont.Erased]{
		Operation: kont.Get[int]{},
		Resume:    func(v kont.Erased) kont.Erased { return v },
		Next:      kont.ReturnFrame{},
	}
	map1 := &kont.MapFrame[kont.Erased, kont.Erased]{
		F:    func(v kont.Erased) kont.Erased { return v.(int) * 2 },
		Next: kont.ReturnFrame{},
	}
	map2 := &kont.MapFrame[kont.Erased, kont.Erased]{
		F:    func(v kont.Erased) kont.Erased { return v.(int) + 0 },
		Next: kont.ReturnFrame{},
	}
	frame := kont.ChainFrames(kont.ChainFrames(ef, map1), map2)
	comp := kont.ExprSuspend[int](frame)

	result, susp := kont.StepExpr(comp)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	if result != 0 {
		t.Fatalf("got %d, want 0 (zero while pending)", result)
	}

	result, susp = susp.Resume(21)
	if susp != nil {
		t.Fatal("expected nil suspension after resume")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestStepTryResume(t *testing.T) {
	m := kont.Perform(kont.Get[int]{})
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}

	result, next, ok := susp.TryResume(42)
	if !ok {
		t.Fatal("expected ok=true on first TryResume")
	}
	if next != nil {
		t.Fatal("expected nil suspension after TryResume")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}

	_, _, ok = susp.TryResume(99)
	if ok {
		t.Fatal("expected ok=false on second TryResume")
	}
}

func TestStepWithMap(t *testing.T) {
	// Map(Perform(Get[int]{}), func(s int) int { return s * 3 })
	m := kont.Map(kont.Perform(kont.Get[int]{}), func(s int) int { return s * 3 })
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(7)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 21 {
		t.Fatalf("got %d, want 21", result)
	}
}

func TestStepWithBind(t *testing.T) {
	// Bind(Perform(Ask[string]{}), func(env string) { Return(env + "!") })
	m := kont.Bind(kont.Perform(kont.Ask[string]{}), func(env string) kont.Eff[string] {
		return kont.Pure(env + "!")
	})
	_, susp := kont.Step(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume("hello")
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != "hello!" {
		t.Fatalf("got %q, want %q", result, "hello!")
	}
}

// --- StepExpr (Expr path) ---

func TestStepExprPure(t *testing.T) {
	m := kont.ExprReturn(42)
	result, susp := kont.StepExpr(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure computation")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestStepExprSingleEffect(t *testing.T) {
	m := kont.ExprPerform(kont.Get[int]{})
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	result, susp := susp.Resume(99)
	if susp != nil {
		t.Fatal("expected nil suspension after resume")
	}
	if result != 99 {
		t.Fatalf("got %d, want 99", result)
	}
}

func TestStepExprChainedEffects(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+10), Get))
	m := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 10}),
			kont.ExprPerform(kont.Get[int]{}))
	})

	state := 5

	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected first suspension (Get)")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	_, susp = susp.Resume(state)

	if susp == nil {
		t.Fatal("expected second suspension (Put)")
	}
	if put, ok := susp.Op().(kont.Put[int]); !ok {
		t.Fatalf("expected Put[int], got %T", susp.Op())
	} else {
		state = put.Value
	}
	_, susp = susp.Resume(struct{}{})

	if susp == nil {
		t.Fatal("expected third suspension (Get)")
	}
	if _, ok := susp.Op().(kont.Get[int]); !ok {
		t.Fatalf("expected Get[int], got %T", susp.Op())
	}
	result, susp := susp.Resume(state)

	if susp != nil {
		t.Fatal("expected nil suspension after final resume")
	}
	if result != 15 {
		t.Fatalf("got %d, want 15", result)
	}
}

func TestStepExprAffinePanic(t *testing.T) {
	m := kont.ExprPerform(kont.Get[int]{})
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	susp.Resume(1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on double resume")
		}
		if r != "kont: suspension resumed twice" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	susp.Resume(2)
}

func TestStepExprOldSuspensionCannotResumeAfterNextStep(t *testing.T) {
	m := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprPerform(kont.Get[int]{})
	})
	_, first := kont.StepExpr(m)
	if first == nil {
		t.Fatal("expected first suspension")
	}
	_, second := first.Resume(1)
	if second == nil {
		t.Fatal("expected second suspension")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on resuming consumed suspension")
		}
		if r != "kont: suspension resumed twice" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	first.Resume(2)
}

func TestStepExprWithMap(t *testing.T) {
	m := kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(s int) int { return s * 3 })
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(7)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 21 {
		t.Fatalf("got %d, want 21", result)
	}
}

func TestStepExprWithBind(t *testing.T) {
	m := kont.ExprBind(kont.ExprPerform(kont.Ask[string]{}), func(env string) kont.Expr[string] {
		return kont.ExprReturn(env + "!")
	})
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume("hello")
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != "hello!" {
		t.Fatalf("got %q, want %q", result, "hello!")
	}
}

func TestStepExprPureMap(t *testing.T) {
	m := kont.ExprMap(kont.ExprReturn(10), func(x int) int { return x * 5 })
	result, susp := kont.StepExpr(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure map")
	}
	if result != 50 {
		t.Fatalf("got %d, want 50", result)
	}
}

func TestStepExprPureBind(t *testing.T) {
	m := kont.ExprBind(kont.ExprReturn(10), func(x int) kont.Expr[int] {
		return kont.ExprReturn(x + 5)
	})
	result, susp := kont.StepExpr(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure bind")
	}
	if result != 15 {
		t.Fatalf("got %d, want 15", result)
	}
}

func TestStepExprPureThen(t *testing.T) {
	m := kont.ExprThen(kont.ExprReturn("ignored"), kont.ExprReturn(42))
	result, susp := kont.StepExpr(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure then")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestStepExprReturnedSuspensionIsData(t *testing.T) {
	_, inner := kont.StepExpr(kont.ExprPerform(kont.Get[any]{}))
	defer inner.Discard()
	for _, tc := range []struct {
		name string
		want *kont.Suspension[any]
	}{{"live", inner}, {"typed nil", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			check := func(path string, got any, next *kont.Suspension[any]) {
				t.Helper()
				if got != tc.want || next != nil {
					t.Errorf("%s = (%T %v, %p), want (%p, nil)", path, got, got, next, tc.want)
				}
			}
			got, next := kont.StepExpr(kont.ExprReturn[any](tc.want))
			check("return", got, next)
			pass := &kont.MapFrame[any, any]{F: func(v any) any { return v }, Next: kont.ReturnFrame{}}
			got, next = kont.StepExpr(kont.Expr[any]{Value: tc.want, Frame: pass})
			check("map", got, next)

			_, terminal := kont.StepExpr(kont.ExprPerform(kont.Get[any]{}))
			got, next = terminal.Resume(tc.want)
			check("terminal effect", got, next)

			m := kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(int) any { return tc.want })
			_, s := kont.StepExpr(m)
			got, next = s.Resume(7)
			check("Resume", got, next)
			_, s = kont.StepExpr(m)
			got, next, ok := s.TryResume(7)
			if !ok {
				t.Fatal("TryResume rejected a fresh handle")
			}
			check("TryResume", got, next)
		})
	}
	if got, next, ok := inner.TryResume(42); !ok || got != 42 || next != nil {
		t.Fatalf("returned handle resumed to (%v, %p, %v), want (42, nil, true)", got, next, ok)
	}
}

func TestTryResumeHeterogeneousChain(t *testing.T) {
	m := kont.ExprBind(kont.ExprPerform(kont.Get[string]{}), func(s string) kont.Expr[int] {
		return kont.ExprMap(kont.ExprPerform(kont.Ask[int]{}), func(n int) int { return len(s) + n })
	})
	_, first := kont.StepExpr(m)
	if first == nil || first.Op() != (kont.Get[string]{}) {
		t.Fatal("expected an initial Get[string]")
	}
	_, second, ok := first.TryResume("abc")
	if !ok || second == nil || second.Op() != (kont.Ask[int]{}) {
		t.Fatal("expected the Ask[int] successor")
	}
	if _, _, ok := first.TryResume("again"); ok {
		t.Fatal("the first handle accepted a second reply")
	}
	if got, next, ok := second.TryResume(39); got != 42 || next != nil || !ok {
		t.Fatalf("second resume = (%d, %p, %v)", got, next, ok)
	}
}

func TestTryResumeNilInterfaceReply(t *testing.T) {
	accept := func(v error) bool { return v == nil }
	for _, tc := range []struct {
		name string
		step func() (bool, *kont.Suspension[bool])
	}{
		{"Cont", func() (bool, *kont.Suspension[bool]) {
			return kont.Step(kont.Map(kont.Perform(kont.Get[error]{}), accept))
		}},
		{"Expr", func() (bool, *kont.Suspension[bool]) {
			return kont.StepExpr(kont.ExprMap(kont.ExprPerform(kont.Get[error]{}), accept))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s := tc.step()
			if got, next, ok := s.TryResume(nil); !got || next != nil || !ok {
				t.Fatalf("nil reply = (%v, %p, %v)", got, next, ok)
			}
		})
	}
	_, s := kont.StepExpr(kont.ExprPerform(kont.Get[error]{}))
	if got, next, ok := s.TryResume(nil); got != nil || next != nil || !ok {
		t.Fatalf("terminal nil reply = (%v, %p, %v)", got, next, ok)
	}
}

func TestResumeInvalidNilOperand(t *testing.T) {
	_, s := kont.Step(kont.Perform(kont.Get[int]{}))
	defer func() {
		if recover() == nil {
			t.Fatal("an untyped nil was accepted as an int")
		}
	}()
	s.Resume(nil)
}

func TestPrimitiveAbandonmentReleasesSuffix(t *testing.T) {
	for _, abandon := range []string{"discard", "short circuit"} {
		t.Run(abandon, func(t *testing.T) {
			tail := kont.AcquireUnwindFrame()
			tail.Data1 = 17
			tail.Unwind = func(_, _, _, _ any) (any, kont.Frame) { panic("abandoned suffix") }
			root := kont.ChainFrames(kont.ExprPerform(kont.Get[int]{}).Frame, tail)
			m := kont.ExprSuspend[int](root)
			if abandon == "discard" {
				_, s := kont.StepExpr(m)
				s.Discard()
			} else {
				h := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 42, false })
				if got := kont.HandleExpr(m, h); got != 42 {
					t.Fatalf("short circuit = %d, want 42", got)
				}
			}
			if tail.Data1 != nil || tail.Unwind != nil {
				t.Fatal("abandoned pooled suffix kept its payload")
			}
		})
	}
}

func TestSuspensionContendedDiscard(t *testing.T) {
	for _, tc := range []struct {
		name string
		step func() (int, *kont.Suspension[int])
	}{
		{"Cont", func() (int, *kont.Suspension[int]) {
			return kont.Step(kont.Perform(kont.Get[int]{}))
		}},
		{"pooled Expr", func() (int, *kont.Suspension[int]) {
			f := kont.AcquireEffectFrame()
			f.Operation, f.Next = kont.Get[int]{}, kont.ReturnFrame{}
			f.Resume = func(v any) any { return v }
			return kont.StepExpr(kont.ExprSuspend[int](f))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 32 {
				_, s := tc.step()
				start := make(chan struct{})
				var wg sync.WaitGroup
				var wins atomic.Int32
				for range 2 {
					wg.Go(func() {
						<-start
						if got, next, ok := s.TryResume(29); ok {
							if got != 29 || next != nil {
								t.Errorf("resume = (%d, %p), want (29, nil)", got, next)
							}
							wins.Add(1)
						}
					})
					wg.Go(func() { <-start; s.Discard() })
				}
				close(start)
				wg.Wait()
				if wins.Load() > 1 {
					t.Fatalf("%d successful resumes", wins.Load())
				}
				if _, _, ok := s.TryResume(31); ok {
					t.Fatal("consumed handle accepted another reply")
				}
			}
		})
	}
}

// --- Benchmarks ---

func BenchmarkStepSingleEffect(b *testing.B) {
	m := kont.Perform(kont.Get[int]{})
	for b.Loop() {
		_, susp := kont.Step(m)
		susp.Resume(42)
	}
}

func BenchmarkStepChainedEffects(b *testing.B) {
	// Get → Put → Get: 3 effect suspensions
	m := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
	})
	for b.Loop() {
		_, susp := kont.Step(m)
		_, susp = susp.Resume(5)
		_, susp = susp.Resume(struct{}{})
		susp.Resume(15)
	}
}

func BenchmarkStepExprSingleEffect(b *testing.B) {
	m := kont.ExprPerform(kont.Get[int]{})
	for b.Loop() {
		_, susp := kont.StepExpr(m)
		susp.Resume(42)
	}
}

func BenchmarkStepExprChainedEffects(b *testing.B) {
	// Bind(Get, func(s) Then(Put(s+10), Get)): 3 effect suspensions
	m := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 10}),
			kont.ExprPerform(kont.Get[int]{}))
	})
	for b.Loop() {
		_, susp := kont.StepExpr(m)
		_, susp = susp.Resume(5)
		_, susp = susp.Resume(struct{}{})
		susp.Resume(15)
	}
}

func BenchmarkStepPure(b *testing.B) {
	m := kont.Pure(42)
	for b.Loop() {
		kont.Step(m)
	}
}

func BenchmarkStepExprPure(b *testing.B) {
	m := kont.ExprReturn(42)
	for b.Loop() {
		kont.StepExpr(m)
	}
}

// --- Step/StepExpr equivalence ---

func TestStepExprEquivalence(t *testing.T) {
	// Verify Step and StepExpr produce equivalent results
	// when driving the same logical computation manually.

	// Cont path: Bind(Get, func(s) Return(s*2))
	contComp := kont.GetState(func(s int) kont.Eff[int] {
		return kont.Pure(s * 2)
	})
	_, contSusp := kont.Step(contComp)
	if contSusp == nil {
		t.Fatal("Cont: expected suspension")
	}
	contResult, contSusp := contSusp.Resume(21)
	if contSusp != nil {
		t.Fatal("Cont: expected nil suspension")
	}

	// Expr path: ExprBind(Get, func(s) ExprReturn(s*2))
	exprComp := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprReturn(s * 2)
	})
	_, exprSusp := kont.StepExpr(exprComp)
	if exprSusp == nil {
		t.Fatal("Expr: expected suspension")
	}
	exprResult, exprSusp := exprSusp.Resume(21)
	if exprSusp != nil {
		t.Fatal("Expr: expected nil suspension")
	}

	if contResult != exprResult {
		t.Fatalf("Cont result %d != Expr result %d", contResult, exprResult)
	}
	if contResult != 42 {
		t.Fatalf("got %d, want 42", contResult)
	}
}
