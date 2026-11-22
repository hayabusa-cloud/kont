// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"os"
	"os/exec"
	"runtime/debug"
	"sync"
	"testing"

	"code.hybscloud.com/kont"
)

func TestAcquireEffectFrame(t *testing.T) {
	ef := kont.AcquireEffectFrame()
	ef.Operation = kont.Get[int]{}
	ef.Resume = func(v any) any { return v }
	ef.Next = kont.ReturnFrame{}

	expr := kont.Expr[int]{Frame: ef}
	result := kont.HandleExpr(expr, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 42, true
	}))
	if result != 42 {
		t.Fatalf("got %v, want 42", result)
	}
}

func TestAcquireBindFrame(t *testing.T) {
	bf := kont.AcquireBindFrame()
	bf.F = func(a any) kont.Expr[any] {
		return kont.ExprReturn[any](a.(int) * 2)
	}
	bf.Next = kont.ReturnFrame{}

	expr := kont.Expr[int]{Value: 21, Frame: bf}
	result := kont.RunPure(expr)
	if result != 42 {
		t.Fatalf("got %v, want 42", result)
	}
}

func TestAcquireThenFrame(t *testing.T) {
	tf := kont.AcquireThenFrame()
	tf.Second = kont.Expr[any]{Value: 99, Frame: kont.ReturnFrame{}}
	tf.Next = kont.ReturnFrame{}

	expr := kont.Expr[int]{Value: 0, Frame: tf}
	result := kont.RunPure(expr)
	if result != 99 {
		t.Fatalf("got %v, want 99", result)
	}
}

func pooledShortCircuitFrames() (*kont.EffectFrame[kont.Erased], *kont.BindFrame[kont.Erased, kont.Erased], *kont.UnwindFrame) {
	effect := kont.AcquireEffectFrame()
	effect.Operation = kont.Get[int]{}
	effect.Resume = func(kont.Erased) kont.Erased { panic("abandoned resume") }
	bind := kont.AcquireBindFrame()
	bind.F = func(kont.Erased) kont.Expr[kont.Erased] { panic("abandoned bind") }
	unwind := kont.AcquireUnwindFrame()
	unwind.Data1, unwind.Data2, unwind.Data3 = 1, 2, 3
	unwind.Unwind = func(_, _, _, _ kont.Erased) (kont.Erased, kont.Frame) {
		panic("abandoned unwind")
	}
	effect.Next, bind.Next = bind, unwind
	return effect, bind, unwind
}

func TestHandleExprShortCircuitReleasesPooledContinuation(t *testing.T) {
	for _, shape := range []string{"direct", "chained"} {
		t.Run(shape, func(t *testing.T) {
			effect, bind, unwind := pooledShortCircuitFrames()
			var root kont.Frame = effect
			if shape == "chained" {
				bind.Next = kont.ReturnFrame{}
				root = kont.ChainFrames(effect, unwind)
			}
			result := kont.HandleExpr(kont.ExprSuspend[int](root),
				kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) {
					return 99, false
				}))
			if result != 99 {
				t.Fatalf("got %d, want 99", result)
			}
			if effect.Operation != nil || effect.Resume != nil || effect.Next != nil {
				t.Error("short-circuit retained the pooled effect frame")
			}
			if bind.F != nil || bind.Next != nil {
				t.Error("short-circuit retained the pooled bind frame")
			}
			if unwind.Data1 != nil || unwind.Data2 != nil || unwind.Data3 != nil || unwind.Unwind != nil {
				t.Error("short-circuit retained the pooled unwind frame")
			}
		})
	}
}

func TestHandleExprShortCircuitPreservesReusableFrames(t *testing.T) {
	expr := kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprReturn(42))
	result := kont.HandleExpr(expr, kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) {
		return 99, false
	}))
	if result != 99 {
		t.Fatalf("short-circuit got %d, want 99", result)
	}
	result = kont.HandleExpr(expr, kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) {
		return 1, true
	}))
	if result != 42 {
		t.Fatalf("reused expression got %d, want 42", result)
	}
}

