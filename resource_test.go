// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestBracketGoPanic(t *testing.T) {
	released := false
	m := kont.Bracket[string](kont.Pure(1), func(int) kont.Eff[struct{}] {
		released = true
		return kont.Pure(struct{}{})
	}, func(int) kont.Eff[int] {
		return kont.Suspend(func(func(int) kont.Resumed) kont.Resumed { panic("use panicked") })
	})
	defer func() {
		if cause := recover(); cause != "use panicked" || released {
			t.Fatalf("panic = %v, released = %v", cause, released)
		}
	}()
	kont.Step(m)
}

func TestBracketDiscardDuringRelease(t *testing.T) {
	finished := false
	m := kont.Bracket[string](kont.Pure(1), func(int) kont.Eff[struct{}] {
		return kont.GetState(func(int) kont.Eff[struct{}] {
			finished = true
			return kont.Pure(struct{}{})
		})
	}, func(n int) kont.Eff[int] { return kont.Pure(n) })
	_, s := kont.Step(m)
	if s == nil || s.Op() != (kont.Get[int]{}) {
		t.Fatal("expected the release operation")
	}
	s.Discard()
	if _, _, ok := s.TryResume(1); ok || finished {
		t.Fatal("abandoned release callback ran")
	}
}

func TestBracketReleaseErrorReachesOuterHandler(t *testing.T) {
	m := kont.Bracket[string](kont.Pure(1),
		func(int) kont.Eff[struct{}] { return kont.ThrowError[string, struct{}]("release") },
		func(int) kont.Eff[int] { return kont.ThrowError[string, int]("use") },
	)
	if err, ok := kont.RunError[string](m).GetLeft(); !ok || err != "release" {
		t.Fatalf("outer error = (%q, %t), want release failure", err, ok)
	}
}

func TestBracketSuccess(t *testing.T) {
	var acquired, released bool

	// Build a bracketed computation
	comp := kont.Bracket[string, int, int](
		// acquire
		kont.Pure(42),
		// release
		func(r int) kont.Eff[struct{}] {
			released = true
			return kont.Pure(struct{}{})
		},
		// use
		func(r int) kont.Eff[int] {
			acquired = true
			return kont.Pure(r * 2)
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
		kont.Pure(42),
		// release
		func(r int) kont.Eff[struct{}] {
			released = true
			return kont.Pure(struct{}{})
		},
		// use - throws error
		func(r int) kont.Eff[int] {
			return kont.ThrowError[string, int]("intentional error")
		},
	)

	result := kont.Handle(comp, kont.HandleFunc[kont.Either[string, int]](func(op kont.Operation) (kont.Resumed, bool) {
		panic("Bracket should return a pure Either; no outer effects expected")
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
		func(e string) kont.Eff[struct{}] {
			cleanedUp = true
			capturedError = e
			return kont.Pure(struct{}{})
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
		kont.Pure(42),
		func(e string) kont.Eff[struct{}] {
			cleanedUp = true
			return kont.Pure(struct{}{})
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

func TestOnErrorCleanupFailure(t *testing.T) {
	m := kont.OnError(kont.ThrowError[string, int]("body"), func(string) kont.Eff[struct{}] {
		return kont.ThrowError[string, struct{}]("cleanup")
	})
	if err, ok := kont.RunError[string](m).GetLeft(); !ok || err != "cleanup" {
		t.Fatalf("error = (%q, %t), want cleanup failure", err, ok)
	}
}
