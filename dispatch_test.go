// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestDispatchHandlerState(t *testing.T) {
	// Test that StateHandler uses dispatch interface (O(1) lookup)
	comp := kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
	})

	result, finalState := kont.RunState[int, int](5, comp)
	if result != 15 {
		t.Fatalf("got result %d, want 15", result)
	}
	if finalState != 15 {
		t.Fatalf("got state %d, want 15", finalState)
	}
}

func TestDispatchHandlerReader(t *testing.T) {
	// Test that ReaderHandler uses dispatch interface
	comp := kont.AskReader(func(s string) kont.Cont[kont.Resumed, string] {
		return kont.Return[kont.Resumed](s)
	})

	result := kont.RunReader("environment", comp)
	if result != "environment" {
		t.Fatalf("got %q, want %q", result, "environment")
	}
}

// CustomOp is an effect operation not handled by StateHandler
type CustomOp struct{ Value int }

func (CustomOp) OpResult() int { panic("phantom") }

func TestDispatchUnhandledPanics(t *testing.T) {
	// Test that unhandled effects in dispatch handler cause panic

	// Create a computation that performs a custom effect
	comp := kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
		// Perform an effect that StateHandler doesn't know how to handle
		return kont.Perform(CustomOp{Value: s})
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unhandled effect")
		}
	}()

	kont.RunState[int, int](0, comp)
}

func TestDispatchStateSequence(t *testing.T) {
	// Test multiple dispatch calls in sequence
	comp := kont.PutState(1,
		kont.ModifyState(func(x int) int { return x + 1 }, func(_ int) kont.Cont[kont.Resumed, int] {
			return kont.ModifyState(func(x int) int { return x * 3 }, func(_ int) kont.Cont[kont.Resumed, int] {
				return kont.GetState(func(s int) kont.Cont[kont.Resumed, int] {
					return kont.ModifyState(func(x int) int { return x + 10 }, func(_ int) kont.Cont[kont.Resumed, int] {
						return kont.Perform(kont.Get[int]{})
					})
				})
			})
		}),
	)

	result, finalState := kont.RunState[int, int](0, comp)
	// (1 + 1) * 3 = 6, then + 10 = 16
	if result != 16 {
		t.Fatalf("got result %d, want 16", result)
	}
	if finalState != 16 {
		t.Fatalf("got state %d, want 16", finalState)
	}
}

func TestDispatchReaderChained(t *testing.T) {
	// Test multiple reader accesses
	type Config struct {
		Host string
		Port int
	}

	comp := kont.AskReader(func(cfg1 Config) kont.Cont[kont.Resumed, string] {
		return kont.Bind(
			kont.MapReader[Config, int](func(c Config) int { return c.Port }),
			func(port int) kont.Cont[kont.Resumed, string] {
				return kont.AskReader(func(cfg2 Config) kont.Cont[kont.Resumed, string] {
					if cfg1.Host != cfg2.Host {
						return kont.Return[kont.Resumed]("mismatch")
					}
					return kont.Return[kont.Resumed](cfg1.Host)
				})
			},
		)
	})

	cfg := Config{Host: "localhost", Port: 8080}
	result := kont.RunReader(cfg, comp)
	if result != "localhost" {
		t.Fatalf("got %q, want %q", result, "localhost")
	}
}