func TestAbandonTypedFrameLinks(t *testing.T) {
	for _, exit := range []string{"handler", "discard"} {
		t.Run(exit, func(t *testing.T) {
			left, right := kont.AcquireUnwindFrame(), kont.AcquireUnwindFrame()
			left.Unwind = func(_, _, _, _ any) (any, kont.Frame) { panic("abandoned left branch") }
			right.Unwind = left.Unwind
			bind := &kont.BindFrame[string, string]{F: func(string) kont.Expr[string] { panic("abandoned bind") }, Next: left}
			mapped := &kont.MapFrame[string, int]{F: func(string) int { panic("abandoned map") }, Next: right}
			seq := &kont.ThenFrame[int, string]{Second: kont.ExprSuspend[string](bind.AsFrame()), Next: mapped.AsFrame()}
			first := kont.ExprPerform(kont.Get[int]{})
			deferred := kont.ExprThen(first, kont.ExprSuspend[int](seq.AsFrame()))
			deferred = kont.ExprMap(deferred, func(int) int { panic("abandoned callback tail") })
			deferred = kont.ExprBind(deferred, func(int) kont.Expr[int] { panic("abandoned bind tail") })
			m := kont.ExprThen(first, deferred)
			if exit == "handler" {
				if got := kont.HandleExpr(m, kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 42, false })); got != 42 {
					t.Fatalf("terminal result = %d", got)
				}
			} else {
				_, s := kont.StepExpr(m)
				s.Discard()
			}
			if left.Unwind != nil || right.Unwind != nil {
				t.Fatal("typed frame retained a pooled child")
			}
			if bind.F == nil || bind.Next != left || mapped.F == nil || mapped.Next != right || seq.Second.Frame != bind.AsFrame() || seq.Next != mapped.AsFrame() {
				t.Fatal("disposal modified an unpooled parent")
			}
		})
	}
}

type customMapState struct{ *kont.MapFrame[int, int] }

type customMapFrame struct {
	kont.ReturnFrame
	*customMapState
	mu sync.Mutex
}

func (f *customMapFrame) Unwind(v any) (any, kont.Frame) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.customMapState.Unwind(v)
}

func TestAbandonKeepsCustomFrameOpaque(t *testing.T) {
	leaf := kont.AcquireUnwindFrame()
	leaf.Unwind = func(_, _, _, v any) (any, kont.Frame) { return v, kont.ReturnFrame{} }
	embedded := &customMapFrame{customMapState: &customMapState{&kont.MapFrame[int, int]{Next: leaf}}}
	bind := struct{ *kont.BindFrame[any, any] }{&kont.BindFrame[any, any]{Next: leaf}}
	seq := struct{ *kont.ThenFrame[any, any] }{&kont.ThenFrame[any, any]{Second: kont.ExprSuspend[any](leaf)}}
	for _, tail := range []kont.Frame{embedded, bind, seq, &customMapFrame{}} {
		m := kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprSuspend[int](tail))
		_, s := kont.StepExpr(m)
		s.Discard()
	}
	if leaf.Unwind == nil {
		t.Fatal("discard reached through a custom frame")
	}
	kont.RunPure(kont.ExprSuspend[int](leaf))
}

func TestAbandonCustomFrameConcurrentUpdate(t *testing.T) {
	for _, exit := range []string{"handler", "discard", "resume"} {
		t.Run(exit, func(t *testing.T) {
			inc := func(n int) int { return n + 1 }
			states := [2]*customMapState{
				{&kont.MapFrame[int, int]{F: inc, Next: kont.ReturnFrame{}}},
				{&kont.MapFrame[int, int]{F: inc, Next: kont.ReturnFrame{}}},
			}
			f := &customMapFrame{customMapState: states[0]}
			m := kont.ExprSuspend[int](kont.ChainFrames(kont.ExprPerform(kont.Get[int]{}).Frame, f))
			resume := exit == "resume"
			h := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 17, resume })
			want := 17
			if resume {
				want++
			}

			var writer sync.WaitGroup
			writer.Go(func() {
				for i := range 128 {
					f.mu.Lock()
					f.customMapState = states[i%2]
					f.mu.Unlock()
				}
			})
			for range 128 {
				if exit == "discard" {
					_, s := kont.StepExpr(m)
					s.Discard()
				} else if got := kont.HandleExpr(m, h); got != want {
					t.Errorf("got %d, want %d", got, want)
					break
				}
			}
			writer.Wait()
		})
	}
}

