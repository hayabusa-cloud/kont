// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestErrorThrow(t *testing.T) {
	comp := kont.ThrowError[string, int]("something went wrong")

	result := kont.RunError[string, int](comp)
	if result.IsRight() {
		t.Fatal("expected Left, got Right")
	}
	err, _ := result.GetLeft()
	if err != "something went wrong" {
		t.Fatalf("got error %q, want %q", err, "something went wrong")
	}
}

func TestErrorNoThrow(t *testing.T) {
	comp := kont.Return[kont.Resumed, int](42)

	result := kont.RunError[string, int](comp)
	if result.IsLeft() {
		t.Fatal("expected Right, got Left")
	}
	val, _ := result.GetRight()
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
}

func TestErrorCatch(t *testing.T) {
	// Computation that throws, but is caught
	comp := kont.CatchError(
		kont.ThrowError[string, int]("error"),
		func(e string) kont.Cont[kont.Resumed, int] {
			return kont.Return[kont.Resumed](99) // recover with default value
		},
	)

	result := kont.RunError[string, int](comp)
	if result.IsLeft() {
		t.Fatal("expected Right after catch, got Left")
	}
	val, _ := result.GetRight()
	if val != 99 {
		t.Fatalf("got %d, want 99", val)
	}
}

func TestErrorCatchNoError(t *testing.T) {
	// Computation that succeeds, handler not called
	comp := kont.CatchError(
		kont.Return[kont.Resumed, int](42),
		func(e string) kont.Cont[kont.Resumed, int] {
			return kont.Return[kont.Resumed](0) // should not be called
		},
	)

	result := kont.RunError[string, int](comp)
	if result.IsLeft() {
		t.Fatal("expected Right, got Left")
	}
	val, _ := result.GetRight()
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
}

func TestErrorChained(t *testing.T) {
	// Error in middle of chain aborts rest
	comp := kont.Bind(
		kont.Return[kont.Resumed, int](1),
		func(x int) kont.Cont[kont.Resumed, int] {
			return kont.Bind(
				kont.ThrowError[string, int]("abort"),
				func(y int) kont.Cont[kont.Resumed, int] {
					return kont.Return[kont.Resumed](x + y) // never reached
				},
			)
		},
	)

	result := kont.RunError[string, int](comp)
	if result.IsRight() {
		t.Fatal("expected Left, got Right")
	}
	err, _ := result.GetLeft()
	if err != "abort" {
		t.Fatalf("got error %q, want %q", err, "abort")
	}
}

func TestExprErrorThrow(t *testing.T) {
	comp := kont.ExprThrowError[string, int]("something went wrong")

	result := kont.RunErrorExpr[string, int](comp)
	if result.IsRight() {
		t.Fatal("expected Left, got Right")
	}
	err, _ := result.GetLeft()
	if err != "something went wrong" {
		t.Fatalf("got error %q, want %q", err, "something went wrong")
	}
}

func TestExprErrorNoThrow(t *testing.T) {
	comp := kont.ExprReturn[int](42)

	result := kont.RunErrorExpr[string, int](comp)
	if result.IsLeft() {
		t.Fatal("expected Right, got Left")
	}
	val, _ := result.GetRight()
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
}

func TestExprErrorChained(t *testing.T) {
	// Error in middle of chain aborts rest
	comp := kont.ExprBind(
		kont.ExprReturn[int](1),
		func(x int) kont.Expr[int] {
			return kont.ExprBind(
				kont.ExprThrowError[string, int]("abort"),
				func(y int) kont.Expr[int] {
					return kont.ExprReturn(x + y) // never reached
				},
			)
		},
	)

	result := kont.RunErrorExpr[string, int](comp)
	if result.IsRight() {
		t.Fatal("expected Left, got Right")
	}
	err, _ := result.GetLeft()
	if err != "abort" {
		t.Fatalf("got error %q, want %q", err, "abort")
	}
}

func TestEitherLeft(t *testing.T) {
	e := kont.Left[string, int]("error")

	if !e.IsLeft() {
		t.Fatal("expected IsLeft true")
	}
	if e.IsRight() {
		t.Fatal("expected IsRight false")
	}
	err, ok := e.GetLeft()
	if !ok {
		t.Fatal("GetLeft should return true")
	}
	if err != "error" {
		t.Fatalf("got %q, want %q", err, "error")
	}
}

func TestEitherRight(t *testing.T) {
	e := kont.Right[string, int](42)

	if e.IsLeft() {
		t.Fatal("expected IsLeft false")
	}
	if !e.IsRight() {
		t.Fatal("expected IsRight true")
	}
	val, ok := e.GetRight()
	if !ok {
		t.Fatal("GetRight should return true")
	}
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
}

func TestMapEither(t *testing.T) {
	right := kont.Right[string, int](21)
	mapped := kont.MapEither(right, func(x int) int { return x * 2 })

	val, ok := mapped.GetRight()
	if !ok || val != 42 {
		t.Fatalf("got %d, want 42", val)
	}

	left := kont.Left[string, int]("error")
	mappedLeft := kont.MapEither(left, func(x int) int { return x * 2 })

	if mappedLeft.IsRight() {
		t.Fatal("mapping Left should remain Left")
	}
}

func TestFlatMapEither(t *testing.T) {
	right := kont.Right[string, int](21)
	result := kont.FlatMapEither(right, func(x int) kont.Either[string, int] {
		return kont.Right[string, int](x * 2)
	})

	val, ok := result.GetRight()
	if !ok || val != 42 {
		t.Fatalf("got %d, want 42", val)
	}

	// FlatMap with error in second computation
	result2 := kont.FlatMapEither(right, func(x int) kont.Either[string, int] {
		return kont.Left[string, int]("second error")
	})

	if result2.IsRight() {
		t.Fatal("expected Left from second computation")
	}
}

func TestMapLeftEither(t *testing.T) {
	left := kont.Left[string, int]("error")
	mapped := kont.MapLeftEither(left, func(e string) string {
		return "wrapped: " + e
	})

	err, ok := mapped.GetLeft()
	if !ok || err != "wrapped: error" {
		t.Fatalf("got %q, want %q", err, "wrapped: error")
	}
}
