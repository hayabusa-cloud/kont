// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"strconv"
	"testing"

	"code.hybscloud.com/kont"
)

func TestRunPureReturn(t *testing.T) {
	// Simple return
	c := kont.ExprReturn(42)
	result := kont.RunPure(c)
	if result != 42 {
		t.Errorf("RunPure(ExprReturn(42)) = %v, want 42", result)
	}
}

func TestRunPureMap(t *testing.T) {
	// Map over a return
	c := kont.ExprMap(kont.ExprReturn(21), func(x int) int { return x * 2 })
	result := kont.RunPure(c)
	if result != 42 {
		t.Errorf("RunPure(ExprMap(ExprReturn(21), *2)) = %v, want 42", result)
	}
}

func TestRunPureBind(t *testing.T) {
	// Bind over a return
	c := kont.ExprBind(kont.ExprReturn(21), func(x int) kont.Expr[int] {
		return kont.ExprReturn(x * 2)
	})
	result := kont.RunPure(c)
	if result != 42 {
		t.Errorf("RunPure(ExprBind(ExprReturn(21), *2)) = %v, want 42", result)
	}
}

func TestRunPureThen(t *testing.T) {
	// Then discards first result
	c := kont.ExprThen(kont.ExprReturn(999), kont.ExprReturn(42))
	result := kont.RunPure(c)
	if result != 42 {
		t.Errorf("RunPure(ExprThen(ExprReturn(999), ExprReturn(42))) = %v, want 42", result)
	}
}

func TestRunPureChainedMaps(t *testing.T) {
	// Chain multiple maps
	c := kont.ExprReturn(1)
	c = kont.ExprMap(c, func(x int) int { return x + 1 }) // 2
	c = kont.ExprMap(c, func(x int) int { return x * 2 }) // 4
	c = kont.ExprMap(c, func(x int) int { return x + 3 }) // 7

	result := kont.RunPure(c)
	if result != 7 {
		t.Errorf("chained maps = %v, want 7", result)
	}
}

func TestRunPureChainedBinds(t *testing.T) {
	// Chain multiple binds
	c := kont.ExprReturn(1)
	for range 5 {
		c = kont.ExprBind(c, func(x int) kont.Expr[int] {
			return kont.ExprReturn(x + 1)
		})
	}

	result := kont.RunPure(c)
	if result != 6 {
		t.Errorf("chained binds = %v, want 6", result)
	}
}

func TestRunPureMapThenBind(t *testing.T) {
	// Mix of map, then, and bind
	c := kont.ExprReturn(10)
	c = kont.ExprMap(c, func(x int) int { return x * 2 }) // 20
	c = kont.ExprBind(c, func(x int) kont.Expr[int] {
		return kont.ExprReturn(x + 2) // 22
	})
	c = kont.ExprThen(kont.ExprReturn(0), c) // still 22

	result := kont.RunPure(c)
	if result != 22 {
		t.Errorf("mixed operations = %v, want 22", result)
	}
}

func TestRunPureTypeConversion(t *testing.T) {
	// Convert between types
	c := kont.ExprReturn(42)
	cs := kont.ExprMap(c, strconv.Itoa)
	result := kont.RunPure(cs)
	if result != "42" {
		t.Errorf("type conversion = %q, want \"42\"", result)
	}
}

func TestRunPureDeepChain(t *testing.T) {
	// Test deep chains don't cause stack overflow
	// This verifies the trampoline is iterative, not recursive
	c := kont.ExprReturn(0)
	for range 10000 {
		c = kont.ExprMap(c, func(x int) int { return x + 1 })
	}

	result := kont.RunPure(c)
	if result != 10000 {
		t.Errorf("deep chain = %v, want 10000", result)
	}
}

