// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// --- Reify (Cont → Expr) ---

func TestReifyPure(t *testing.T) {
	cont := kont.Pure(42)
	expr := kont.Reify(cont)
	result := kont.RunPure(expr)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestReifyState(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+10), Get))
	cont := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
	})
	expr := kont.Reify(cont)
	result, state := kont.RunStateExpr[int, int](0, expr)
	if result != 10 {
		t.Fatalf("got result %d, want 10", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

func TestReifyReader(t *testing.T) {
	cont := kont.AskReader(func(e string) kont.Eff[string] {
		return kont.Pure(e + "!")
	})
	expr := kont.Reify(cont)
	result := kont.RunReaderExpr[string, string]("hello", expr)
	if result != "hello!" {
		t.Fatalf("got %q, want %q", result, "hello!")
	}
}

func TestReifyWriter(t *testing.T) {
	cont := kont.TellWriter("msg", kont.Pure(42))
	expr := kont.Reify(cont)
	result, logs := kont.RunWriterExpr[string, int](expr)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 1 || logs[0] != "msg" {
		t.Fatalf("got logs %v, want [msg]", logs)
	}
}

func TestReifyError(t *testing.T) {
	cont := kont.ThrowError[string, int]("fail")
	expr := kont.Reify(cont)
	either := kont.RunErrorExpr[string, int](expr)
	if !either.IsLeft() {
		t.Fatal("expected Left")
	}
	e, _ := either.GetLeft()
	if e != "fail" {
		t.Fatalf("got %q, want %q", e, "fail")
	}
}

func TestReifyChained(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+1), Bind(Get, func(s) Then(Put(s+1), Get))))
	cont := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s+1, kont.GetState(func(s2 int) kont.Eff[int] {
			return kont.PutState(s2+1, kont.Perform(kont.Get[int]{}))
		}))
	})
	expr := kont.Reify(cont)
	result, state := kont.RunStateExpr[int, int](0, expr)
	if result != 2 {
		t.Fatalf("got result %d, want 2", result)
	}
	if state != 2 {
		t.Fatalf("got state %d, want 2", state)
	}
}

// --- Reflect (Expr → Cont) ---

func TestReflectPure(t *testing.T) {
	expr := kont.ExprReturn(42)
	cont := kont.Reflect(expr)
	result := kont.Handle(cont, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("no effects expected")
	}))
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestReflectState(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+10), Get))
	expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 10}),
			kont.ExprPerform(kont.Get[int]{}))
	})
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](0, cont)
	if result != 10 {
		t.Fatalf("got result %d, want 10", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

func TestReflectReader(t *testing.T) {
	expr := kont.ExprBind(kont.ExprPerform(kont.Ask[string]{}), func(e string) kont.Expr[string] {
		return kont.ExprReturn(e + "!")
	})
	cont := kont.Reflect(expr)
	result := kont.RunReader[string, string]("hello", cont)
	if result != "hello!" {
		t.Fatalf("got %q, want %q", result, "hello!")
	}
}

func TestReflectWriter(t *testing.T) {
	expr := kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "msg"}),
		kont.ExprReturn(42))
	cont := kont.Reflect(expr)
	result, logs := kont.RunWriter[string, int](cont)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 1 || logs[0] != "msg" {
		t.Fatalf("got logs %v, want [msg]", logs)
	}
}

func TestReflectError(t *testing.T) {
	expr := kont.ExprThrowError[string, int]("fail")
	cont := kont.Reflect(expr)
	either := kont.RunError[string, int](cont)
	if !either.IsLeft() {
		t.Fatal("expected Left")
	}
	e, _ := either.GetLeft()
	if e != "fail" {
		t.Fatalf("got %q, want %q", e, "fail")
	}
}

func TestReflectChained(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+1), Bind(Get, func(s) Then(Put(s+1), Get))))
	expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 1}),
			kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s2 int) kont.Expr[int] {
				return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s2 + 1}),
					kont.ExprPerform(kont.Get[int]{}))
			}))
	})
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](0, cont)
	if result != 2 {
		t.Fatalf("got result %d, want 2", result)
	}
	if state != 2 {
		t.Fatalf("got state %d, want 2", state)
	}
}

// --- Round-trips ---

func TestRoundTripReifyReflect(t *testing.T) {
	// Cont → Expr → Cont
	original := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s*2, kont.Perform(kont.Get[int]{}))
	})
	expr := kont.Reify(original)
	roundTripped := kont.Reflect(expr)
	result, state := kont.RunState[int, int](5, roundTripped)
	if result != 10 {
		t.Fatalf("got result %d, want 10", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

func TestRoundTripReflectReify(t *testing.T) {
	// Expr → Cont → Expr
	original := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s * 2}),
			kont.ExprPerform(kont.Get[int]{}))
	})
	cont := kont.Reflect(original)
	roundTripped := kont.Reify(cont)
	result, state := kont.RunStateExpr[int, int](5, roundTripped)
	if result != 10 {
		t.Fatalf("got result %d, want 10", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// --- Reify composed with Expr combinators (regression: EffectFrame.Next in chained path) ---

func TestReifyComposedWithExprBind(t *testing.T) {
	// Multi-effect Cont: Get → Put(s+10) → Get
	cont := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
	})
	// Reify then compose with ExprBind — exercises EffectFrame.Next in chained path
	composed := kont.ExprBind(kont.Reify(cont), func(a int) kont.Expr[int] {
		return kont.ExprReturn(a + 100)
	})
	result, state := kont.RunStateExpr[int, int](5, composed)
	if result != 115 {
		t.Fatalf("got result %d, want 115", result)
	}
	if state != 15 {
		t.Fatalf("got state %d, want 15", state)
	}
}

func TestReifyComposedWithExprMap(t *testing.T) {
	// Multi-effect Cont: Get → Put(s+10) → Get
	cont := kont.GetState(func(s int) kont.Eff[int] {
		return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
	})
	// Reify then compose with ExprMap — exercises EffectFrame.Next in chained path
	mapped := kont.ExprMap(kont.Reify(cont), func(a int) int { return a * 2 })
	result, state := kont.RunStateExpr[int, int](5, mapped)
	if result != 30 {
		t.Fatalf("got result %d, want 30", result)
	}
	if state != 15 {
		t.Fatalf("got state %d, want 15", state)
	}
}

// --- Benchmarks ---

func BenchmarkReifyState(b *testing.B) {
	for b.Loop() {
		cont := kont.GetState(func(s int) kont.Eff[int] {
			return kont.PutState(s+1, kont.Perform(kont.Get[int]{}))
		})
		expr := kont.Reify(cont)
		kont.RunStateExpr[int, int](0, expr)
	}
}

func BenchmarkReflectState(b *testing.B) {
	for b.Loop() {
		expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 1}),
				kont.ExprPerform(kont.Get[int]{}))
		})
		cont := kont.Reflect(expr)
		kont.RunState[int, int](0, cont)
	}
}

func BenchmarkRoundTripReifyReflect(b *testing.B) {
	for b.Loop() {
		cont := kont.GetState(func(s int) kont.Eff[int] {
			return kont.Pure(s * 2)
		})
		expr := kont.Reify(cont)
		roundTripped := kont.Reflect(expr)
		kont.RunState[int, int](5, roundTripped)
	}
}
