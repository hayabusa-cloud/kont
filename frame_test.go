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
	cont := kont.ExprSuspend[string](frame)

	if cont.Frame != frame {
		t.Error("ExprSuspend should preserve the frame")
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
	c := kont.ExprPerform(kont.Get[int]{})

	if c.Frame == nil {
		t.Fatal("ExprPerform should produce non-nil Frame")
	}
	if _, ok := c.Frame.(*kont.EffectFrame[kont.Erased]); !ok {
		t.Errorf("ExprPerform frame type = %T, want *EffectFrame[Erased]", c.Frame)
	}
}
