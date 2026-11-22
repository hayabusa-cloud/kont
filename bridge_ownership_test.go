// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func acquiredBridgeExpr() (kont.Expr[int], *kont.EffectFrame[any], *kont.UnwindFrame) {
	effect := kont.AcquireEffectFrame()
	tail := kont.AcquireUnwindFrame()
	effect.Operation = kont.Get[int]{}
	effect.Resume = func(v any) any { return v.(int) + 1 }
	effect.Next = tail
	tail.Data1 = 2
	tail.Unwind = func(add, _, _, v any) (any, kont.Frame) {
		return v.(int) + add.(int), kont.ReturnFrame{}
	}
	return kont.ExprSuspend[int](effect), effect, tail
}

type bridgeAbort struct{ kont.Phantom[int] }

func (bridgeAbort) DispatchState(*int) (kont.Resumed, bool) { return 42, false }

func TestBridgeOwnsPooledFrames(t *testing.T) {
	stop := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 42, false })
	for _, path := range []string{
		"reflect resume", "reflect discard", "reflect short circuit", "reified handler short circuit", "state short circuit",
		"reify reflect discard", "abandon before reify", "discard after second operation", "round trip resume",
	} {
		t.Run(path, func(t *testing.T) {
			m, f, tail := acquiredBridgeExpr()
			if path != "reflect resume" && path != "round trip resume" {
				f.Resume = func(any) any { t.Fatal("abandoned effect resumed"); return nil }
				tail.Unwind = func(_, _, _, _ any) (any, kont.Frame) {
					t.Fatal("abandoned suffix evaluated")
					return nil, nil
				}
			}
			switch path {
			case "reflect resume":
				_, s := kont.Step(kont.Reflect(m))
				if v, next := s.Resume(39); v != 42 || next != nil {
					t.Fatal(v, next)
				}
			case "reflect discard":
				_, s := kont.Step(kont.Reflect(m))
				s.Discard()
			case "reflect short circuit":
				if got := kont.Handle(kont.Reflect(m), stop); got != 42 {
					t.Fatal(got)
				}
			case "reified handler short circuit":
				if got := kont.HandleExpr(kont.Reify(kont.Reflect(m)), stop); got != 42 {
					t.Fatal(got)
				}
			case "state short circuit":
				f.Operation = bridgeAbort{}
				if got, state := kont.RunState(7, kont.Reflect(m)); got != 42 || state != 7 {
					t.Fatal(got, state)
				}
			case "reify reflect discard":
				_, s := kont.StepExpr(kont.Reify(kont.Reflect(m)))
				s.Discard()
			case "abandon before reify":
				later := kont.Reify(kont.Reflect(m))
				_, s := kont.StepExpr(kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), later))
				s.Discard()
			case "discard after second operation":
				c := kont.GetState(func(string) kont.Eff[int] { return kont.Reflect(m) })
				expr := kont.ExprThen(kont.Reify(c), kont.ExprReturn(true))
				_, first := kont.StepExpr(expr)
				if first == nil || first.Op() != (kont.Get[string]{}) {
					t.Fatal("expected the initial Get[string]")
				}
				_, second, ok := first.TryResume("ready")
				if !ok || second == nil || second.Op() != (kont.Get[int]{}) {
					t.Fatal("expected the reflected operation", second, ok)
				}
				second.Discard()
			case "round trip resume":
				_, s := kont.Step(kont.Reflect(kont.Reify(kont.Reflect(m))))
				if v, next := s.Resume(39); v != 42 || next != nil {
					t.Fatal(v, next)
				}
			}
			if f.Operation != nil || f.Next != nil || f.Resume != nil {
				t.Error("effect frame kept its payload")
			}
			if tail.Data1 != nil || tail.Unwind != nil {
				t.Error("suffix kept its payload")
			}
		})
	}
}

func TestReifyTransfersCaptureOnce(t *testing.T) {
	source := kont.Perform(kont.Get[int]{})
	e := kont.Reify(source)
	_, first := kont.StepExpr(e)
	_, stale := kont.StepExpr(kont.ExprThen(kont.ExprPerform(kont.Get[string]{}), e))
	stale.Discard()
	if got, next := first.Resume(7); got != 7 || next != nil {
		t.Fatal("discarding a copied expression disturbed its owner", got, next)
	}

	_, fresh := kont.StepExpr(kont.Reify(source))
	t.Run("consumed expression", func(t *testing.T) {
		defer func() {
			if r := recover(); r != "kont: reified computation already consumed" {
				t.Errorf("second evaluation panic = %v", r)
			}
		}()
		kont.StepExpr(e)
	})
	if got, next := fresh.Resume(11); got != 11 || next != nil {
		t.Fatal("fresh reification did not complete independently", got, next)
	}
}

