// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"sync"
	"testing"

	"code.hybscloud.com/kont"
)

func TestAffineResume(t *testing.T) {
	k := func(x int) string {
		return "received"
	}
	aff := kont.Once(k)

	got := aff.Resume(42)
	if got != "received" {
		t.Fatalf("got %q, want %q", got, "received")
	}

	// After resume, TryResume must fail
	_, ok := aff.TryResume(0)
	if ok {
		t.Fatal("expected TryResume to fail after Resume")
	}
}

func TestAffinePanicOnReuse(t *testing.T) {
	k := func(x int) int { return x * 2 }
	aff := kont.Once(k)

	// First resume should succeed
	_ = aff.Resume(10)

	// Second resume should panic
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on second Resume")
		}
		if s, ok := r.(string); !ok || s != "kont: affine continuation resumed twice" {
			t.Fatalf("unexpected panic message: %v", r)
		}
	}()

	_ = aff.Resume(20)
}

func TestAffineTryResume(t *testing.T) {
	k := func(x int) int { return x * 2 }
	aff := kont.Once(k)

	// First try should succeed
	got, ok := aff.TryResume(10)
	if !ok {
		t.Fatal("expected first TryResume to succeed")
	}
	if got != 20 {
		t.Fatalf("got %d, want 20", got)
	}

	// Second try should fail without panic
	got, ok = aff.TryResume(20)
	if ok {
		t.Fatal("expected second TryResume to fail")
	}
	if got != 0 {
		t.Fatalf("got %d, want 0 on failed TryResume", got)
	}
}

func TestAffineDiscard(t *testing.T) {
	k := func(x int) int { return x }
	aff := kont.Once(k)

	aff.Discard()

	// Resume after discard should fail
	_, ok := aff.TryResume(42)
	if ok {
		t.Fatal("expected TryResume to fail after Discard")
	}
}

func TestAffineDiscardThenPanic(t *testing.T) {
	k := func(x int) int { return x }
	aff := kont.Once(k)
	aff.Discard()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic after Discard")
		}
	}()

	_ = aff.Resume(42)
}

func TestAffineConcurrentResume(t *testing.T) {
	k := func(x int) int { return x }
	aff := kont.Once(k)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	successCount := make(chan int, goroutines)
	panicCount := make(chan int, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount <- 1
				}
			}()
			_ = aff.Resume(1)
			successCount <- 1
		}()
	}

	wg.Wait()
	close(successCount)
	close(panicCount)

	successes := 0
	for range successCount {
		successes++
	}

	panics := 0
	for range panicCount {
		panics++
	}

	if successes != 1 {
		t.Fatalf("expected exactly 1 success, got %d", successes)
	}
	if panics != goroutines-1 {
		t.Fatalf("expected %d panics, got %d", goroutines-1, panics)
	}
}

func TestAffineConcurrentTryResume(t *testing.T) {
	k := func(x int) int { return x }
	aff := kont.Once(k)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	successCount := make(chan int, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			if _, ok := aff.TryResume(1); ok {
				successCount <- 1
			}
		}()
	}

	wg.Wait()
	close(successCount)

	successes := 0
	for range successCount {
		successes++
	}

	if successes != 1 {
		t.Fatalf("expected exactly 1 success, got %d", successes)
	}
}

// --- Benchmarks ---

func BenchmarkAffineResume(b *testing.B) {
	for b.Loop() {
		aff := kont.Once(func(x int) int { return x })
		_ = aff.Resume(42)
	}
}

func BenchmarkAffineTryResume(b *testing.B) {
	for b.Loop() {
		aff := kont.Once(func(x int) int { return x })
		aff.TryResume(42)
	}
}
