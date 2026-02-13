// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// Shift/Reset tests

func TestShiftIgnoreContinuation(t *testing.T) {
	// Shift that discards the continuation entirely
	m := kont.Shift[int, int](func(k func(int) int) int {
		// Never call k, just return directly
		return 100
	})
	got := kont.Run(m)
	if got != 100 {
		t.Fatalf("got %d, want 100", got)
	}
}

func TestShiftMultipleApplications(t *testing.T) {
	// Apply continuation three times
	m := kont.Bind(
		kont.Shift[int, int](func(k func(int) int) int {
			return k(1) + k(2) + k(3)
		}),
		func(x int) kont.Cont[int, int] {
			return kont.Return[int](x * 10)
		},
	)
	got := kont.Run(m)
	// k(1) = 10, k(2) = 20, k(3) = 30 => 60
	if got != 60 {
		t.Fatalf("got %d, want 60", got)
	}
}

func TestResetNestedShift(t *testing.T) {
	// Nested shift operations with reset
	inner := kont.Bind(
		kont.Shift[int, int](func(k func(int) int) int {
			return k(5) * 2
		}),
		func(x int) kont.Cont[int, int] {
			return kont.Return[int](x + 1)
		},
	)
	outer := kont.Bind(
		kont.Reset[int](inner),
		func(x int) kont.Cont[int, int] {
			return kont.Return[int](x + 100)
		},
	)
	got := kont.Run(outer)
	// inner: k(5) = 5+1 = 6, 6*2 = 12
	// outer: 12 + 100 = 112
	if got != 112 {
		t.Fatalf("got %d, want 112", got)
	}
}

func TestResetIsolatesShift(t *testing.T) {
	// Reset should isolate inner shift from outer continuation
	m := kont.Bind(
		kont.Reset[int](kont.Bind(
			kont.Shift[int, int](func(k func(int) int) int {
				return 42 // Discards inner continuation
			}),
			func(x int) kont.Cont[int, int] {
				return kont.Return[int](x * 1000) // Should not run
			},
		)),
		func(x int) kont.Cont[int, int] {
			return kont.Return[int](x + 1) // Should run with 42
		},
	)
	got := kont.Run(m)
	if got != 43 {
		t.Fatalf("got %d, want 43", got)
	}
}

func TestResetChained(t *testing.T) {
	// Multiple resets in sequence
	m1 := kont.Reset[int](kont.Bind(
		kont.Shift[int, int](func(k func(int) int) int {
			return k(10)
		}),
		func(x int) kont.Cont[int, int] {
			return kont.Return[int](x + 1)
		},
	))
	m2 := kont.Reset[int](kont.Bind(
		kont.Shift[int, int](func(k func(int) int) int {
			return k(20)
		}),
		func(x int) kont.Cont[int, int] {
			return kont.Return[int](x + 2)
		},
	))
	combined := kont.Bind(m1, func(a int) kont.Cont[int, int] {
		return kont.Bind(m2, func(b int) kont.Cont[int, int] {
			return kont.Return[int](a + b)
		})
	})
	got := kont.Run(combined)
	// m1: 10+1 = 11, m2: 20+2 = 22, combined: 11+22 = 33
	if got != 33 {
		t.Fatalf("got %d, want 33", got)
	}
}

func TestShiftWithMapChain(t *testing.T) {
	// Shift followed by Map operations
	m := kont.Bind(
		kont.Shift[int, int](func(k func(int) int) int {
			return k(7)
		}),
		func(x int) kont.Cont[int, int] {
			return kont.Map(kont.Return[int](x), func(y int) int {
				return y * 3
			})
		},
	)
	got := kont.Run(m)
	if got != 21 {
		t.Fatalf("got %d, want 21", got)
	}
}

func TestResetWithIdentity(t *testing.T) {
	// Reset around Return should be identity
	m := kont.Reset[int](kont.Return[int](42))
	got := kont.Run(m)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestShiftZeroApplications(t *testing.T) {
	// Shift that never uses the continuation at all
	sideEffect := 0
	m := kont.Bind(
		kont.Shift[int, int](func(k func(int) int) int {
			// Continuation is available but never used
			_ = k
			return 999
		}),
		func(x int) kont.Cont[int, int] {
			sideEffect = x // Should not execute
			return kont.Return[int](x * 2)
		},
	)
	got := kont.Run(m)
	if got != 999 {
		t.Fatalf("got %d, want 999", got)
	}
	if sideEffect != 0 {
		t.Fatal("continuation body executed when it should not have")
	}
}

func TestShiftStringType(t *testing.T) {
	// Shift with string type
	m := kont.Bind(
		kont.Shift[string, string](func(k func(string) string) string {
			return k("hello") + " " + k("world")
		}),
		func(s string) kont.Cont[string, string] {
			return kont.Return[string]("[" + s + "]")
		},
	)
	got := kont.Run(m)
	if got != "[hello] [world]" {
		t.Fatalf("got %q, want %q", got, "[hello] [world]")
	}
}
