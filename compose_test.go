// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

type composeUnhandledOp struct{}

func (composeUnhandledOp) OpResult() int { panic("phantom") }

func TestRunStateReader(t *testing.T) {
	// Computation that reads environment and modifies state based on it
	comp := kont.AskReader(func(env int) kont.Cont[kont.Resumed, int] {
		return kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
			return kont.PutState(s+env, kont.Perform(kont.Get[int]{}))
		})
	})

	result, finalState := kont.RunStateReader[int, int, int](10, 32, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 42 {
		t.Fatalf("got state %d, want 42", finalState)
	}
}

func TestRunStateReaderMultipleOps(t *testing.T) {
	// Interleave state and reader operations
	comp := kont.AskReader(func(prefix string) kont.Cont[kont.Resumed, string] {
		return kont.ModifyState(func(s int) int { return s + 1 }, func(newState int) kont.Cont[kont.Resumed, string] {
			return kont.AskReader(func(prefix2 string) kont.Cont[kont.Resumed, string] {
				return kont.GetState(func(s int) kont.Cont[kont.Resumed, string] {
					if prefix != prefix2 {
						return kont.Return[kont.Resumed]("mismatch")
					}
					return kont.Return[kont.Resumed](prefix)
				})
			})
		})
	})

	result, finalState := kont.RunStateReader[int, string, string](0, "hello", comp)
	if result != "hello" {
		t.Fatalf("got result %q, want %q", result, "hello")
	}
	if finalState != 1 {
		t.Fatalf("got state %d, want 1", finalState)
	}
}

func TestRunStateReaderPure(t *testing.T) {
	// Pure computation should pass through both handlers
	comp := kont.Return[kont.Resumed, int](42)

	result, finalState := kont.RunStateReader[int, string, int](100, "env", comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 100 {
		t.Fatalf("got state %d, want 100 (unchanged)", finalState)
	}
}

func TestExprStateReader(t *testing.T) {
	// Computation that reads environment and modifies state based on it
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(env int) kont.Expr[int] {
		return kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + env}), kont.ExprPerform(kont.Get[int]{}))
		})
	})

	result, finalState := kont.RunStateReaderExpr[int, int, int](10, 32, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 42 {
		t.Fatalf("got state %d, want 42", finalState)
	}
}

func TestExprStateReaderMultipleOps(t *testing.T) {
	// Interleave state and reader operations
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[string]{}), func(prefix string) kont.Expr[string] {
		return kont.ExprBind(kont.ExprPerform(kont.Modify[int]{F: func(s int) int { return s + 1 }}), func(newState int) kont.Expr[string] {
			return kont.ExprBind(kont.ExprPerform(kont.Ask[string]{}), func(prefix2 string) kont.Expr[string] {
				return kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[string] {
					if prefix != prefix2 {
						return kont.ExprReturn("mismatch")
					}
					return kont.ExprReturn(prefix)
				})
			})
		})
	})

	result, finalState := kont.RunStateReaderExpr[int, string, string](0, "hello", comp)
	if result != "hello" {
		t.Fatalf("got result %q, want %q", result, "hello")
	}
	if finalState != 1 {
		t.Fatalf("got state %d, want 1", finalState)
	}
}