func TestHandleExprShortCircuitReleasesAllBranches(t *testing.T) {
	var leaves [16]*kont.UnwindFrame
	var frames [len(leaves)]kont.Frame
	for i := range leaves {
		leaf := kont.AcquireUnwindFrame()
		leaf.Unwind = func(_, _, _, _ kont.Erased) (kont.Erased, kont.Frame) {
			panic("abandoned unwind")
		}
		leaves[i], frames[i] = leaf, leaf
	}
	// Both children of the upper forks still have pending branches.
	for width := len(frames); width > 2; width /= 2 {
		for i := 0; i < width; i += 2 {
			frames[i/2] = kont.ChainFrames(frames[i], frames[i+1])
		}
	}
	fork := kont.AcquireThenFrame()
	fork.Second = kont.ExprSuspend[kont.Erased](frames[0])
	fork.Next = frames[1]
	m := kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprSuspend[int](fork))
	got := kont.HandleExpr(m, kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 99, false }))
	if got != 99 {
		t.Fatalf("got %d, want 99", got)
	}
	if fork.Second.Frame != nil || fork.Next != nil {
		t.Error("abandoned then frame retained a branch")
	}
	for i, leaf := range leaves {
		if leaf.Unwind != nil {
			t.Errorf("abandoned leaf %d retained its callback", i)
		}
	}
}

// SetMaxStack applies to the whole process. Run each depth test in its own child.
func inSmallStackProcess(t *testing.T) bool {
	t.Helper()
	const child = "KONT_TEST_SMALL_STACK"
	if os.Getenv(child) == t.Name() {
		debug.SetMaxStack(256 << 10)
		return true
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = append(cmd.Environ(), child+"="+t.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bounded-stack evaluation: %v\n%s", err, out)
	}
	return false
}

func TestHandleExprShortCircuitDeepFrames(t *testing.T) {
	if !inSmallStackProcess(t) {
		return
	}
	effect, bind, left := pooledShortCircuitFrames()
	right := kont.AcquireUnwindFrame()
	right.Unwind = left.Unwind
	sibling := &kont.UnwindFrame{Unwind: left.Unwind}
	var subtree kont.Frame = left
	for range 4096 {
		subtree = (&kont.ThenFrame[int, int]{
			Second: kont.ExprSuspend[int](kont.ChainFrames(subtree, sibling)),
			Next:   sibling,
		}).AsFrame()
		subtree = &kont.ThenFrame[kont.Erased, kont.Erased]{
			Second: kont.ExprSuspend[kont.Erased](kont.ChainFrames(subtree, sibling)),
			Next:   sibling,
		}
	}
	bind.Next = kont.ChainFrames(subtree, right)
	got := kont.HandleExpr(kont.ExprSuspend[int](effect),
		kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 99, false }))
	if got != 99 || left.Unwind != nil || right.Unwind != nil || sibling.Unwind == nil {
		t.Fatal("deep short-circuit lost its result or mishandled frame ownership")
	}
}

func BenchmarkHandleExprPooledShortCircuit(b *testing.B) {
	handler := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) {
		return 99, false
	})
	b.ReportAllocs()
	for b.Loop() {
		effect, _, _ := pooledShortCircuitFrames()
		if result := kont.HandleExpr(kont.ExprSuspend[int](effect), handler); result != 99 {
			b.Fatalf("got %d, want 99", result)
		}
	}
}

func BenchmarkHandleExprShortCircuitMapChain(b *testing.B) {
	tail := kont.ExprPerform(kont.Get[int]{})
	for range 64 {
		tail = kont.ExprMap(tail, func(int) int { panic("abandoned map") })
	}
	m := kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), tail)
	handler := kont.HandleFunc[int](func(kont.Operation) (kont.Resumed, bool) { return 99, false })
	b.ReportAllocs()
	for b.Loop() {
		if got := kont.HandleExpr(m, handler); got != 99 {
			b.Fatalf("got %d, want 99", got)
		}
	}
}