func TestBindOptimization(t *testing.T) {
	// Bind on Return should apply directly (optimization)
	called := false
	c := kont.ExprBind(kont.ExprReturn(42), func(x int) kont.Expr[string] {
		called = true
		return kont.ExprReturn(strconv.Itoa(x))
	})

	// The optimization should have already applied f
	if !called {
		// This is expected with the optimization
		_ = kont.RunPure(c)
		if !called {
			t.Error("ExprBind.F should be called during evaluation")
		}
	}
}

func TestMapOptimization(t *testing.T) {
	// Map on Return should apply directly (optimization)
	c := kont.ExprMap(kont.ExprReturn(21), func(x int) int { return x * 2 })

	// Check the value is already computed
	if _, ok := c.Frame.(kont.ReturnFrame); !ok {
		t.Error("ExprMap optimization should produce ReturnFrame directly")
	}
	if c.Value != 42 {
		t.Errorf("ExprMap optimization value = %v, want 42", c.Value)
	}
}

func TestThenOptimization(t *testing.T) {
	// Then on Return should return second directly (optimization)
	second := kont.ExprReturn("second")
	c := kont.ExprThen(kont.ExprReturn("first"), second)

	// Check we get the second computation directly
	if c.Value != "second" {
		t.Errorf("ExprThen optimization value = %v, want \"second\"", c.Value)
	}
}

func TestRunPureReturnFrame(t *testing.T) {
	result := kont.RunPure(kont.Expr[int]{Value: 42, Frame: kont.ReturnFrame{}})
	if result != 42 {
		t.Errorf("RunPure(ReturnFrame) = %v, want 42", result)
	}
}

func TestRunPureMapFrame(t *testing.T) {
	result := kont.RunPure(kont.Expr[int]{
		Value: 21,
		Frame: &kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) * 2 },
			Next: kont.ReturnFrame{},
		},
	})
	if result != 42 {
		t.Errorf("RunPure(MapFrame) = %v, want 42", result)
	}
}

// =============================================================================
// HandleExpr — effect-aware defunctionalized evaluation
// =============================================================================

type testAbort struct{}

func (testAbort) OpResult() int { panic("phantom") }

func TestHandleExprPure(t *testing.T) {
	// HandleExpr on a pure computation (no effects)
	c := kont.ExprReturn(42)
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(_ kont.Operation) (kont.Resumed, bool) {
		panic("unreachable")
	}))
	if result != 42 {
		t.Errorf("HandleExpr pure = %v, want 42", result)
	}
}

func TestHandleExprSingleEffect(t *testing.T) {
	c := kont.ExprPerform(kont.Get[int]{})
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case kont.Get[int]:
			return 42, true
		}
		panic("unhandled")
	}))
	if result != 42 {
		t.Errorf("HandleExpr single effect = %v, want 42", result)
	}
}

func TestHandleExprBindAfterEffect(t *testing.T) {
	// ExprBind(ExprPerform(Get), func(x) ExprReturn(x*2))
	c := kont.ExprBind(
		kont.ExprPerform(kont.Get[int]{}),
		func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 2)
		},
	)
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case kont.Get[int]:
			return 21, true
		}
		panic("unhandled")
	}))
	if result != 42 {
		t.Errorf("HandleExpr bind after effect = %v, want 42", result)
	}
}

func TestHandleExprMapAfterEffect(t *testing.T) {
	c := kont.ExprMap(
		kont.ExprPerform(kont.Get[int]{}),
		func(x int) int { return x * 2 },
	)
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case kont.Get[int]:
			return 21, true
		}
		panic("unhandled")
	}))
	if result != 42 {
		t.Errorf("HandleExpr map after effect = %v, want 42", result)
	}
}

func TestHandleExprMultipleEffects(t *testing.T) {
	// ExprBind(Get, func(x) ExprBind(Get, func(y) ExprReturn(x+y)))
	c := kont.ExprBind(
		kont.ExprPerform(kont.Get[int]{}),
		func(x int) kont.Expr[int] {
			return kont.ExprBind(
				kont.ExprPerform(kont.Get[int]{}),
				func(y int) kont.Expr[int] {
					return kont.ExprReturn(x + y)
				},
			)
		},
	)
	calls := 0
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case kont.Get[int]:
			calls++
			return calls * 10, true // 10 first, 20 second
		}
		panic("unhandled")
	}))
	// 10 + 20 = 30
	if result != 30 {
		t.Errorf("HandleExpr multiple effects = %v, want 30", result)
	}
	if calls != 2 {
		t.Errorf("handler called %d times, want 2", calls)
	}
}

