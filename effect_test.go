// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// askOp is a test effect operation that requests a value.
// Named askOp to avoid shadowing kont.Ask in the kont_test package scope.
type askOp struct{}

func (askOp) OpResult() int { panic("phantom") }

// tellOp is a test effect operation that outputs a value.
// Named tellOp to avoid shadowing kont.Tell in the kont_test package scope.
type tellOp struct{ Value int }

func (tellOp) OpResult() struct{} { panic("phantom") }

// getOp is a test effect operation for reading state.
// Named getOp to avoid shadowing kont.Get in the kont_test package scope.
type getOp struct{}

func (getOp) OpResult() int { panic("phantom") }

// putOp is a test effect operation for writing state.
// Named putOp to avoid shadowing kont.Put in the kont_test package scope.
type putOp struct{ Value int }

func (putOp) OpResult() struct{} { panic("phantom") }

func TestPerformHandle(t *testing.T) {
	// Computation that asks for a value and doubles it
	comp := kont.Bind(
		kont.Perform(askOp{}),
		func(x int) kont.Cont[kont.Resumed, int] {
			return kont.Return[kont.Resumed](x * 2)
		},
	)

	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case askOp:
			return 21, true // resume with 21
		default:
			panic("unhandled effect")
		}
	})

	got := kont.Handle(comp, handler)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestPerformHandleMultiple(t *testing.T) {
	// Computation with multiple effects
	comp := kont.Bind(
		kont.Perform(askOp{}),
		func(x int) kont.Cont[kont.Resumed, int] {
			return kont.Bind(
				kont.Perform(askOp{}),
				func(y int) kont.Cont[kont.Resumed, int] {
					return kont.Return[kont.Resumed](x + y)
				},
			)
		},
	)

	callCount := 0
	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch op.(type) {
		case askOp:
			callCount++
			return callCount * 10, true // 10, then 20
		default:
			panic("unhandled effect")
		}
	})

	got := kont.Handle(comp, handler)
	if got != 30 {
		t.Fatalf("got %d, want 30 (10 + 20)", got)
	}
	if callCount != 2 {
		t.Fatalf("handler called %d times, want 2", callCount)
	}
}

func TestHandleNoEffect(t *testing.T) {
	// Computation with no effects
	comp := kont.Return[kont.Resumed, int](42)

	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("should not be called")
	})

	got := kont.Handle(comp, handler)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestStateEffect(t *testing.T) {
	// State monad via effects
	// Bind(getOp, func(s) Then(putOp(s+1), getOp))
	comp := kont.Bind(
		kont.Perform(getOp{}),
		func(s int) kont.Cont[kont.Resumed, int] {
			return kont.Bind(
				kont.Perform(putOp{Value: s + 1}),
				func(_ struct{}) kont.Cont[kont.Resumed, int] {
					return kont.Perform(getOp{})
				},
			)
		},
	)

	// State handler
	state := 10
	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch e := op.(type) {
		case getOp:
			_ = e
			return state, true
		case putOp:
			state = e.Value
			return struct{}{}, true
		default:
			panic("unhandled effect")
		}
	})

	got := kont.Handle(comp, handler)
	if got != 11 {
		t.Fatalf("got %d, want 11", got)
	}
	if state != 11 {
		t.Fatalf("state is %d, want 11", state)
	}
}

func TestHandleFuncType(t *testing.T) {
	// Verify HandleFunc returns a concrete handler type
	h := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 0, true
	})
	// Verify it can be used with Handle
	comp := kont.Return[kont.Resumed, int](42)
	got := kont.Handle(comp, h)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestMixedEffects(t *testing.T) {
	// Computation mixing askOp and tellOp effects
	comp := kont.Bind(
		kont.Perform(askOp{}),
		func(x int) kont.Cont[kont.Resumed, int] {
			return kont.Bind(
				kont.Perform(tellOp{Value: x}),
				func(_ struct{}) kont.Cont[kont.Resumed, int] {
					return kont.Return[kont.Resumed](x * 2)
				},
			)
		},
	)

	told := 0
	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		switch e := op.(type) {
		case askOp:
			_ = e
			return 5, true
		case tellOp:
			told = e.Value
			return struct{}{}, true
		default:
			panic("unhandled effect")
		}
	})

	got := kont.Handle(comp, handler)
	if got != 10 {
		t.Fatalf("got %d, want 10", got)
	}
	if told != 5 {
		t.Fatalf("told %d, want 5", told)
	}
}

func TestBindEffectChain(t *testing.T) {
	// Test a longer chain of Bind
	comp := kont.Bind(
		kont.Return[kont.Resumed, int](1),
		func(a int) kont.Cont[kont.Resumed, int] {
			return kont.Bind(
				kont.Return[kont.Resumed, int](a+1),
				func(b int) kont.Cont[kont.Resumed, int] {
					return kont.Bind(
						kont.Return[kont.Resumed, int](b+1),
						func(c int) kont.Cont[kont.Resumed, int] {
							return kont.Return[kont.Resumed](c + 1)
						},
					)
				},
			)
		},
	)

	handler := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("should not be called")
	})

	got := kont.Handle(comp, handler)
	if got != 4 {
		t.Fatalf("got %d, want 4", got)
	}
}
