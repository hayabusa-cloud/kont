// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"strconv"
	"testing"

	"code.hybscloud.com/kont"
)

type exprPair struct{ x, y int }

func TestExprReturnWideAllocations(t *testing.T) {
	m := kont.ExprReturn(exprPair{1025, 2049})
	h := kont.HandleFunc[exprPair](func(kont.Operation) (kont.Resumed, bool) { panic("unexpected effect") })
	allocs := testing.AllocsPerRun(100, func() {
		v, pending := kont.StepExpr(m)
		if v != m.Value || pending != nil || kont.HandleExpr(m, h) != v || kont.RunPure(m) != v {
			t.Fatal("completed expression lost its value")
		}
		if got, state := kont.RunStateExpr(17, m); got != v || state != 17 {
			t.Fatalf("completed state expression returned (%v, %d), want (%v, 17)", got, state, v)
		}
	})
	if allocs != 0 {
		t.Fatalf("completed expression allocated %g times per run", allocs)
	}
}

func TestExprMapRunPreservesSuffix(t *testing.T) {
	type count int
	first := kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(n int) count { return count(n + 1) })
	first = kont.ExprMap(first, func(n count) count { return 2 * n })
	first = kont.ExprMap(first, func(n count) count { return n + 3 })
	converted := kont.ExprMap(first, func(n count) int { return int(n) - 1 })
	bound := kont.ExprBind(converted, func(n int) kont.Expr[int] {
		return kont.ExprMap(kont.ExprPerform(kont.Ask[int]{}), func(reply int) int { return n + reply })
	})
	m := kont.ExprMap(bound, func(n int) int { return 3 * n })
	for range 2 {
		_, s := kont.StepExpr(m)
		if s == nil {
			t.Fatal("expected Get")
		}
		if _, ok := s.Op().(kont.Get[int]); !ok {
			t.Fatalf("first operation is %T", s.Op())
		}
		_, s = s.Resume(4)
		if s == nil {
			t.Fatal("expected Ask after map run")
		}
		if _, ok := s.Op().(kont.Ask[int]); !ok {
			t.Fatalf("second operation is %T", s.Op())
		}
		if got, next := s.Resume(7); got != 57 || next != nil {
			t.Fatalf("got (%d, %v), want (57, nil)", got, next)
		}
	}
}

func TestExprCompositionPreservesInitialValue(t *testing.T) {
	for _, seed := range []int{0, 17} {
		calls := 0
		frame := &kont.MapFrame[int, int]{
			F: func(v int) int {
				calls++
				if v != seed {
					t.Fatalf("first frame got %d, want %d", v, seed)
				}
				return v + 1
			},
			Next: kont.ReturnFrame{},
		}
		m := kont.Expr[int]{Value: seed, Frame: frame.AsFrame()}
		want := strconv.Itoa(seed + 1)
		if got := kont.RunPure(kont.ExprMap(m, strconv.Itoa)); got != want {
			t.Fatalf("map got %q, want %q", got, want)
		}
		bound := kont.ExprBind(m, func(v int) kont.Expr[string] { return kont.ExprReturn(strconv.Itoa(v)) })
		if got := kont.RunPure(bound); got != want {
			t.Fatalf("bind got %q, want %q", got, want)
		}
		if got := kont.RunPure(kont.ExprThen(m, kont.ExprReturn("done"))); got != "done" || calls != 3 {
			t.Fatalf("then got %q after %d calls", got, calls)
		}
	}
}

func TestExprCompositionNilOperand(t *testing.T) {
	m := kont.ExprBind(kont.ExprPerform(kont.Get[any]{}), func(v any) kont.Expr[any] {
		if v != nil {
			t.Fatalf("bind operand = %v", v)
		}
		return kont.ExprReturn[any](nil)
	})
	result := kont.ExprMap(m, func(v any) bool { return v == nil })
	h := kont.HandleFunc[bool](func(kont.Operation) (kont.Resumed, bool) { return nil, true })
	if !kont.HandleExpr(result, h) {
		t.Fatal("map lost its nil operand")
	}
}

func TestExprThenAbandonsSecondEffect(t *testing.T) {
	second := kont.AcquireEffectFrame()
	second.Operation = kont.Ask[int]{}
	second.Resume = func(kont.Erased) kont.Erased { panic("abandoned resume") }
	tail := kont.AcquireUnwindFrame()
	tail.Unwind = func(_, _, _, _ kont.Erased) (kont.Erased, kont.Frame) { panic("abandoned unwind") }
	second.Next = tail
	m := kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprSuspend[int](second))
	h := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 42, false })
	if got := kont.HandleExpr(m, h); got != 42 || second.Next != nil || tail.Unwind != nil {
		t.Fatal("early return lost its result or retained a pooled successor")
	}
}

func BenchmarkExprComposition(b *testing.B) {
	const depth = 8
	increment := func(x int) int { return x + 1 }
	bind := func(x int) kont.Expr[int] { return kont.ExprReturn(x + 1) }
	step := kont.ExprPerform(kont.Get[int]{})
	handler := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 1, true })

	for _, tc := range []struct {
		name   string
		append func(kont.Expr[int], int) kont.Expr[int]
		want   int
	}{
		{"Map", func(e kont.Expr[int], _ int) kont.Expr[int] { return kont.ExprMap(e, increment) }, 9},
		{"Bind", func(e kont.Expr[int], _ int) kont.Expr[int] { return kont.ExprBind(e, bind) }, 9},
		{"Then", func(e kont.Expr[int], _ int) kont.Expr[int] { return kont.ExprThen(e, step) }, 1},
		{"Mixed", func(e kont.Expr[int], i int) kont.Expr[int] {
			switch i % 3 {
			case 0:
				return kont.ExprMap(e, increment)
			case 1:
				return kont.ExprBind(e, bind)
			default:
				return kont.ExprThen(e, step)
			}
		}, 3},
	} {
		build := func() kont.Expr[int] {
			e := step
			for i := range depth {
				e = tc.append(e, i)
			}
			return e
		}
		b.Run(tc.name+"/Reuse", func(b *testing.B) {
			e := build()
			_ = kont.HandleExpr(e, handler)
			b.ReportAllocs()
			for b.Loop() {
				if got := kont.HandleExpr(e, handler); got != tc.want {
					b.Fatalf("got %d, want %d", got, tc.want)
				}
			}
		})
		b.Run(tc.name+"/BuildAndRun", func(b *testing.B) {
			_ = kont.HandleExpr(build(), handler)
			b.ReportAllocs()
			for b.Loop() {
				if got := kont.HandleExpr(build(), handler); got != tc.want {
					b.Fatalf("got %d, want %d", got, tc.want)
				}
			}
		})
	}
}

func BenchmarkExprMapWide(b *testing.B) {
	var reply kont.Resumed = exprPair{1025, 2049}
	h := kont.HandleFunc[exprPair](func(kont.Operation) (kont.Resumed, bool) { return reply, true })
	m := kont.ExprPerform(kont.Get[exprPair]{})
	for range 8 {
		m = kont.ExprMap(m, func(p exprPair) exprPair { return exprPair{p.x + 1, p.y + 1} })
	}
	_ = kont.HandleExpr(m, h)
	b.ReportAllocs()
	for b.Loop() {
		if got := kont.HandleExpr(m, h); got != (exprPair{1033, 2057}) {
			b.Fatal(got)
		}
	}
}
