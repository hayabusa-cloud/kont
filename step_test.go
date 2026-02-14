// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// --- Step (Cont path) ---

func TestStepPure(t *testing.T) {
	m := kont.Return[kont.Resumed](42)
	result, susp := kont.Step(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure computation")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
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
	m := kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
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
	m := kont.Bind(kont.Perform(kont.Ask[string]{}), func(env string) kont.Cont[kont.Resumed, string] {
		return kont.Return[kont.Resumed](env + "!")
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
	m := kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
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
	m := kont.Return[kont.Resumed](42)
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
	contComp := kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
		return kont.Return[kont.Resumed](s * 2)
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
