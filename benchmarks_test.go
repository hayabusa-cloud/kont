// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"slices"
	"testing"

	"code.hybscloud.com/kont"
)

// BenchmarkWarmCycle runs four expression shapes on the same warm pool per iteration.
func BenchmarkWarmCycle(b *testing.B) {
	get := kont.ExprPerform(kont.Get[int]{})
	increment := func(v int) int { return v + 1 }
	bind := func(v int) kont.Expr[int] { return kont.ExprReturn(v + 1) }
	shapes := [...]struct {
		name     string
		depth    int
		wantGets int
		want     int
		add      func(kont.Expr[int], int) kont.Expr[int]
	}{
		{"Map", 1, 1, 2, func(e kont.Expr[int], _ int) kont.Expr[int] { return kont.ExprMap(e, increment) }},
		{"Bind", 8, 1, 9, func(e kont.Expr[int], _ int) kont.Expr[int] { return kont.ExprBind(e, bind) }},
		{"Then", 64, 65, 1, func(e kont.Expr[int], _ int) kont.Expr[int] { return kont.ExprThen(e, get) }},
		{"Mixed", 17, 6, 3, func(e kont.Expr[int], i int) kont.Expr[int] {
			switch i % 3 {
			case 0:
				return kont.ExprMap(e, increment)
			case 1:
				return kont.ExprBind(e, bind)
			default:
				return kont.ExprThen(e, get)
			}
		}},
	}
	build := func(i int) kont.Expr[int] {
		e := get
		for j := range shapes[i].depth {
			e = shapes[i].add(e, j)
		}
		return e
	}
	var ready [len(shapes)]kont.Expr[int]
	for i, shape := range shapes {
		ready[i] = build(i)
		gets := 0
		check := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
			if _, ok := op.(kont.Get[int]); !ok {
				b.Fatalf("%s yielded %T, want Get[int]", shape.name, op)
			}
			gets++
			return 1, true
		})
		if got := kont.HandleExpr(ready[i], check); gets != shape.wantGets || got != shape.want {
			b.Fatalf("%s (Get count, result) = (%d, %d), want (%d, %d)", shape.name, gets, got, shape.wantGets, shape.want)
		}
	}
	resume := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 1, true })
	stop := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 7, false })
	cycle := func(b *testing.B, run func(int)) {
		for i := range shapes {
			run(i)
		}
		b.ReportAllocs()
		for b.Loop() {
			for i := range shapes {
				run(i)
			}
		}
	}
	b.Run("Reuse", func(b *testing.B) {
		cycle(b, func(i int) {
			if got := kont.HandleExpr(ready[i], resume); got != shapes[i].want {
				b.Fatalf("%s returned %d, want %d", shapes[i].name, got, shapes[i].want)
			}
		})
	})
	b.Run("BuildAndRun", func(b *testing.B) {
		cycle(b, func(i int) {
			if got := kont.HandleExpr(build(i), resume); got != shapes[i].want {
				b.Fatalf("%s returned %d, want %d", shapes[i].name, got, shapes[i].want)
			}
		})
	})
	b.Run("ShortCircuit", func(b *testing.B) {
		cycle(b, func(i int) {
			if got := kont.HandleExpr(ready[i], stop); got != 7 {
				b.Fatalf("%s stopped with %d, want 7", shapes[i].name, got)
			}
		})
	})
	b.Run("StepDiscard", func(b *testing.B) {
		cycle(b, func(i int) {
			_, s := kont.StepExpr(ready[i])
			if s == nil {
				b.Fatalf("%s completed before its first Get", shapes[i].name)
			}
			s.Discard()
		})
	})
}

// The hand-written program starts at Get. Each reply advances it by one state.
type benchStateProgram uint8

func (pc *benchStateProgram) resume(v kont.Resumed) (int, kont.Operation) {
	switch *pc {
	case 0:
		*pc = 1
		return 0, kont.Put[int]{Value: v.(int) + 10}
	case 1:
		*pc = 2
		return 0, kont.Get[int]{}
	default:
		return v.(int), nil
	}
}

func BenchmarkStepProtocol(b *testing.B) {
	type handler func(kont.Operation) kont.Resumed
	drive := func(reply handler, value int, s *kont.Suspension[int]) int {
		for s != nil {
			value, s = s.Resume(reply(s.Op()))
		}
		return value
	}
	read := kont.Perform(kont.Get[int]{})
	cont := kont.GetState(func(v int) kont.Eff[int] {
		return kont.PutState(v+10, read)
	})
	get := kont.ExprPerform(kont.Get[int]{})
	expr := kont.ExprBind(get, func(v int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: v + 10}), get)
	})
	for _, tc := range []struct {
		name string
		run  func(handler) int
	}{
		{"Go", func(reply handler) int {
			var pc benchStateProgram
			var value int
			var op kont.Operation = kont.Get[int]{}
			for op != nil {
				value, op = pc.resume(reply(op))
			}
			return value
		}},
		{"Cont", func(reply handler) int {
			value, s := kont.Step(cont)
			return drive(reply, value, s)
		}},
		{"Reify", func(reply handler) int {
			value, s := kont.StepExpr(kont.Reify(cont))
			return drive(reply, value, s)
		}},
		{"Expr", func(reply handler) int {
			value, s := kont.StepExpr(expr)
			return drive(reply, value, s)
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			state := 5
			reply := func(op kont.Operation) kont.Resumed {
				switch op := op.(type) {
				case kont.Get[int]:
					return state
				case kont.Put[int]:
					state = op.Value
					return struct{}{}
				default:
					b.Fatalf("unexpected operation %T", op)
					return nil
				}
			}
			var trace []kont.Operation
			got := tc.run(func(op kont.Operation) kont.Resumed {
				trace = append(trace, op)
				return reply(op)
			})
			want := []kont.Operation{kont.Get[int]{}, kont.Put[int]{Value: 15}, kont.Get[int]{}}
			if !slices.Equal(trace, want) || got != 15 || state != 15 {
				b.Fatalf("trace %v, result %d, state %d", trace, got, state)
			}
			b.ReportAllocs()
			initial := 0
			for b.Loop() {
				initial = 1 - initial
				state = initial
				if result := tc.run(reply); result != initial+10 || state != initial+10 {
					b.Fatalf("got value %d and state %d, want %d for both", result, state, initial+10)
				}
			}
		})
	}
}