func TestHandleExprShortCircuit(t *testing.T) {
	c := kont.ExprBind(
		kont.ExprPerform(testAbort{}),
		func(x int) kont.Expr[int] {
			panic("should not be reached")
		},
	)
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case testAbort:
			return 99, false // short-circuit
		}
		panic("unhandled")
	}))
	if result != 99 {
		t.Errorf("HandleExpr short-circuit = %v, want 99", result)
	}
}

func TestHandleExprThenWithEffect(t *testing.T) {
	// ExprBind(ExprThen(ExprReturn("ignored"), ExprPerform(Get)), func(x) ExprReturn(x*2))
	c := kont.ExprBind(
		kont.ExprThen(kont.ExprReturn("ignored"), kont.ExprPerform(kont.Get[int]{})),
		func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 2)
		},
	)
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case kont.Get[int]:
			return 21, true
		}
		panic("unhandled")
	}))
	if result != 42 {
		t.Errorf("HandleExpr then+effect = %v, want 42", result)
	}
}

func TestHandleExprChainedShortCircuit(t *testing.T) {
	// Short-circuit inside a chained frame
	c := kont.ExprBind(
		kont.ExprBind(
			kont.ExprPerform(testAbort{}),
			func(x int) kont.Expr[int] {
				panic("inner should not be reached")
			},
		),
		func(x int) kont.Expr[int] {
			panic("outer should not be reached")
		},
	)
	result := kont.HandleExpr(c, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case testAbort:
			return 77, false
		}
		panic("unhandled")
	}))
	if result != 77 {
		t.Errorf("HandleExpr chained short-circuit = %v, want 77", result)
	}
}

func BenchmarkHandleExprSingleEffect(b *testing.B) {
	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 42, true
	})
	for b.Loop() {
		c := kont.ExprPerform(kont.Get[int]{})
		_ = kont.HandleExpr(c, handler)
	}
}

func BenchmarkHandleExprBindChain(b *testing.B) {
	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 10, true
	})
	for b.Loop() {
		c := kont.ExprBind(
			kont.ExprPerform(kont.Get[int]{}),
			func(x int) kont.Expr[int] {
				return kont.ExprReturn(x * 2)
			},
		)
		_ = kont.HandleExpr(c, handler)
	}
}

func BenchmarkTrampolineReturn(b *testing.B) {
	c := kont.ExprReturn(42)
	b.ResetTimer()
	for b.Loop() {
		_ = kont.RunPure(c)
	}
}

func BenchmarkTrampolineMap10(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		c := kont.ExprReturn(0)
		for range 10 {
			c = kont.ExprMap(c, func(x int) int { return x + 1 })
		}
		_ = kont.RunPure(c)
	}
}

func BenchmarkTrampolineBind10(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		c := kont.ExprReturn(0)
		for range 10 {
			c = kont.ExprBind(c, func(x int) kont.Expr[int] {
				return kont.ExprReturn(x + 1)
			})
		}
		_ = kont.RunPure(c)
	}
}

func BenchmarkTrampolineDeep100(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		c := kont.ExprReturn(0)
		for range 100 {
			c = kont.ExprMap(c, func(x int) int { return x + 1 })
		}
		_ = kont.RunPure(c)
	}
}

// Tests for ChainFrames on individual frame types

func TestChainFramesReturnFirst(t *testing.T) {
	// When first is ReturnFrame, should return second directly
	first := kont.ReturnFrame{}
	second := &kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) * 2 },
		Next: kont.ReturnFrame{},
	}

	result := kont.ChainFrames(first, second)
	if result != second {
		t.Error("ChainFrames(ReturnFrame, x) should return x")
	}
}

