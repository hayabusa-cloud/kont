// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestExprReturn(t *testing.T) {
	cont := kont.ExprReturn(42)

	if cont.Value != 42 {
		t.Errorf("ExprReturn(42).Value = %v, want 42", cont.Value)
	}

	if _, ok := cont.Frame.(kont.ReturnFrame); !ok {
		t.Errorf("ExprReturn(42).Frame should be ReturnFrame, got %T", cont.Frame)
	}
}

func TestExprSuspend(t *testing.T) {
	frame := &kont.BindFrame[int, string]{
		F:    func(i int) kont.Expr[string] { return kont.ExprReturn("") },
		Next: kont.ReturnFrame{},
	}
	cont := kont.ExprSuspend[string](frame.AsFrame())

	if cont.Frame != frame.AsFrame() {
		t.Error("ExprSuspend should preserve the frame")
	}
}

func TestAsFramePreservesDescriptor(t *testing.T) {
	mapper := &kont.MapFrame[int, int]{Next: kont.ReturnFrame{}}
	binder := &kont.BindFrame[int, int]{Next: mapper.AsFrame()}
	seq := &kont.ThenFrame[int, int]{Next: kont.ReturnFrame{}}
	m := kont.ExprSuspend[int](seq.AsFrame())

	seq.Second = kont.Expr[int]{Value: 20, Frame: binder.AsFrame()}
	binder.F = func(n int) kont.Expr[int] { return kont.ExprReturn(n + 1) }
	mapper.F = func(n int) int { return n * 2 }
	if got := kont.RunPure(m); got != 42 {
		t.Fatalf("projected frames returned %d, want 42", got)
	}

	var frame kont.Frame
	allocs := testing.AllocsPerRun(100, func() { frame = seq.AsFrame() })
	if allocs != 0 || frame != m.Frame {
		t.Fatalf("AsFrame allocated %g times or changed identity", allocs)
	}
}

func TestAsFramePreservesErasedIdentity(t *testing.T) {
	bind := kont.AcquireBindFrame()
	seq := kont.AcquireThenFrame()
	mapper := &kont.MapFrame[any, any]{Next: kont.ReturnFrame{}}
	if bind.AsFrame() != bind || seq.AsFrame() != seq || mapper.AsFrame() != mapper {
		t.Fatal("AsFrame changed the identity of an erased frame")
	}
	bind.F = func(v any) kont.Expr[any] { return kont.ExprReturn(v) }
	bind.Next = mapper
	mapper.F = func(v any) any { return v }
	seq.Second = kont.Expr[any]{Value: 42, Frame: bind}
	seq.Next = kont.ReturnFrame{}
	if got := kont.RunPure(kont.ExprSuspend[int](seq)); got != 42 || bind.F != nil || seq.Second.Frame != nil {
		t.Fatal("erased frames lost their result or pool ownership")
	}
}

func TestBindFrameStructure(t *testing.T) {
	// Test that BindFrame can hold a function and next frame
	called := false
	frame := &kont.BindFrame[int, string]{
		F: func(i int) kont.Expr[string] {
			called = true
			return kont.ExprReturn("done")
		},
		Next: kont.ReturnFrame{},
	}

	// Call the function
	result := frame.F(42)
	if !called {
		t.Error("F should be callable")
	}
	if result.Value != "done" {
		t.Errorf("F(42).Value = %v, want \"done\"", result.Value)
	}
}

func TestMapFrameStructure(t *testing.T) {
	frame := &kont.MapFrame[int, string]{
		F: func(i int) string {
			return "mapped"
		},
		Next: kont.ReturnFrame{},
	}

	result := frame.F(42)
	if result != "mapped" {
		t.Errorf("F(42) = %v, want \"mapped\"", result)
	}
}

func TestThenFrameStructure(t *testing.T) {
	frame := &kont.ThenFrame[int, string]{
		Second: kont.ExprReturn("second"),
		Next:   kont.ReturnFrame{},
	}

	if frame.Second.Value != "second" {
		t.Errorf("Second.Value = %v, want \"second\"", frame.Second.Value)
	}
}

func TestEffectFrameStructure(t *testing.T) {
	called := false
	frame := &kont.EffectFrame[int]{
		Resume: func(i int) any {
			called = true
			return i * 2
		},
		Next: kont.ReturnFrame{},
	}

	result := frame.Resume(21)
	if !called {
		t.Error("Resume should be callable")
	}
	if result != 42 {
		t.Errorf("Resume(21) = %v, want 42", result)
	}
}

func TestEffectFrameOperation(t *testing.T) {
	frame := &kont.EffectFrame[int]{
		Operation: kont.Get[int]{},
		Resume:    func(i int) any { return i },
		Next:      kont.ReturnFrame{},
	}

	if frame.Operation == nil {
		t.Fatal("EffectFrame.Operation should not be nil")
	}
	if _, ok := frame.Operation.(kont.Get[int]); !ok {
		t.Errorf("EffectFrame.Operation = %T, want Get[int]", frame.Operation)
	}
}

func TestBindFrameUnwind(t *testing.T) {
	frame := &kont.BindFrame[int, int]{
		F: func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 2)
		},
		Next: kont.ReturnFrame{},
	}
	result, next := frame.Unwind(21)
	if result.(int) != 42 {
		t.Fatalf("Unwind result = %v, want 42", result)
	}
	if _, ok := next.(kont.ReturnFrame); !ok {
		t.Fatalf("Unwind next = %T, want ReturnFrame", next)
	}
}

func TestMapFrameUnwind(t *testing.T) {
	frame := &kont.MapFrame[int, int]{
		F:    func(x int) int { return x * 2 },
		Next: kont.ReturnFrame{},
	}
	result, next := frame.Unwind(21)
	if result.(int) != 42 {
		t.Fatalf("Unwind result = %v, want 42", result)
	}
	if _, ok := next.(kont.ReturnFrame); !ok {
		t.Fatalf("Unwind next = %T, want ReturnFrame", next)
	}
}

func TestThenFrameUnwind(t *testing.T) {
	frame := &kont.ThenFrame[int, string]{
		Second: kont.ExprReturn("hello"),
		Next:   kont.ReturnFrame{},
	}
	result, next := frame.Unwind(999)
	if result.(string) != "hello" {
		t.Fatalf("Unwind result = %v, want hello", result)
	}
	if _, ok := next.(kont.ReturnFrame); !ok {
		t.Fatalf("Unwind next = %T, want ReturnFrame", next)
	}
}

func TestExprPerform(t *testing.T) {
	m := kont.ExprPerform(kont.Get[int]{})
	for _, reply := range []int{41, 42} {
		_, s := kont.StepExpr(m)
		if _, ok := s.Op().(kont.Get[int]); !ok {
			t.Fatalf("operation = %T, want Get[int]", s.Op())
		}
		if v, next := s.Resume(reply); v != reply || next != nil {
			t.Fatalf("resume = (%d, %p), want (%d, nil)", v, next, reply)
		}
	}
}

func TestExprPerformTypedNilOperation(t *testing.T) {
	var op *kont.Get[int]
	m := kont.ExprPerform(op)
	h := kont.HandleFunc[int](func(got kont.Operation) (kont.Resumed, bool) {
		if got != op {
			t.Fatalf("operation = %#v, want typed nil %T", got, op)
		}
		return 42, true
	})
	if got := kont.HandleExpr(m, h); got != 42 {
		t.Fatalf("result = %d, want 42", got)
	}
}