func TestExprStateReaderPure(t *testing.T) {
	// Pure computation should pass through both handlers
	comp := kont.ExprReturn[int](42)

	result, finalState := kont.RunStateReaderExpr[int, string, int](100, "env", comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 100 {
		t.Fatalf("got state %d, want 100 (unchanged)", finalState)
	}
}

func TestRunStateReaderUnhandledEffectPanics(t *testing.T) {
	comp := kont.Perform(composeUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "unhandled effect in StateReaderHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _ = kont.RunStateReader[int, int, int](0, 0, comp)
}

func TestRunStateWriterUnhandledEffectPanics(t *testing.T) {
	comp := kont.Perform(composeUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "unhandled effect in StateWriterHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _, _ = kont.RunStateWriter[int, int, int](0, comp)
}

func TestRunStateErrorUnhandledEffectPanics(t *testing.T) {
	comp := kont.Perform(composeUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "unhandled effect in StateErrorHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _ = kont.RunStateError[int, string, int](0, comp)
}

func TestRunReaderStateErrorUnhandledEffectPanics(t *testing.T) {
	comp := kont.Perform(composeUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "unhandled effect in ReaderStateErrorHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _ = kont.RunReaderStateError[int, int, string, int](0, 0, comp)
}

// --- RunStateError tests ---

func TestRunStateErrorSuccess(t *testing.T) {
	// State + Error, success path: Get → Put → Get
	comp := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(x+1, kont.Perform(kont.Get[int]{}))
	})

	either, state := kont.RunStateError[int, string, int](10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 11 {
		t.Fatalf("got %d, want 11", v)
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
}

func TestRunStateErrorThrow(t *testing.T) {
	// Throw aborts, state preserved at point of throw
	comp := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(x+1, kont.ThrowError[string, int]("fail"))
	})

	either, state := kont.RunStateError[int, string, int](10, comp)
	if !either.IsLeft() {
		t.Fatal("expected Left")
	}
	e, _ := either.GetLeft()
	if e != "fail" {
		t.Fatalf("got error %q, want %q", e, "fail")
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
}

func TestRunStateErrorCatch(t *testing.T) {
	// State ops outside Catch boundary; Catch body is error-only
	// (like Listen/Censor, Catch body only handles Error effects)
	comp := kont.PutState(99,
		kont.CatchError[string](
			kont.ThrowError[string, int]("err"),
			func(e string) kont.Cont[kont.Resumed, int] {
				return kont.Return[kont.Resumed](42)
			},
		),
	)

	either, state := kont.RunStateError[int, string, int](0, comp)
	if !either.IsRight() {
		t.Fatal("expected Right after catch")
	}
	v, _ := either.GetRight()
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
	if state != 99 {
		t.Fatalf("got state %d, want 99", state)
	}
}

func TestRunStateErrorPure(t *testing.T) {
	comp := kont.Return[kont.Resumed, int](42)
	either, state := kont.RunStateError[int, string, int](10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

func TestEvalStateError(t *testing.T) {
	comp := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.Return[kont.Resumed](x + 1)
	})
	either := kont.EvalStateError[int, string, int](10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 11 {
		t.Fatalf("got %d, want 11", v)
	}
}

func TestExecStateError(t *testing.T) {
	comp := kont.Perform(kont.Put[int]{Value: 42})
	state := kont.ExecStateError[int, string, struct{}](0, comp)
	if state != 42 {
		t.Fatalf("got state %d, want 42", state)
	}
}

func TestRunStateErrorExprSuccess(t *testing.T) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(x int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: x + 1}), kont.ExprPerform(kont.Get[int]{}))
	})

	either, state := kont.RunStateErrorExpr[int, string, int](10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 11 {
		t.Fatalf("got %d, want 11", v)
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
}

func TestRunStateErrorExprThrow(t *testing.T) {
	comp := kont.ExprThen(
		kont.ExprPerform(kont.Put[int]{Value: 99}),
		kont.ExprThrowError[string, int]("err"),
	)

	either, state := kont.RunStateErrorExpr[int, string, int](0, comp)
	if !either.IsLeft() {
		t.Fatal("expected Left")
	}
	e, _ := either.GetLeft()
	if e != "err" {
		t.Fatalf("got error %q, want %q", e, "err")
	}
	if state != 99 {
		t.Fatalf("got state %d, want 99", state)
	}
}

// --- RunStateWriter tests ---

func TestRunStateWriterSuccess(t *testing.T) {
	comp := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.TellWriter("a", kont.PutState(x+1,
			kont.TellWriter("b", kont.Perform(kont.Get[int]{}))))
	})

	result, state, output := kont.RunStateWriter[int, string, int](10, comp)
	if result != 11 {
		t.Fatalf("got result %d, want 11", result)
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
	if len(output) != 2 || output[0] != "a" || output[1] != "b" {
		t.Fatalf("got output %v, want [a b]", output)
	}
}

func TestRunStateWriterPure(t *testing.T) {
	comp := kont.Return[kont.Resumed, int](42)
	result, state, output := kont.RunStateWriter[int, string, int](10, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
	if len(output) != 0 {
		t.Fatalf("got output %v, want empty", output)
	}
}

func TestRunStateWriterExprSuccess(t *testing.T) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(x int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "hello"}),
			kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: x + 1}),
				kont.ExprPerform(kont.Get[int]{})))
	})

	result, state, output := kont.RunStateWriterExpr[int, string, int](10, comp)
	if result != 11 {
		t.Fatalf("got result %d, want 11", result)
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
	if len(output) != 1 || output[0] != "hello" {
		t.Fatalf("got output %v, want [hello]", output)
	}
}

// --- RunReaderStateError tests ---

