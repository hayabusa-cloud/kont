// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestBracketSuccess(t *testing.T) {
	var acquired, released bool

	// Build a bracketed computation
	comp := kont.Bracket[string, int, int](
		// acquire
		kont.Return[kont.Resumed](42),
		// release
		func(r int) kont.Cont[kont.Resumed, struct{}] {
			released = true
			return kont.Return[kont.Resumed](struct{}{})
		},
		// use
		func(r int) kont.Cont[kont.Resumed, int] {
			acquired = true
			return kont.Return[kont.Resumed](r * 2)
		},
	)

	result := kont.Handle(comp, kont.HandleFunc[kont.Either[string, int]](func(op kont.Operation) (kont.Resumed, bool) {
		panic("no effects expected")
	}))

	if !result.IsRight() {
		t.Fatalf("expected Right, got Left")
	}
	val, _ := result.GetRight()
	if val != 84 {
		t.Fatalf("got %d, want 84", val)
	}
	if !acquired {
		t.Fatal("resource not acquired")
	}
	if !released {
		t.Fatal("resource not released")
	}
}

func TestBracketReleasesOnError(t *testing.T) {
	var released bool

	// Build a bracketed computation that throws an error
	comp := kont.Bracket[string, int, int](
		// acquire
		kont.Return[kont.Resumed](42),
		// release
		func(r int) kont.Cont[kont.Resumed, struct{}] {
			released = true
			return kont.Return[kont.Resumed](struct{}{})
		},
		// use - throws error
		func(r int) kont.Cont[kont.Resumed, int] {
			return kont.ThrowError[string, int]("intentional error")
		},
	)

	result := kont.Handle(comp, kont.HandleFunc[kont.Either[string, int]](func(op kont.Operation) (kont.Resumed, bool) {
		// Handle error effect
		switch o := op.(type) {
		case kont.Throw[string]:
			return kont.Left[string, int](o.Err), false
		}
		panic("unexpected effect")
	}))

	if result.IsRight() {
		t.Fatal("expected Left (error), got Right")
	}
	errVal, _ := result.GetLeft()
	if errVal != "intentional error" {
		t.Fatalf("got error %q, want %q", errVal, "intentional error")
	}
	if !released {
		t.Fatal("resource not released after error")
	}
}

func TestOnErrorRunsOnError(t *testing.T) {
	var cleanedUp bool
	var capturedError string

	comp := kont.OnError[string, int](
		kont.ThrowError[string, int]("test error"),
		func(e string) kont.Cont[kont.Resumed, struct{}] {
			cleanedUp = true
			capturedError = e
			return kont.Return[kont.Resumed](struct{}{})
		},
	)

	result := kont.RunError[string, int](comp)

	if result.IsRight() {
		t.Fatal("expected Left (error), got Right")
	}
	errVal, _ := result.GetLeft()
	if errVal != "test error" {
		t.Fatalf("got error %q, want %q", errVal, "test error")
	}
	if !cleanedUp {
		t.Fatal("cleanup not called on error")
	}
	if capturedError != "test error" {
		t.Fatalf("captured error %q, want %q", capturedError, "test error")
	}
}

func TestOnErrorSkippedOnSuccess(t *testing.T) {
	var cleanedUp bool

	comp := kont.OnError[string, int](
		kont.Return[kont.Resumed](42),
		func(e string) kont.Cont[kont.Resumed, struct{}] {
			cleanedUp = true
			return kont.Return[kont.Resumed](struct{}{})
		},
	)

	result := kont.RunError[string, int](comp)

	if !result.IsRight() {
		t.Fatal("expected Right, got Left")
	}
	val, _ := result.GetRight()
	if val != 42 {
		t.Fatalf("got %d, want 42", val)
	}
	if cleanedUp {
		t.Fatal("cleanup should not be called on success")
	}
}