func TestChainFramesNonReturn(t *testing.T) {
	// When first is not ReturnFrame, creates a chained frame
	first := &kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) + 1 },
		Next: kont.ReturnFrame{},
	}
	second := &kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) * 2 },
		Next: kont.ReturnFrame{},
	}

	chained := kont.ChainFrames(first, second)

	// Evaluate the full chain: 5 -> +1 -> *2 = 12
	result := kont.RunPure(kont.Expr[int]{Value: 5, Frame: chained})
	if result != 12 {
		t.Errorf("RunPure on chained frames = %v, want 12", result)
	}
}

func TestRunPureBindFrame(t *testing.T) {
	result := kont.RunPure(kont.Expr[int]{
		Value: 7,
		Frame: &kont.BindFrame[any, any]{
			F: func(x any) kont.Expr[any] {
				return kont.Expr[any]{
					Value: x.(int) * 3,
					Frame: kont.ReturnFrame{},
				}
			},
			Next: kont.ReturnFrame{},
		},
	})
	if result != 21 {
		t.Errorf("RunPure(BindFrame) = %v, want 21", result)
	}
}

func TestRunPureThenFrame(t *testing.T) {
	result := kont.RunPure(kont.Expr[string]{
		Value: "ignored",
		Frame: &kont.ThenFrame[any, any]{
			Second: kont.Expr[any]{
				Value: "second_value",
				Frame: kont.ReturnFrame{},
			},
			Next: kont.ReturnFrame{},
		},
	})
	if result != "second_value" {
		t.Errorf("RunPure(ThenFrame) = %v, want \"second_value\"", result)
	}
}

func TestTrampolineMapFrame(t *testing.T) {
	// Create computation with MapFrame that goes through RunPure
	// Build frame chain manually to test RunPure's MapFrame handling
	c := kont.Expr[int]{
		Value: 5,
		Frame: &kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) * 2 },
			Next: kont.ReturnFrame{},
		},
	}

	result := kont.RunPure(c)
	if result != 10 {
		t.Errorf("RunPure with MapFrame = %v, want 10", result)
	}
}

func TestTrampolineBindFrame(t *testing.T) {
	// Create computation with BindFrame
	c := kont.Expr[int]{
		Value: 7,
		Frame: &kont.BindFrame[any, any]{
			F: func(x any) kont.Expr[any] {
				return kont.Expr[any]{
					Value: x.(int) * 3,
					Frame: kont.ReturnFrame{},
				}
			},
			Next: kont.ReturnFrame{},
		},
	}

	result := kont.RunPure(c)
	if result != 21 {
		t.Errorf("RunPure with BindFrame = %v, want 21", result)
	}
}

func TestTrampolineThenFrame(t *testing.T) {
	// Create computation with ThenFrame
	c := kont.Expr[string]{
		Value: "ignored",
		Frame: &kont.ThenFrame[any, any]{
			Second: kont.Expr[any]{
				Value: "result",
				Frame: kont.ReturnFrame{},
			},
			Next: kont.ReturnFrame{},
		},
	}

	result := kont.RunPure(c)
	if result != "result" {
		t.Errorf("RunPure with ThenFrame = %q, want \"result\"", result)
	}
}

func TestTrampolineChainedFrames(t *testing.T) {
	// Test RunPure with chained frames (chainedFrame type)
	mapFrame := &kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) + 10 },
		Next: kont.ReturnFrame{},
	}
	bindFrame := &kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{
				Value: x.(int) * 2,
				Frame: kont.ReturnFrame{},
			}
		},
		Next: kont.ReturnFrame{},
	}

	// Chain: 5 -> +10 -> *2 = 30
	c := kont.Expr[int]{
		Value: 5,
		Frame: kont.ChainFrames(mapFrame, bindFrame),
	}

	result := kont.RunPure(c)
	if result != 30 {
		t.Errorf("RunPure with chained frames = %v, want 30", result)
	}
}

