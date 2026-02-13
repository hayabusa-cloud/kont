// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestStateGetPut(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+1), Get))
	comp := kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(s+1, kont.Perform(kont.Get[int]{}))
	})

	result, finalState := kont.RunState[int, int](10, comp)
	if result != 11 {
		t.Fatalf("got result %d, want 11", result)
	}
	if finalState != 11 {
		t.Fatalf("got state %d, want 11", finalState)
	}
}

func TestStateModify(t *testing.T) {
	comp := kont.ModifyState(func(s int) int { return s * 2 }, func(s int) kont.Cont[kont.Resumed, int] {
		return kont.Return[kont.Resumed](s)
	})

	result, finalState := kont.RunState[int, int](21, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 42 {
		t.Fatalf("got state %d, want 42", finalState)
	}
}

func TestStateEval(t *testing.T) {
	comp := kont.PutState(100, kont.Perform(kont.Get[int]{}))

	result := kont.EvalState[int, int](0, comp)
	if result != 100 {
		t.Fatalf("got %d, want 100", result)
	}
}

func TestStateExec(t *testing.T) {
	comp := kont.PutState(50, kont.Return[kont.Resumed]("done"))

	finalState := kont.ExecState[int, string](0, comp)
	if finalState != 50 {
		t.Fatalf("got state %d, want 50", finalState)
	}
}

func TestStateChained(t *testing.T) {
	// Multiple state updates in sequence
	comp := kont.PutState(1,
		kont.ModifyState(func(x int) int { return x + 1 }, func(_ int) kont.Cont[kont.Resumed, int] {
			return kont.ModifyState(func(x int) int { return x * 2 }, func(_ int) kont.Cont[kont.Resumed, int] {
				return kont.Perform(kont.Get[int]{})
			})
		}),
	)

	result, _ := kont.RunState[int, int](0, comp)
	if result != 4 { // (1 + 1) * 2 = 4
		t.Fatalf("got %d, want 4", result)
	}
}

func TestStatePure(t *testing.T) {
	// Pure value should not affect state
	comp := kont.Return[kont.Resumed, int](42)

	result, finalState := kont.RunState[int, int](100, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 100 {
		t.Fatalf("got state %d, want 100", finalState)
	}
}

func TestExprStateGetPut(t *testing.T) {
	// Bind(Get, func(s) Then(Put(s+1), Get))
	comp := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 1}), kont.ExprPerform(kont.Get[int]{}))
	})

	result, finalState := kont.RunStateExpr[int, int](10, comp)
	if result != 11 {
		t.Fatalf("got result %d, want 11", result)
	}
	if finalState != 11 {
		t.Fatalf("got state %d, want 11", finalState)
	}
}

func TestExprStateModify(t *testing.T) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Modify[int]{F: func(s int) int { return s * 2 }}), func(s int) kont.Expr[int] {
		return kont.ExprReturn(s)
	})

	result, finalState := kont.RunStateExpr[int, int](21, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 42 {
		t.Fatalf("got state %d, want 42", finalState)
	}
}

func TestExprStateEval(t *testing.T) {
	comp := kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: 100}), kont.ExprPerform(kont.Get[int]{}))

	result, _ := kont.RunStateExpr[int, int](0, comp)
	if result != 100 {
		t.Fatalf("got %d, want 100", result)
	}
}

func TestExprStateExec(t *testing.T) {
	comp := kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: 50}), kont.ExprReturn("done"))

	_, finalState := kont.RunStateExpr[int, string](0, comp)
	if finalState != 50 {
		t.Fatalf("got state %d, want 50", finalState)
	}
}

func TestExprStateChained(t *testing.T) {
	// Then(Put(1), Bind(Modify(+1), func(_) Then(Modify(*2), Get)))
	comp := kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: 1}),
		kont.ExprBind(kont.ExprPerform(kont.Modify[int]{F: func(x int) int { return x + 1 }}), func(_ int) kont.Expr[int] {
			return kont.ExprBind(kont.ExprPerform(kont.Modify[int]{F: func(x int) int { return x * 2 }}), func(_ int) kont.Expr[int] {
				return kont.ExprPerform(kont.Get[int]{})
			})
		}),
	)

	result, _ := kont.RunStateExpr[int, int](0, comp)
	if result != 4 { // (1 + 1) * 2 = 4
		t.Fatalf("got %d, want 4", result)
	}
}

func TestExprStatePure(t *testing.T) {
	// Pure value should not affect state
	comp := kont.ExprReturn[int](42)

	result, finalState := kont.RunStateExpr[int, int](100, comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if finalState != 100 {
		t.Fatalf("got state %d, want 100", finalState)
	}
}