func TestDeepBridgeAbandonment(t *testing.T) {
	if !inSmallStackProcess(t) {
		return
	}
	stop := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 42, false })
	for _, path := range []string{"discard", "handler", "unentered capture"} {
		t.Run(path, func(t *testing.T) {
			m, effect, tail := acquiredBridgeExpr()
			effect.Resume = func(any) any { panic("abandoned effect resumed") }
			tail.Unwind = func(_, _, _, _ any) (any, kont.Frame) { panic("abandoned suffix ran") }
			for range 8192 {
				m = kont.Reify(kont.Reflect(m))
			}

			// The reflected suffix owns both the deep reply chain and its sibling.
			sibling := kont.AcquireUnwindFrame()
			sibling.Unwind = tail.Unwind
			m = kont.Reify(kont.Reflect(kont.ExprSuspend[int](kont.ChainFrames(m.Frame, sibling))))
			if path == "unentered capture" {
				m = kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), m)
			}
			if path == "handler" {
				if got := kont.HandleExpr(m, stop); got != 42 {
					t.Fatalf("terminal result = %d, want 42", got)
				}
			} else {
				_, s := kont.StepExpr(m)
				s.Discard()
			}
			if effect.Resume != nil || tail.Unwind != nil || sibling.Unwind != nil {
				t.Fatal("abandonment retained the effect or an owned suffix")
			}
		})
	}
}

func TestAbandonTypedFrameConsumesCapture(t *testing.T) {
	for _, exit := range []string{"handler", "discard"} {
		t.Run(exit, func(t *testing.T) {
			m, effect, tail := acquiredBridgeExpr()
			capture := kont.Reify(kont.Reflect(m))
			wrapper := &kont.MapFrame[int, int]{F: func(int) int { panic("abandoned map") }, Next: capture.Frame}
			outer := kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprSuspend[int](wrapper.AsFrame()))
			if exit == "handler" {
				kont.HandleExpr(outer, kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 42, false }))
			} else {
				_, s := kont.StepExpr(outer)
				s.Discard()
			}
			if effect.Resume != nil || tail.Unwind != nil {
				t.Error("capture retained its pooled frames")
			}
			defer func() {
				if r := recover(); r != "kont: reified computation already consumed" {
					t.Errorf("stepping an abandoned capture panicked with %v", r)
				}
			}()
			kont.StepExpr(capture)
		})
	}
}

func TestReifyCopiesCompete(t *testing.T) {
	const contenders = 8
	expr := kont.Reify(kont.Perform(kont.Get[int]{}))
	start := make(chan struct{})
	results := make(chan bool, contenders)
	for range contenders {
		go func(e kont.Expr[int]) {
			won := false
			defer func() {
				if r := recover(); r != nil && r != "kont: reified computation already consumed" {
					t.Errorf("unexpected evaluation panic: %v", r)
				}
				results <- won
			}()
			<-start
			_, s := kont.StepExpr(e)
			got, next := s.Resume(42)
			won = got == 42 && next == nil
			if !won {
				t.Errorf("winning evaluation returned %v, %v", got, next)
			}
		}(expr)
	}
	close(start)
	winners := 0
	for range contenders {
		if <-results {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("got %d owners, want 1", winners)
	}
}

func TestReifyComposedAnswer(t *testing.T) {
	// An effectful CPS computation may finish without invoking its continuation.
	answer := func(v kont.Resumed) kont.Eff[int] {
		return func(func(int) kont.Resumed) kont.Resumed {
			return kont.Perform(kont.Get[string]{})(func(string) kont.Resumed { return v })
		}
	}
	t.Run("nil concrete answer", func(t *testing.T) {
		e := kont.ExprMap(kont.Reify(answer(nil)), func(n int) bool { return n == 0 })
		_, s := kont.StepExpr(e)
		if got, next := s.Resume("reply"); !got || next != nil {
			t.Fatal(got, next)
		}
	})
	t.Run("check before discarding answer", func(t *testing.T) {
		e := kont.ExprThen(kont.Reify(answer("wrong type")), kont.ExprReturn(true))
		_, s := kont.StepExpr(e)
		defer func() {
			if recover() == nil {
				t.Fatal("accepted a string from a computation with an int answer")
			}
		}()
		s.Resume("reply")
	})
}

func TestBridgeEvaluationOrder(t *testing.T) {
	var starts, replies, returns int
	source := func(k func(int) kont.Resumed) kont.Resumed {
		starts++
		return kont.GetState(func(n int) kont.Eff[int] {
			replies++
			return kont.Pure(n + 1)
		})(k)
	}
	e := kont.Reify(source)
	if starts != 1 || replies != 0 {
		t.Fatal("reification advanced past the first operation", starts, replies)
	}
	c := kont.Map(kont.Reflect(e), func(n int) int {
		returns++
		return n * 2
	})
	_, s := kont.Step(c)
	if replies != 0 || returns != 0 {
		t.Fatal("observation ran a continuation", replies, returns)
	}
	if got, next := s.Resume(20); got != 42 || next != nil || starts != 1 || replies != 1 || returns != 1 {
		t.Fatal(got, next, starts, replies, returns)
	}
}

func BenchmarkBridgePooledDiscard(b *testing.B) {
	for b.Loop() {
		m, _, _ := acquiredBridgeExpr()
		_, s := kont.Step(kont.Reflect(m))
		s.Discard()
	}
}

func BenchmarkBridgePooledResume(b *testing.B) {
	for b.Loop() {
		m, _, _ := acquiredBridgeExpr()
		_, s := kont.Step(kont.Reflect(m))
		if v, next := s.Resume(39); v != 42 || next != nil {
			b.Fatal(v, next)
		}
	}
}
