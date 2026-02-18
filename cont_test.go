// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestReturnRun(t *testing.T) {
	got := kont.Run(kont.Return[int](42))
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestReturnRunString(t *testing.T) {
	got := kont.Run(kont.Return[string]("hello"))
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestRunWith(t *testing.T) {
	m := kont.Return[string, int](42)
	got := kont.RunWith(m, func(x int) string {
		return "value"
	})
	if got != "value" {
		t.Fatalf("got %q, want %q", got, "value")
	}
}

func TestBindSimple(t *testing.T) {
	m := kont.Return[int](10)
	n := kont.Bind(m, func(x int) kont.Cont[int, int] {
		return kont.Return[int](x * 2)
	})
	got := kont.Run(n)
	if got != 20 {
		t.Fatalf("got %d, want 20", got)
	}
}

func TestBindChain(t *testing.T) {
	m := kont.Return[int](5)
	n := kont.Bind(m, func(x int) kont.Cont[int, int] {
		return kont.Bind(kont.Return[int](x+1), func(y int) kont.Cont[int, int] {
			return kont.Return[int](y * 2)
		})
	})
	got := kont.Run(n)
	if got != 12 {
		t.Fatalf("got %d, want 12", got)
	}
}

func TestBindLeftIdentity(t *testing.T) {
	// Bind(Return(a), f) ≡ f(a)
	a := 7
	f := func(x int) kont.Cont[int, int] {
		return kont.Return[int](x * 3)
	}

	left := kont.Run(kont.Bind(kont.Return[int](a), f))
	right := kont.Run(f(a))

	if left != right {
		t.Fatalf("left identity failed: %d != %d", left, right)
	}
}

func TestBindRightIdentity(t *testing.T) {
	// Bind(m, Return) ≡ m
	m := kont.Return[int](42)

	left := kont.Run(kont.Bind(m, func(x int) kont.Cont[int, int] {
		return kont.Return[int](x)
	}))
	right := kont.Run(m)

	if left != right {
		t.Fatalf("right identity failed: %d != %d", left, right)
	}
}

func TestBindAssociativity(t *testing.T) {
	// Bind(Bind(m, f), g) ≡ Bind(m, func(x) Bind(f(x), g))
	m := kont.Return[int](2)
	f := func(x int) kont.Cont[int, int] {
		return kont.Return[int](x + 3)
	}
	g := func(x int) kont.Cont[int, int] {
		return kont.Return[int](x * 2)
	}

	left := kont.Run(kont.Bind(kont.Bind(m, f), g))
	right := kont.Run(kont.Bind(m, func(x int) kont.Cont[int, int] {
		return kont.Bind(f(x), g)
	}))

	if left != right {
		t.Fatalf("associativity failed: %d != %d", left, right)
	}
}

func TestMap(t *testing.T) {
	m := kont.Return[int](10)
	n := kont.Map(m, func(x int) int {
		return x * 3
	})
	got := kont.Run(n)
	if got != 30 {
		t.Fatalf("got %d, want 30", got)
	}
}

func TestSuspend(t *testing.T) {
	m := kont.Suspend[int, int](func(k func(int) int) int {
		return k(42) + 1
	})
	got := kont.Run(m)
	if got != 43 {
		t.Fatalf("got %d, want 43", got)
	}
}

func TestPure(t *testing.T) {
	got := kont.Handle(kont.Pure(42), kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("should not be called")
	}))
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestPureString(t *testing.T) {
	got := kont.Handle(kont.Pure("hello"), kont.HandleFunc[string](func(op kont.Operation) (kont.Resumed, bool) {
		panic("should not be called")
	}))
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestEffBindPure(t *testing.T) {
	// Eff[int] used as Cont[Resumed, int] in Bind
	comp := kont.Bind(
		kont.Pure(10),
		func(x int) kont.Eff[int] {
			return kont.Pure(x * 2)
		},
	)

	got := kont.Handle(comp, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("should not be called")
	}))
	if got != 20 {
		t.Fatalf("got %d, want 20", got)
	}
}

func TestBindLeftIdentityWithStrings(t *testing.T) {
	a := "hello"
	f := func(s string) kont.Cont[string, string] {
		return kont.Return[string](s + " world")
	}

	left := kont.Run(kont.Bind(kont.Return[string](a), f))
	right := kont.Run(f(a))

	if left != right {
		t.Fatalf("Bind left identity (string) failed: %q != %q", left, right)
	}
}

func TestBindAssociativityWithTypeChange(t *testing.T) {
	m := kont.Return[string](42)
	f := func(x int) kont.Cont[string, string] {
		return kont.Return[string]("value")
	}
	g := func(s string) kont.Cont[string, string] {
		return kont.Return[string](s + "!")
	}

	left := kont.Run(kont.Bind(kont.Bind(m, f), g))
	right := kont.Run(kont.Bind(m, func(x int) kont.Cont[string, string] {
		return kont.Bind(f(x), g)
	}))

	if left != right {
		t.Fatalf("Bind associativity (type change) failed: %q != %q", left, right)
	}
}
