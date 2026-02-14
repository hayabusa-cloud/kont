// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// CustomFrame implements Unwind to provide custom reduction logic.
type CustomFrame struct {
	kont.ReturnFrame
	Val  int
	Next kont.Frame
}

func (f *CustomFrame) Unwind(current kont.Erased) (kont.Erased, kont.Frame) {
	return current.(int) + f.Val, f.Next
}

// IncFrame increments the current value by 1.
type IncFrame struct {
	kont.ReturnFrame
	Next kont.Frame
}

func (f *IncFrame) Unwind(current kont.Erased) (kont.Erased, kont.Frame) {
	return current.(int) + 1, f.Next
}

// NoUnwindFrame embeds ReturnFrame but does not implement Unwind.
type NoUnwindFrame struct {
	kont.ReturnFrame
}

// --- Unwind dispatch tests ---

func TestUnwindIntegration(t *testing.T) {
	// 10 -> CustomFrame(+5) -> 15
	expr := kont.Expr[int]{
		Value: 10,
		Frame: &CustomFrame{Val: 5, Next: kont.ReturnFrame{}},
	}
	result := kont.RunPure(expr)
	if result != 15 {
		t.Errorf("got %v, want 15", result)
	}
}

func TestUnwindIntegrationWithBind(t *testing.T) {
	// 10 -> CustomFrame(+5) -> Bind(*2) -> 30
	bindFrame := &kont.BindFrame[kont.Erased, kont.Erased]{
		F: func(a kont.Erased) kont.Expr[kont.Erased] {
			return kont.Expr[kont.Erased]{
				Value: a.(int) * 2,
				Frame: kont.ReturnFrame{},
			}
		},
		Next: kont.ReturnFrame{},
	}
	expr := kont.Expr[int]{
		Value: 10,
		Frame: &CustomFrame{Val: 5, Next: bindFrame},
	}
	result := kont.RunPure(expr)
	if result != 30 {
		t.Errorf("got %v, want 30", result)
	}
}

func TestUnwindChainedPath(t *testing.T) {
	// Exercise the chained Unwind path in evalFrames:
	// ChainFrames(CustomFrame(+5), MapFrame(*2))
	// 10 -> CustomFrame(+5) -> 15 -> Map(*2) -> 30
	mapFrame := &kont.MapFrame[kont.Erased, kont.Erased]{
		F:    func(a kont.Erased) kont.Erased { return a.(int) * 2 },
		Next: kont.ReturnFrame{},
	}
	chain := kont.ChainFrames(&CustomFrame{Val: 5, Next: kont.ReturnFrame{}}, mapFrame)
	expr := kont.Expr[int]{Value: 10, Frame: chain}
	result := kont.RunPure(expr)
	if result != 30 {
		t.Errorf("got %v, want 30", result)
	}
}

func TestUnwindPanicNonChained(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "kont: unknown frame type" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	expr := kont.Expr[int]{Value: 42, Frame: &NoUnwindFrame{}}
	kont.RunPure(expr)
}

func TestUnwindPanicChained(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "kont: unknown frame type in chain" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	chain := kont.ChainFrames(&NoUnwindFrame{}, &kont.MapFrame[kont.Erased, kont.Erased]{
		F:    func(a kont.Erased) kont.Erased { return a },
		Next: kont.ReturnFrame{},
	})
	expr := kont.Expr[int]{Value: 42, Frame: chain}
	kont.RunPure(expr)
}

// --- Benchmarks ---

func BenchmarkDispatchOptimized(b *testing.B) {
	count := 100
	var head kont.Frame = kont.ReturnFrame{}
	for i := 0; i < count; i++ {
		head = &kont.MapFrame[kont.Erased, kont.Erased]{
			F:    func(a kont.Erased) kont.Erased { return a.(int) + 1 },
			Next: head,
		}
	}
	m := kont.Expr[int]{Value: 0, Frame: head}

	for b.Loop() {
		kont.RunPure(m)
	}
}

func BenchmarkDispatchUnwind(b *testing.B) {
	count := 100
	var head kont.Frame = kont.ReturnFrame{}
	for i := 0; i < count; i++ {
		head = &IncFrame{Next: head}
	}
	m := kont.Expr[int]{Value: 0, Frame: head}

	for b.Loop() {
		kont.RunPure(m)
	}
}