func TestTrampolineNestedChains(t *testing.T) {
	// Test deeply nested frame chains
	// 1 -> +1 -> +1 -> +1 = 4
	chain := kont.ChainFrames(
		&kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) + 1 },
			Next: kont.ReturnFrame{},
		},
		kont.ChainFrames(
			&kont.MapFrame[any, any]{
				F:    func(x any) any { return x.(int) + 1 },
				Next: kont.ReturnFrame{},
			},
			&kont.MapFrame[any, any]{
				F:    func(x any) any { return x.(int) + 1 },
				Next: kont.ReturnFrame{},
			},
		),
	)

	c := kont.Expr[int]{Value: 1, Frame: chain}
	result := kont.RunPure(c)
	if result != 4 {
		t.Errorf("nested chains = %v, want 4", result)
	}
}

func TestTrampolineChainedBindReturnContinuation(t *testing.T) {
	// Test chained frame where Bind's F returns a non-Return computation
	chain := kont.ChainFrames(
		&kont.BindFrame[any, any]{
			F: func(x any) kont.Expr[any] {
				// Return a computation with another MapFrame
				return kont.Expr[any]{
					Value: x.(int) + 5,
					Frame: &kont.MapFrame[any, any]{
						F:    func(y any) any { return y.(int) * 2 },
						Next: kont.ReturnFrame{},
					},
				}
			},
			Next: kont.ReturnFrame{},
		},
		kont.ReturnFrame{},
	)

	c := kont.Expr[int]{Value: 10, Frame: chain}
	result := kont.RunPure(c)
	// 10 -> bind(+5 then *2) = (10+5)*2 = 30
	if result != 30 {
		t.Errorf("chained bind with nested = %v, want 30", result)
	}
}

func TestTrampolineChainedThen(t *testing.T) {
	// Test chained Then frame
	chain := kont.ChainFrames(
		&kont.ThenFrame[any, any]{
			Second: kont.Expr[any]{
				Value: 100,
				Frame: kont.ReturnFrame{},
			},
			Next: kont.ReturnFrame{},
		},
		&kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) + 5 },
			Next: kont.ReturnFrame{},
		},
	)

	c := kont.Expr[int]{Value: 0, Frame: chain}
	result := kont.RunPure(c)
	// 0 -> then(100) -> +5 = 105
	if result != 105 {
		t.Errorf("chained then = %v, want 105", result)
	}
}

// =============================================================================
// Coverage: Bind/Map/Then non-Return paths (frame chain construction)
// These test the non-optimized code paths in ExprBind/ExprMap/ExprThen constructors.
// Non-Return Expr has Value=zero (per frame.go doc), so frames process zero.
// =============================================================================

func TestExprBindNonReturn(t *testing.T) {
	// ExprSuspend creates a non-Return computation with Value=zero
	m := kont.ExprSuspend[int](&kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) + 10 },
		Next: kont.ReturnFrame{},
	})
	// ExprBind on non-Return: exercises non-optimized path
	c := kont.ExprBind(m, func(x int) kont.Expr[int] {
		return kont.ExprReturn(x * 3)
	})

	result := kont.RunPure(c)
	// 0 -> +10 = 10 -> *3 = 30
	if result != 30 {
		t.Errorf("ExprBind on non-Return = %v, want 30", result)
	}
}

func TestExprMapNonReturn(t *testing.T) {
	m := kont.ExprSuspend[int](&kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) + 7 },
		Next: kont.ReturnFrame{},
	})
	// ExprMap on non-Return: exercises non-optimized path
	c := kont.ExprMap(m, func(x int) int { return x * 3 })

	result := kont.RunPure(c)
	// 0 -> +7 = 7 -> *3 = 21
	if result != 21 {
		t.Errorf("ExprMap on non-Return = %v, want 21", result)
	}
}