func TestRunReaderStateErrorSuccess(t *testing.T) {
	comp := kont.AskReader(func(env string) kont.Cont[kont.Resumed, string] {
		return kont.GetState(func(x int) kont.Cont[kont.Resumed, string] {
			return kont.PutState(x+1, kont.Return[kont.Resumed](env))
		})
	})

	either, state := kont.RunReaderStateError[string, int, string, string]("hello", 10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != "hello" {
		t.Fatalf("got %q, want %q", v, "hello")
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
}

func TestRunReaderStateErrorThrow(t *testing.T) {
	comp := kont.AskReader(func(env int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(env, kont.ThrowError[string, int]("fail"))
	})

	either, state := kont.RunReaderStateError[int, int, string, int](42, 0, comp)
	if !either.IsLeft() {
		t.Fatal("expected Left")
	}
	e, _ := either.GetLeft()
	if e != "fail" {
		t.Fatalf("got error %q, want %q", e, "fail")
	}
	if state != 42 {
		t.Fatalf("got state %d, want 42", state)
	}
}

func TestRunReaderStateErrorCatch(t *testing.T) {
	// State ops outside Catch boundary; Catch body is error-only
	// (like Listen/Censor, Catch body only handles Error effects)
	comp := kont.PutState(99,
		kont.CatchError[string](
			kont.ThrowError[string, int]("err"),
			func(e string) kont.Cont[kont.Resumed, int] {
				return kont.Return[kont.Resumed](100)
			},
		),
	)

	either, state := kont.RunReaderStateError[int, int, string, int](1, 0, comp)
	if !either.IsRight() {
		t.Fatal("expected Right after catch")
	}
	v, _ := either.GetRight()
	if v != 100 {
		t.Fatalf("got %d, want 100", v)
	}
	if state != 99 {
		t.Fatalf("got state %d, want 99", state)
	}
}

func TestRunReaderStateErrorPure(t *testing.T) {
	comp := kont.Return[kont.Resumed, int](42)
	either, state := kont.RunReaderStateError[string, int, string, int]("env", 10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

func TestRunReaderStateErrorExprSuccess(t *testing.T) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(env int) kont.Expr[int] {
		return kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + env}), kont.ExprPerform(kont.Get[int]{}))
		})
	})

	either, state := kont.RunReaderStateErrorExpr[int, int, string, int](5, 10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 15 {
		t.Fatalf("got %d, want 15", v)
	}
	if state != 15 {
		t.Fatalf("got state %d, want 15", state)
	}
}

func TestRunReaderStateErrorExprThrow(t *testing.T) {
	comp := kont.ExprThen(
		kont.ExprPerform(kont.Put[int]{Value: 77}),
		kont.ExprThrowError[string, int]("boom"),
	)

	either, state := kont.RunReaderStateErrorExpr[int, int, string, int](0, 0, comp)
	if !either.IsLeft() {
		t.Fatal("expected Left")
	}
	e, _ := either.GetLeft()
	if e != "boom" {
		t.Fatalf("got error %q, want %q", e, "boom")
	}
	if state != 77 {
		t.Fatalf("got state %d, want 77", state)
	}
}

// --- Benchmarks ---

func BenchmarkRunStateReader(b *testing.B) {
	comp := kont.AskReader(func(env int) kont.Cont[kont.Resumed, int] {
		return kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
			return kont.PutState(s+env, kont.Perform(kont.Get[int]{}))
		})
	})

	for b.Loop() {
		_, _ = kont.RunStateReader[int, int, int](0, 1, comp)
	}
}

func BenchmarkRunStateErrorSuccess(b *testing.B) {
	comp := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(x+1, kont.Perform(kont.Get[int]{}))
	})

	for b.Loop() {
		_, _ = kont.RunStateError[int, string, int](0, comp)
	}
}

func BenchmarkRunStateErrorThrow(b *testing.B) {
	comp := kont.PutState(1, kont.ThrowError[string, int]("err"))

	for b.Loop() {
		_, _ = kont.RunStateError[int, string, int](0, comp)
	}
}

func BenchmarkRunStateErrorCatch(b *testing.B) {
	comp := kont.CatchError[string](
		kont.ThrowError[string, int]("err"),
		func(e string) kont.Cont[kont.Resumed, int] {
			return kont.Return[kont.Resumed](0)
		},
	)

	for b.Loop() {
		_, _ = kont.RunStateError[int, string, int](0, comp)
	}
}

func BenchmarkRunStateWriter(b *testing.B) {
	comp := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.TellWriter("a", kont.PutState(x+1, kont.Perform(kont.Get[int]{})))
	})

	for b.Loop() {
		_, _, _ = kont.RunStateWriter[int, string, int](0, comp)
	}
}

func BenchmarkRunReaderStateErrorSuccess(b *testing.B) {
	comp := kont.AskReader(func(env int) kont.Cont[kont.Resumed, int] {
		return kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
			return kont.PutState(s+env, kont.Perform(kont.Get[int]{}))
		})
	})

	for b.Loop() {
		_, _ = kont.RunReaderStateError[int, int, string, int](1, 0, comp)
	}
}

func BenchmarkRunReaderStateErrorThrow(b *testing.B) {
	comp := kont.AskReader(func(env int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(env, kont.ThrowError[string, int]("err"))
	})

	for b.Loop() {
		_, _ = kont.RunReaderStateError[int, int, string, int](42, 0, comp)
	}
}

func BenchmarkRunStateReaderExprCompose(b *testing.B) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(env int) kont.Expr[int] {
		return kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + env}), kont.ExprPerform(kont.Get[int]{}))
		})
	})

	for b.Loop() {
		_, _ = kont.RunStateReaderExpr[int, int, int](0, 1, comp)
	}
}
