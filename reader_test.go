// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

type Config struct {
	Debug bool
	Port  int
}

func TestReaderAsk(t *testing.T) {
	comp := kont.AskReader(func(x int) kont.Eff[int] {
		return kont.Pure(x)
	})

	result := kont.RunReader[int, int](42, comp)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestMapReader(t *testing.T) {
	comp := kont.MapReader[Config, int](func(c Config) int {
		return c.Port
	})

	result := kont.RunReader[Config, int](Config{Debug: true, Port: 8080}, comp)
	if result != 8080 {
		t.Fatalf("got %d, want 8080", result)
	}
}

func TestReaderChained(t *testing.T) {
	// Ask twice and combine
	comp := kont.AskReader(func(x int) kont.Eff[int] {
		return kont.AskReader(func(y int) kont.Eff[int] {
			return kont.Pure(x + y)
		})
	})

	result := kont.RunReader[int, int](21, comp)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestReaderWithConfig(t *testing.T) {
	comp := kont.Bind(
		kont.MapReader[Config, bool](func(c Config) bool { return c.Debug }),
		func(debug bool) kont.Eff[string] {
			if debug {
				return kont.Pure("debug mode")
			}
			return kont.Pure("production")
		},
	)

	result := kont.RunReader[Config, string](Config{Debug: true, Port: 80}, comp)
	if result != "debug mode" {
		t.Fatalf("got %q, want %q", result, "debug mode")
	}

	result = kont.RunReader[Config, string](Config{Debug: false, Port: 80}, comp)
	if result != "production" {
		t.Fatalf("got %q, want %q", result, "production")
	}
}

func TestReaderPure(t *testing.T) {
	// Pure should ignore the environment
	comp := kont.Pure(100)

	result := kont.RunReader[int, int](42, comp)
	if result != 100 {
		t.Fatalf("got %d, want 100", result)
	}
}

func TestReaderBind(t *testing.T) {
	// Bind should thread the environment through
	comp := kont.AskReader(func(env int) kont.Eff[int] {
		return kont.Pure(env * 2)
	})

	result := kont.RunReader[int, int](21, comp)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestExprReaderAsk(t *testing.T) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(x int) kont.Expr[int] {
		return kont.ExprReturn(x)
	})

	result := kont.RunReaderExpr[int, int](42, comp)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestExprMapReader(t *testing.T) {
	comp := kont.ExprMap(kont.ExprPerform(kont.Ask[Config]{}), func(c Config) int {
		return c.Port
	})

	result := kont.RunReaderExpr[Config, int](Config{Debug: true, Port: 8080}, comp)
	if result != 8080 {
		t.Fatalf("got %d, want 8080", result)
	}
}

func TestExprReaderChained(t *testing.T) {
	// Ask twice and combine
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(x int) kont.Expr[int] {
		return kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(y int) kont.Expr[int] {
			return kont.ExprReturn(x + y)
		})
	})

	result := kont.RunReaderExpr[int, int](21, comp)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestExprReaderPure(t *testing.T) {
	// Pure should ignore the environment
	comp := kont.ExprReturn[int](100)

	result := kont.RunReaderExpr[int, int](42, comp)
	if result != 100 {
		t.Fatalf("got %d, want 100", result)
	}
}

func TestExprReaderWithConfig(t *testing.T) {
	comp := kont.ExprBind(
		kont.ExprMap(kont.ExprPerform(kont.Ask[Config]{}), func(c Config) bool { return c.Debug }),
		func(debug bool) kont.Expr[string] {
			if debug {
				return kont.ExprReturn("debug mode")
			}
			return kont.ExprReturn("production")
		},
	)

	result := kont.RunReaderExpr[Config, string](Config{Debug: true, Port: 80}, comp)
	if result != "debug mode" {
		t.Fatalf("got %q, want %q", result, "debug mode")
	}

	result = kont.RunReaderExpr[Config, string](Config{Debug: false, Port: 80}, comp)
	if result != "production" {
		t.Fatalf("got %q, want %q", result, "production")
	}
}