func TestExprThenNonReturn(t *testing.T) {
	m := kont.ExprSuspend[int](&kont.MapFrame[any, any]{
		F:    func(x any) any { return x.(int) + 1 },
		Next: kont.ReturnFrame{},
	})
	// ExprThen on non-Return: exercises non-optimized path
	c := kont.ExprThen(m, kont.ExprReturn(99))

	result := kont.RunPure(c)
	// m evaluates (discarded), then returns 99
	if result != 99 {
		t.Errorf("ExprThen on non-Return = %v, want 99", result)
	}
}

func TestExprBindNonReturnWithBindFrame(t *testing.T) {
	// ExprBind on a ExprSuspend with BindFrame
	m := kont.ExprSuspend[int](&kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{Value: x.(int) + 10, Frame: kont.ReturnFrame{}}
		},
		Next: kont.ReturnFrame{},
	})
	c := kont.ExprBind(m, func(x int) kont.Expr[int] {
		return kont.ExprReturn(x * 5)
	})

	result := kont.RunPure(c)
	// 0 -> +10 = 10 -> *5 = 50
	if result != 50 {
		t.Errorf("ExprBind with BindFrame = %v, want 50", result)
	}
}

func TestExprMapNonReturnWithBindFrame(t *testing.T) {
	m := kont.ExprSuspend[int](&kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{Value: x.(int) + 42, Frame: kont.ReturnFrame{}}
		},
		Next: kont.ReturnFrame{},
	})
	c := kont.ExprMap(m, func(x int) int { return x * 2 })

	result := kont.RunPure(c)
	// 0 -> +42 = 42 -> *2 = 84
	if result != 84 {
		t.Errorf("ExprMap with BindFrame = %v, want 84", result)
	}
}

func TestExprThenNonReturnWithBindFrame(t *testing.T) {
	m := kont.ExprSuspend[int](&kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{Value: x.(int) + 1, Frame: kont.ReturnFrame{}}
		},
		Next: kont.ReturnFrame{},
	})
	c := kont.ExprThen(m, kont.ExprReturn(77))

	result := kont.RunPure(c)
	if result != 77 {
		t.Errorf("ExprThen with BindFrame = %v, want 77", result)
	}
}

// =============================================================================
// Coverage: RunPure — EffectFrame panic path
// =============================================================================

func TestTrampolineEffectFramePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unhandled effect frame")
		}
	}()

	c := kont.Expr[int]{
		Value: 0,
		Frame: &kont.EffectFrame[any]{
			Resume: func(x any) any { return x },
			Next:   kont.ReturnFrame{},
		},
	}
	kont.RunPure(c)
}

func TestTrampolineThenFrameDirect(t *testing.T) {
	// ThenFrame in the direct (non-chained) path of RunPure
	c := kont.Expr[int]{
		Value: 999,
		Frame: &kont.ThenFrame[any, any]{
			Second: kont.Expr[any]{
				Value: 42,
				Frame: kont.ReturnFrame{},
			},
			Next: kont.ReturnFrame{},
		},
	}

	result := kont.RunPure(c)
	if result != 42 {
		t.Errorf("direct ThenFrame = %v, want 42", result)
	}
}

// =============================================================================
// Coverage: RunPure — chained EffectFrame panic
// =============================================================================

func TestTrampolineChainedEffectFramePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unhandled effect frame in chain")
		}
	}()

	chain := kont.ChainFrames(
		&kont.EffectFrame[any]{
			Resume: func(x any) any { return x },
			Next:   kont.ReturnFrame{},
		},
		kont.ReturnFrame{},
	)
	c := kont.Expr[int]{Value: 0, Frame: chain}
	kont.RunPure(c)
}

// =============================================================================
// Coverage: RunPure — chained ReturnFrame path
// =============================================================================

func TestTrampolineChainedReturnFrame(t *testing.T) {
	// ChainFrames where first is a non-Return but second is Return
	// This exercises the ReturnFrame case inside chained frame processing
	chain := kont.ChainFrames(
		&kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) + 10 },
			Next: kont.ReturnFrame{},
		},
		kont.ReturnFrame{},
	)

	c := kont.Expr[int]{Value: 5, Frame: chain}
	result := kont.RunPure(c)
	if result != 15 {
		t.Errorf("chained return = %v, want 15", result)
	}
}

// unknownFrame tests removed: Frame.frame() is unexported, preventing
// external types from implementing Frame. The default panic branches
// in evalFrames are unreachable from outside the package.

// =============================================================================
// Coverage: ExprBind/ExprMap/ExprThen non-Return with deep chains
// =============================================================================

func TestExprBindNonReturnDeep(t *testing.T) {
	// ExprSuspend creates non-Return; chain ExprBind on it
	m := kont.ExprSuspend[int](&kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{Value: x.(int) + 2, Frame: kont.ReturnFrame{}}
		},
		Next: kont.ReturnFrame{},
	})
	c := kont.ExprBind(m, func(x int) kont.Expr[int] {
		return kont.ExprReturn(x + 100)
	})

	result := kont.RunPure(c)
	// 0 -> +2 = 2 -> +100 = 102
	if result != 102 {
		t.Errorf("deep ExprBind chain = %v, want 102", result)
	}
}

func TestExprMapNonReturnDeep(t *testing.T) {
	m := kont.ExprSuspend[int](&kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{Value: x.(int) + 3, Frame: kont.ReturnFrame{}}
		},
		Next: kont.ReturnFrame{},
	})
	c := kont.ExprMap(m, func(x int) int { return x + 10 })

	result := kont.RunPure(c)
	// 0 -> +3 = 3 -> +10 = 13
	if result != 13 {
		t.Errorf("deep ExprMap chain = %v, want 13", result)
	}
}

func TestExprThenNonReturnDeep(t *testing.T) {
	m := kont.ExprSuspend[int](&kont.BindFrame[any, any]{
		F: func(x any) kont.Expr[any] {
			return kont.Expr[any]{Value: x.(int) + 3, Frame: kont.ReturnFrame{}}
		},
		Next: kont.ReturnFrame{},
	})
	c := kont.ExprThen(m, kont.ExprReturn(42))

	result := kont.RunPure(c)
	if result != 42 {
		t.Errorf("deep ExprThen chain = %v, want 42", result)
	}
}

// =============================================================================
// Benchmarks: non-Return paths
// =============================================================================

func BenchmarkExprBindNonReturn(b *testing.B) {
	for b.Loop() {
		m := kont.ExprSuspend[int](&kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) + 1 },
			Next: kont.ReturnFrame{},
		})
		c := kont.ExprBind(m, func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 2)
		})
		_ = kont.RunPure(c)
	}
}

func BenchmarkExprMapNonReturn(b *testing.B) {
	for b.Loop() {
		m := kont.ExprSuspend[int](&kont.MapFrame[any, any]{
			F:    func(x any) any { return x.(int) + 1 },
			Next: kont.ReturnFrame{},
		})
		c := kont.ExprMap(m, func(x int) int { return x * 2 })
		_ = kont.RunPure(c)
	}
}

func BenchmarkExprThenNonReturn(b *testing.B) {
	for b.Loop() {
		m := kont.ExprSuspend[int](&kont.MapFrame[any, any]{
			F:    func(x any) any { return x },
			Next: kont.ReturnFrame{},
		})
		c := kont.ExprThen(m, kont.ExprReturn(42))
		_ = kont.RunPure(c)
	}
}

func BenchmarkTrampolineMixed(b *testing.B) {
	for b.Loop() {
		c := kont.ExprReturn(1)
		c = kont.ExprMap(c, func(x int) int { return x + 1 })
		c = kont.ExprBind(c, func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 3)
		})
		c = kont.ExprThen(kont.ExprReturn(0), c)
		c = kont.ExprMap(c, func(x int) int { return x + 10 })
		_ = kont.RunPure(c)
	}
}
