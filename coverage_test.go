// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// Edge cases for coverage

func TestReturnZeroValue(t *testing.T) {
	// Zero value of various types
	got := kont.Run(kont.Return[int](0))
	if got != 0 {
		t.Fatalf("got %d, want 0", got)
	}

	gotStr := kont.Run(kont.Return[string](""))
	if gotStr != "" {
		t.Fatalf("got %q, want empty string", gotStr)
	}
}

func TestSuspendIdentity(t *testing.T) {
	// Suspend with identity function
	m := kont.Suspend[int, int](func(k func(int) int) int {
		return k(42)
	})
	got := kont.Run(m)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestRunWithCustomContinuation(t *testing.T) {
	// RunWith with a transformation continuation
	m := kont.Return[[]int, int](42)
	got := kont.RunWith(m, func(x int) []int {
		return []int{x, x * 2}
	})
	if len(got) != 2 || got[0] != 42 || got[1] != 84 {
		t.Fatalf("got %v, want [42 84]", got)
	}
}

func TestMatchEither(t *testing.T) {
	// MatchEither on Right
	right := kont.Right[string](42)
	result := kont.MatchEither(right,
		func(e string) int { return 0 },
		func(a int) int { return a * 2 },
	)
	if result != 84 {
		t.Fatalf("got %d, want 84", result)
	}

	// MatchEither on Left
	left := kont.Left[string, int]("error")
	resultStr := kont.MatchEither(left,
		func(e string) string { return "left: " + e },
		func(a int) string { return "right" },
	)
	if resultStr != "left: error" {
		t.Fatalf("got %q, want %q", resultStr, "left: error")
	}
}

func TestEitherGetLeftOnRight(t *testing.T) {
	// GetLeft on Right should return zero, false
	right := kont.Right[string](42)
	val, ok := right.GetLeft()
	if ok {
		t.Fatal("GetLeft on Right should return false")
	}
	if val != "" {
		t.Fatalf("got %q, want empty string", val)
	}
}

func TestEitherGetRightOnLeft(t *testing.T) {
	// GetRight on Left should return zero, false
	left := kont.Left[string, int]("error")
	val, ok := left.GetRight()
	if ok {
		t.Fatal("GetRight on Left should return false")
	}
	if val != 0 {
		t.Fatalf("got %d, want 0", val)
	}
}

func TestMapLeftEitherTypeChange(t *testing.T) {
	// MapLeftEither changing error type from string to int
	left := kont.Left[string, int]("err")
	result := kont.MapLeftEither(left, func(e string) int {
		return len(e)
	})
	if result.IsRight() {
		t.Fatal("should still be Left")
	}
	val, _ := result.GetLeft()
	if val != 3 {
		t.Fatalf("got %d, want 3", val)
	}

	// MapLeftEither on Right preserves value type
	right := kont.Right[string](42)
	result2 := kont.MapLeftEither(right, func(e string) int {
		return len(e)
	})
	if !result2.IsRight() {
		t.Fatal("should still be Right")
	}
	val2, _ := result2.GetRight()
	if val2 != 42 {
		t.Fatalf("got %d, want 42", val2)
	}
}

func TestMapEitherOnLeft(t *testing.T) {
	// MapEither on Left should preserve Left
	left := kont.Left[string, int]("error")
	result := kont.MapEither(left, func(x int) int {
		return x * 2
	})
	if result.IsRight() {
		t.Fatal("MapEither on Left should preserve Left")
	}
	errVal, _ := result.GetLeft()
	if errVal != "error" {
		t.Fatalf("got %q, want %q", errVal, "error")
	}
}

func TestPairType(t *testing.T) {
	p := kont.Pair[int, string]{Fst: 42, Snd: "hello"}
	if p.Fst != 42 {
		t.Fatalf("Fst: got %d, want 42", p.Fst)
	}
	if p.Snd != "hello" {
		t.Fatalf("Snd: got %q, want %q", p.Snd, "hello")
	}
}

func TestWriterContextType(t *testing.T) {
	// Verify WriterContext is accessible
	var output []int
	ctx := &kont.WriterContext[int]{Output: &output}
	if ctx.Output != &output {
		t.Fatal("WriterContext.Output should point to output slice")
	}
}

func TestReturnAndBindEffect(t *testing.T) {
	// Return + Bind with no effects
	m := kont.Bind(
		kont.Pure(10),
		func(x int) kont.Eff[int] {
			return kont.Pure(x * 2)
		},
	)
	result := kont.Handle(m, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("no effects expected")
	}))
	if result != 20 {
		t.Fatalf("got %d, want 20", result)
	}
}

func TestHandleFuncWrapper(t *testing.T) {
	// HandleFunc should correctly wrap a function
	h := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 42, true
	})
	// Verify it's a valid handler by using Dispatch directly
	v, shouldResume := h.Dispatch("test")
	if !shouldResume {
		t.Fatal("expected shouldResume=true")
	}
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
}

// Writer Listen/Censor operation creation coverage

func TestListenWriterCreation(t *testing.T) {
	// ListenWriter creates a Cont that performs a Listen effect
	body := kont.Pure(42)
	cont := kont.ListenWriter[string, int](body)
	// Verify it's a valid continuation (type check)
	_ = cont
}

func TestCensorWriterCreation(t *testing.T) {
	// CensorWriter creates a Cont that performs a Censor effect
	body := kont.Pure(42)
	cont := kont.CensorWriter[string](func(logs []string) []string {
		return logs
	}, body)
	// Verify it's a valid continuation (type check)
	_ = cont
}

func TestWriterHandlerUnhandledPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from unhandled effect")
		}
	}()

	h, _ := kont.WriterHandler[string, int]()
	// Pass an unknown effect type via Dispatch
	h.Dispatch(struct{ unknown bool }{})
}

// ThrowError coverage with type inference

func TestThrowErrorTypeInference(t *testing.T) {
	// ThrowError returns Eff[A] where A is inferred
	comp := kont.ThrowError[string, int]("test error")

	result := kont.RunError[string, int](comp)
	if result.IsRight() {
		t.Fatal("expected Left, got Right")
	}
	errVal, _ := result.GetLeft()
	if errVal != "test error" {
		t.Fatalf("got %q, want %q", errVal, "test error")
	}
}

// Additional RunError coverage

func TestRunErrorCatchSuccess(t *testing.T) {
	// Catch where body succeeds
	comp := kont.CatchError[string, int](
		kont.Pure(42),
		func(e string) kont.Eff[int] {
			return kont.Pure(0)
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
}

// RunError catch path with recovery

func TestRunErrorCatchWithRecovery(t *testing.T) {
	// Catch where body throws and handler recovers
	comp := kont.CatchError[string, int](
		kont.ThrowError[string, int]("original error"),
		func(e string) kont.Eff[int] {
			if e == "original error" {
				return kont.Pure(99)
			}
			return kont.ThrowError[string, int]("unexpected: " + e)
		},
	)

	result := kont.RunError[string, int](comp)
	if !result.IsRight() {
		t.Fatal("expected Right after catch recovery")
	}
	val, _ := result.GetRight()
	if val != 99 {
		t.Fatalf("got %d, want 99", val)
	}
}

// ThrowError short-circuit path

func TestThrowErrorShortCircuit(t *testing.T) {
	// ThrowError uses effectMarker to trigger the Throw effect
	// The continuation is never called because Throw short-circuits
	comp := kont.ThrowError[string, int]("error")

	result := kont.RunError[string, int](comp)
	if result.IsRight() {
		t.Fatal("expected Left")
	}
}

// RunError with nested catch

func TestRunErrorNestedCatch(t *testing.T) {
	// Outer catch -> inner catch -> inner throw
	comp := kont.CatchError[string, int](
		kont.CatchError[string, int](
			kont.ThrowError[string, int]("inner error"),
			func(e string) kont.Eff[int] {
				return kont.ThrowError[string, int]("rethrown: " + e)
			},
		),
		func(e string) kont.Eff[int] {
			if e == "rethrown: inner error" {
				return kont.Pure(100)
			}
			return kont.ThrowError[string, int]("unexpected: " + e)
		},
	)

	result := kont.RunError[string, int](comp)
	if !result.IsRight() {
		err, _ := result.GetLeft()
		t.Fatalf("expected Right, got Left with error: %s", err)
	}
	val, _ := result.GetRight()
	if val != 100 {
		t.Fatalf("got %d, want 100", val)
	}
}

// FlatMapEither edge case

func TestFlatMapEitherLeft(t *testing.T) {
	left := kont.Left[string, int]("error")
	result := kont.FlatMapEither(left, func(x int) kont.Either[string, int] {
		return kont.Right[string](x * 2)
	})

	if result.IsRight() {
		t.Fatal("expected Left")
	}
	errVal, _ := result.GetLeft()
	if errVal != "error" {
		t.Fatalf("got %q, want %q", errVal, "error")
	}
}

// handleDispatch nil path coverage

func TestHandleResultNilReturn(t *testing.T) {
	// Create a computation that returns nil directly.
	// This exercises the nil check in handleDispatch.
	nilReturningComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		// Don't call k, just return nil directly
		return nil
	})

	// Use a simple handler that should never be called
	h := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		t.Fatal("handler should not be called")
		return 0, true
	})

	result := kont.Handle(nilReturningComp, h)
	// When result is nil, handleDispatch returns the zero value of int
	if result != 0 {
		t.Fatalf("got %d, want 0", result)
	}
}

// testOp is a simple test operation for coverage tests
type testOp struct{}

func (testOp) OpResult() int { panic("phantom") }

// OpResult phantom method tests
// These methods exist for type inference and should panic if called directly.
// Testing panic behavior validates they work as designed.

func TestOpResultPanicPhantom(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Phantom.OpResult should panic")
		}
	}()
	var p kont.Phantom[int]
	p.OpResult()
}

func TestOpResultPanicThrow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Throw.OpResult should panic")
		}
	}()
	var op kont.Throw[string]
	op.OpResult()
}

func TestOpResultPanicCatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Catch.OpResult should panic")
		}
	}()
	var op kont.Catch[string, int]
	op.OpResult()
}

func TestOpResultPanicAsk(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Ask.OpResult should panic")
		}
	}()
	var op kont.Ask[string]
	op.OpResult()
}

func TestOpResultPanicGet(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Get.OpResult should panic")
		}
	}()
	var op kont.Get[int]
	op.OpResult()
}

func TestOpResultPanicPut(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Put.OpResult should panic")
		}
	}()
	var op kont.Put[int]
	op.OpResult()
}

func TestOpResultPanicModify(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Modify.OpResult should panic")
		}
	}()
	var op kont.Modify[int]
	op.OpResult()
}

func TestOpResultPanicTell(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Tell.OpResult should panic")
		}
	}()
	var op kont.Tell[string]
	op.OpResult()
}

func TestOpResultPanicListen(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Listen.OpResult should panic")
		}
	}()
	var op kont.Listen[string, int]
	op.OpResult()
}

func TestOpResultPanicCensor(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Censor.OpResult should panic")
		}
	}()
	var op kont.Censor[string, int]
	op.OpResult()
}

func TestRunErrorUnhandledEffect(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from unhandled effect")
		}
	}()

	// Create a computation with a non-error effect
	comp := kont.Perform(kont.Get[int]{}) // State effect, not Error effect

	// RunError only handles Error effects
	kont.RunError[string, int](comp)
}

// floatOp is a test operation returning float64.
type floatOp struct{}

func (floatOp) OpResult() float64 { panic("phantom") }

// anyOp is a test operation returning any.
type anyOp struct{ v any }

func (anyOp) OpResult() any { panic("phantom") }

// boolOp is a test operation returning bool.
type boolOp struct{ want bool }

func (boolOp) OpResult() bool { panic("phantom") }

func TestEffectMarkerFloat(t *testing.T) {
	// Perform with float64 result type through HandleFunc
	comp := kont.Perform(floatOp{})

	result := kont.Handle(comp, kont.HandleFunc[float64](func(op kont.Operation) (kont.Resumed, bool) {
		if _, ok := op.(floatOp); ok {
			return 3.14, true
		}
		panic("unexpected operation")
	}))

	if result != 3.14 {
		t.Fatalf("got %v, want 3.14", result)
	}
}

func TestEffectMarkerAnyType(t *testing.T) {
	// Perform with any result type through HandleFunc
	comp := kont.Perform(anyOp{v: "test"})

	result := kont.Handle(comp, kont.HandleFunc[any](func(op kont.Operation) (kont.Resumed, bool) {
		if o, ok := op.(anyOp); ok {
			return o.v, true
		}
		panic("unexpected operation")
	}))

	if result != "test" {
		t.Fatalf("got %v, want 'test'", result)
	}
}

func TestEffectMarkerBoolType(t *testing.T) {
	// Perform with bool result type through HandleFunc
	comp := kont.Perform(boolOp{want: true})

	result := kont.Handle(comp, kont.HandleFunc[bool](func(op kont.Operation) (kont.Resumed, bool) {
		if o, ok := op.(boolOp); ok {
			return o.want, true
		}
		panic("unexpected operation")
	}))

	if result != true {
		t.Fatalf("got %v, want true", result)
	}
}

// =============================================================================
// Coverage: Then combinator (monad.go)
// =============================================================================

func TestThenCombinator(t *testing.T) {
	// Then sequences two computations, discarding the first result
	first := kont.Return[int](42)
	second := kont.Return[int](100)

	result := kont.Run(kont.Then(first, second))
	if result != 100 {
		t.Fatalf("got %d, want 100", result)
	}
}

func TestThenWithEffects(t *testing.T) {
	// Then with effectful computations — TellWriter now fuses Tell+Then directly
	comp := kont.TellWriter("first", kont.TellWriter("second", kont.Pure(42)))

	result, logs := kont.RunWriter[string, int](comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 2 || logs[0] != "first" || logs[1] != "second" {
		t.Fatalf("got logs %v, want [first second]", logs)
	}
}

// =============================================================================
// Coverage: Dispatch paths via HandleFunc (short-circuit)
// =============================================================================

func TestHandleDispatchShortCircuit(t *testing.T) {
	// When shouldResume=false, handleDispatch returns the value directly
	comp := kont.Perform(testOp{})
	result := kont.Handle(comp, kont.HandleFunc[int](func(_ kont.Operation) (kont.Resumed, bool) {
		return 99, false // short-circuit, don't resume
	}))
	if result != 99 {
		t.Fatalf("got %d, want 99", result)
	}
}

func TestHandleDispatchNilResult(t *testing.T) {
	// A computation that returns nil after dispatch should return zero value
	nilComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		return nil
	})
	result := kont.Handle(nilComp, kont.HandleFunc[int](func(_ kont.Operation) (kont.Resumed, bool) {
		return nil, true
	}))
	if result != 0 {
		t.Fatalf("got %d, want 0", result)
	}
}

// =============================================================================
// Coverage: Handler panic paths
// =============================================================================

func TestStateHandlerUnhandledPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from unhandled effect")
		}
	}()
	h, _ := kont.StateHandler[int, int](0)
	h.Dispatch(struct{ unknown bool }{})
}

func TestReaderHandlerUnhandledPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from unhandled effect")
		}
	}()
	h := kont.ReaderHandler[string, int]("env")
	h.Dispatch(struct{ unknown bool }{})
}

func TestWriterDispatchUnhandledPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from unhandled effect")
		}
	}()
	h, _ := kont.WriterHandler[string, int]()
	h.Dispatch(struct{ unknown bool }{})
}

// =============================================================================
// Coverage: Dispatch paths via standard handlers
// =============================================================================

func TestStateDispatchHandler(t *testing.T) {
	// Exercise the Dispatch path directly for all State ops
	comp := kont.GetState(func(x int) kont.Eff[string] {
		return kont.PutState(x+1,
			kont.ModifyState(func(s int) int { return s * 2 }, func(s int) kont.Eff[string] {
				return kont.Pure("done")
			}),
		)
	})

	result, state := kont.RunState[int, string](10, comp)
	if result != "done" {
		t.Fatalf("result = %q, want %q", result, "done")
	}
	// 10 → get → put(11) → modify(*2) → 22
	if state != 22 {
		t.Fatalf("state = %d, want 22", state)
	}
}

func TestReaderDispatchHandler(t *testing.T) {
	// Exercise Reader dispatch path
	comp := kont.AskReader(func(env string) kont.Eff[string] {
		return kont.Pure("env: " + env)
	})

	result := kont.RunReader[string, string]("test-env", comp)
	if result != "env: test-env" {
		t.Fatalf("result = %q, want %q", result, "env: test-env")
	}
}

func TestWriterDispatchHandler(t *testing.T) {
	// Exercise Writer dispatch path
	comp := kont.TellWriter("log1", kont.TellWriter("log2", kont.Pure(42)))

	result, logs := kont.RunWriter[string, int](comp)
	if result != 42 {
		t.Fatalf("result = %d, want 42", result)
	}
	if len(logs) != 2 || logs[0] != "log1" || logs[1] != "log2" {
		t.Fatalf("logs = %v, want [log1 log2]", logs)
	}
}

// =============================================================================
// Coverage: RunError — nil result path
// =============================================================================

func TestRunErrorNilResult(t *testing.T) {
	// Create a computation that returns nil to exercise nil path in RunError
	nilComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		// Return nil instead of calling continuation
		return nil
	})

	result := kont.RunError[string, int](nilComp)
	if !result.IsRight() {
		t.Fatal("expected Right with zero value")
	}
	val, _ := result.GetRight()
	if val != 0 {
		t.Fatalf("got %d, want 0", val)
	}
}

// =============================================================================
// Coverage: Bracket acquire failure
// =============================================================================

func TestBracketAcquireFailure(t *testing.T) {
	var released bool

	comp := kont.Bracket[string, int, int](
		kont.ThrowError[string, int]("acquire failed"),
		func(_ int) kont.Eff[struct{}] {
			released = true
			return kont.Pure(struct{}{})
		},
		func(r int) kont.Eff[int] {
			return kont.Pure(r * 2)
		},
	)

	result := kont.RunError[string, kont.Either[string, int]](comp)

	if released {
		t.Fatal("release should not be called when acquire fails")
	}
	// The outer RunError should capture the acquire error
	if result.IsRight() {
		val, _ := result.GetRight()
		if val.IsRight() {
			t.Fatal("expected error from acquire failure")
		}
	}
}

// =============================================================================
// Coverage: evalFrames[reflectProcessor] — non-chained Map/Then/Bind/EffectFrame paths
// =============================================================================

func TestReflectWithMap(t *testing.T) {
	// ExprMap creates a MapFrame. Reflect must handle it in evalFrames[reflectProcessor].
	expr := kont.ExprMap(kont.ExprReturn(21), func(x int) int { return x * 2 })
	cont := kont.Reflect(expr)
	result := kont.Handle(cont, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("no effects expected")
	}))
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestReflectWithThen(t *testing.T) {
	// ExprThen creates a ThenFrame. Reflect must handle it in evalFrames[reflectProcessor].
	expr := kont.ExprThen(kont.ExprReturn("ignored"), kont.ExprReturn(42))
	cont := kont.Reflect(expr)
	result := kont.Handle(cont, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("no effects expected")
	}))
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestReflectWithBind(t *testing.T) {
	// ExprBind creates a BindFrame. Reflect must handle it in evalFrames[reflectProcessor].
	expr := kont.ExprBind(kont.ExprReturn(21), func(x int) kont.Expr[int] {
		return kont.ExprReturn(x * 2)
	})
	cont := kont.Reflect(expr)
	result := kont.Handle(cont, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		panic("no effects expected")
	}))
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestReflectWithEffectFrame(t *testing.T) {
	// ExprPerform creates an EffectFrame. Reflect must handle it in evalFrames[reflectProcessor].
	expr := kont.ExprPerform(kont.Get[int]{})
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](21, cont)
	if result != 21 {
		t.Fatalf("got result %d, want 21", result)
	}
	if state != 21 {
		t.Fatalf("got state %d, want 21", state)
	}
}

func TestReflectMapThenEffect(t *testing.T) {
	// Map(Perform(Get), f) — tests evalFrames[reflectProcessor] non-chained EffectFrame + Map
	expr := kont.ExprMap(
		kont.ExprPerform(kont.Get[int]{}),
		func(s int) string {
			return "val"
		},
	)
	cont := kont.Reflect(expr)
	result, _ := kont.RunState[int, string](42, cont)
	if result != "val" {
		t.Fatalf("got %q, want %q", result, "val")
	}
}

func TestReflectDeepChainedFrames(t *testing.T) {
	// Build a deep chain: Bind(Bind(Bind(Perform(Get), f1), f2), f3)
	// This exercises the nested chainedFrame flattening in evalFrames[reflectProcessor].
	expr := kont.ExprBind(
		kont.ExprBind(
			kont.ExprBind(
				kont.ExprPerform(kont.Get[int]{}),
				func(s int) kont.Expr[int] { return kont.ExprReturn(s + 1) },
			),
			func(s int) kont.Expr[int] { return kont.ExprReturn(s * 2) },
		),
		func(s int) kont.Expr[int] { return kont.ExprReturn(s + 10) },
	)
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](5, cont)
	// (5+1)*2+10 = 22
	if result != 22 {
		t.Fatalf("got %d, want 22", result)
	}
	if state != 5 {
		t.Fatalf("got state %d, want 5", state)
	}
}

func TestReflectChainedMapThenEffect(t *testing.T) {
	// Exercises evalFrames[reflectProcessor] chained branch: Map, Then, Effect inside chained frames
	expr := kont.ExprBind(
		kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(s int) int { return s + 1 }),
		func(s int) kont.Expr[int] {
			return kont.ExprThen(
				kont.ExprPerform(kont.Put[int]{Value: s * 2}),
				kont.ExprPerform(kont.Get[int]{}),
			)
		},
	)
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](10, cont)
	// get=10, map=11, put(22), get=22
	if result != 22 {
		t.Fatalf("got result %d, want 22", result)
	}
	if state != 22 {
		t.Fatalf("got state %d, want 22", state)
	}
}

// =============================================================================
// Coverage: evalFrames[stepProcessor] — chained Map/Then/Effect paths
// =============================================================================

func TestStepExprChainedMapFrame(t *testing.T) {
	// Map inside a Bind chain: exercises chained MapFrame in evalFrames[stepProcessor]
	m := kont.ExprBind(
		kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(s int) int { return s + 1 }),
		func(s int) kont.Expr[int] { return kont.ExprReturn(s * 3) },
	)
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(10)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	// (10+1)*3 = 33
	if result != 33 {
		t.Fatalf("got %d, want 33", result)
	}
}

func TestStepExprChainedThenFrame(t *testing.T) {
	// Then inside a Bind chain: exercises chained ThenFrame in evalFrames[stepProcessor]
	m := kont.ExprBind(
		kont.ExprPerform(kont.Get[int]{}),
		func(s int) kont.Expr[int] {
			return kont.ExprThen(
				kont.ExprReturn("discard"),
				kont.ExprReturn(s*2),
			)
		},
	)
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(21)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

func TestStepExprChainedEffectFrame(t *testing.T) {
	// Effect inside a Bind chain: exercises chained EffectFrame in evalFrames[stepProcessor]
	m := kont.ExprBind(
		kont.ExprPerform(kont.Get[int]{}),
		func(s int) kont.Expr[int] {
			return kont.ExprBind(
				kont.ExprPerform(kont.Put[int]{Value: s + 10}),
				func(_ struct{}) kont.Expr[int] {
					return kont.ExprPerform(kont.Get[int]{})
				},
			)
		},
	)
	state := 5
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected first suspension")
	}
	_, susp = susp.Resume(state) // Get → 5
	if susp == nil {
		t.Fatal("expected second suspension")
	}
	if put, ok := susp.Op().(kont.Put[int]); ok {
		state = put.Value // 15
	}
	_, susp = susp.Resume(struct{}{}) // Put
	if susp == nil {
		t.Fatal("expected third suspension")
	}
	result, susp := susp.Resume(state) // Get → 15
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 15 {
		t.Fatalf("got %d, want 15", result)
	}
}

func TestStepExprChainedReturnFrame(t *testing.T) {
	// Return inside a chain: exercises chained ReturnFrame in evalFrames[stepProcessor]
	m := kont.ExprBind(
		kont.ExprReturn(10),
		func(x int) kont.Expr[int] {
			return kont.ExprBind(
				kont.ExprReturn(x+5),
				func(y int) kont.Expr[int] { return kont.ExprReturn(y * 2) },
			)
		},
	)
	result, susp := kont.StepExpr(m)
	if susp != nil {
		t.Fatal("expected nil suspension for pure chain")
	}
	if result != 30 {
		t.Fatalf("got %d, want 30", result)
	}
}

// =============================================================================
// Coverage: TryResume with Expr path
// =============================================================================

func TestStepExprTryResume(t *testing.T) {
	m := kont.ExprPerform(kont.Get[int]{})
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected suspension")
	}

	result, next, ok := susp.TryResume(42)
	if !ok {
		t.Fatal("expected ok=true on first TryResume")
	}
	if next != nil {
		t.Fatal("expected nil suspension after TryResume")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}

	_, _, ok = susp.TryResume(99)
	if ok {
		t.Fatal("expected ok=false on second TryResume")
	}
}

func TestStepExprTryResumeChained(t *testing.T) {
	// TryResume with Expr path that produces another suspension
	m := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
		return kont.ExprMap(kont.ExprPerform(kont.Put[int]{Value: s + 10}), func(_ struct{}) int { return 0 })
	})
	_, susp := kont.StepExpr(m)
	if susp == nil {
		t.Fatal("expected first suspension")
	}

	_, next, ok := susp.TryResume(5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if next == nil {
		t.Fatal("expected second suspension")
	}
	if _, isPut := next.Op().(kont.Put[int]); !isPut {
		t.Fatalf("expected Put, got %T", next.Op())
	}
}

// =============================================================================
// Coverage: classifyResumed nil path
// =============================================================================

func TestStepNilResult(t *testing.T) {
	// Computation that returns nil Resumed to exercise nil path in classifyResumed
	nilComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		return nil
	})
	result, susp := kont.Step(nilComp)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 0 {
		t.Fatalf("got %d, want 0", result)
	}
}

// =============================================================================
// Coverage: fromResumed nil path
// =============================================================================

func TestReifyNilResumed(t *testing.T) {
	// Computation that returns nil — fromResumed nil branch
	nilComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		return nil
	})
	expr := kont.Reify(nilComp)
	result := kont.RunPure(expr)
	if result != 0 {
		t.Fatalf("got %d, want 0", result)
	}
}

// =============================================================================
// Coverage: StateError Catch success continuation (rightVal path)
// =============================================================================

func TestRunStateErrorCatchSuccess(t *testing.T) {
	// Catch where body succeeds — exercises the rightVal/Resume continuation.
	// Catch body is error-only (like Listen/Censor); State ops outside.
	comp := kont.PutState(11,
		kont.CatchError[string](
			kont.Pure(11),
			func(e string) kont.Eff[int] {
				return kont.Pure(-1) // should not be called
			},
		),
	)

	either, state := kont.RunStateError[int, string, int](10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 11 {
		t.Fatalf("got %d, want 11", v)
	}
	if state != 11 {
		t.Fatalf("got state %d, want 11", state)
	}
}

// =============================================================================
// Coverage: ReaderStateError Catch success continuation (rightVal path)
// =============================================================================

func TestRunReaderStateErrorCatchSuccess(t *testing.T) {
	// Catch where body succeeds — exercises the rightVal/Resume continuation.
	// Catch body is error-only (like Listen/Censor); Reader/State ops outside.
	comp := kont.PutState(10,
		kont.CatchError[string](
			kont.Pure(15),
			func(e string) kont.Eff[int] {
				return kont.Pure(-1) // should not be called
			},
		),
	)

	either, state := kont.RunReaderStateError[int, int, string, int](5, 10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 15 {
		t.Fatalf("got %d, want 15", v)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// =============================================================================
// Coverage: StateHandler/WriterHandler getter paths
// =============================================================================

func TestStateHandlerGetter(t *testing.T) {
	// Exercise the getter closure returned by StateHandler
	h, getState := kont.StateHandler[int, int](42)
	state := getState()
	if state != 42 {
		t.Fatalf("got state %d, want 42", state)
	}
	// Use handler to verify it works
	comp := kont.Perform(kont.Get[int]{})
	result := kont.Handle(comp, h)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
	state = getState()
	if state != 42 {
		t.Fatalf("got state %d, want 42", state)
	}
}

func TestWriterHandlerGetter(t *testing.T) {
	// Exercise the getter closure returned by WriterHandler
	h, getOutput := kont.WriterHandler[string, int]()
	output := getOutput()
	if len(output) != 0 {
		t.Fatalf("got output %v, want empty", output)
	}
	// Use handler to verify it works
	comp := kont.TellWriter("hello", kont.Pure(42))
	result := kont.Handle(comp, h)
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
	output = getOutput()
	if len(output) != 1 || output[0] != "hello" {
		t.Fatalf("got output %v, want [hello]", output)
	}
}

// =============================================================================
// Coverage: RunStateErrorExpr with pure computation
// =============================================================================

func TestRunStateErrorExprPure(t *testing.T) {
	comp := kont.ExprReturn[int](42)
	either, state := kont.RunStateErrorExpr[int, string, int](10, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// =============================================================================
// Coverage: RunReaderStateErrorExpr with Reader dispatch
// =============================================================================

func TestRunReaderStateErrorExprReader(t *testing.T) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[string]{}), func(env string) kont.Expr[string] {
		return kont.ExprReturn(env + "!")
	})
	either, state := kont.RunReaderStateErrorExpr[string, int, string, string]("hello", 0, comp)
	if !either.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := either.GetRight()
	if v != "hello!" {
		t.Fatalf("got %q, want %q", v, "hello!")
	}
	if state != 0 {
		t.Fatalf("got state %d, want 0", state)
	}
}

// =============================================================================
// Coverage: RunErrorExpr with chain
// =============================================================================

func TestRunErrorExprChained(t *testing.T) {
	// Chain that succeeds — exercises the resume path in RunErrorExpr
	comp := kont.ExprBind(kont.ExprReturn(21), func(x int) kont.Expr[int] {
		return kont.ExprReturn(x * 2)
	})
	result := kont.RunErrorExpr[string, int](comp)
	if !result.IsRight() {
		t.Fatal("expected Right")
	}
	v, _ := result.GetRight()
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
}

// =============================================================================
// Coverage: evalFrames[handlerProcessor] — chained ThenFrame path (trampoline.go)
// =============================================================================

func TestRunLoopChainedThenFrame(t *testing.T) {
	// ExprThen(ExprThen(ExprPerform(Get), ExprReturn(10)), ExprReturn(20))
	// After Get dispatches, frame becomes chainedFrame{first: ThenFrame, rest: ThenFrame}
	comp := kont.ExprThen(
		kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprReturn(10)),
		kont.ExprReturn(20),
	)
	result, state := kont.RunStateExpr[int, int](5, comp)
	if result != 20 {
		t.Fatalf("got result %d, want 20", result)
	}
	if state != 5 {
		t.Fatalf("got state %d, want 5", state)
	}
}

// =============================================================================
// Coverage: evalFrames[handlerProcessor] — non-chained EffectFrame short-circuit (!shouldResume)
// =============================================================================

func TestRunLoopNonChainedEffectShortCircuit(t *testing.T) {
	// Bare ExprPerform (non-chained EffectFrame) with handler that short-circuits
	comp := kont.ExprPerform(kont.Get[int]{})
	h := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 99, false // short-circuit
	})
	result := kont.HandleExpr(comp, h)
	if result != 99 {
		t.Fatalf("got %d, want 99", result)
	}
}

// =============================================================================
// Coverage: evalFrames[handlerProcessor] — chained EffectFrame short-circuit (!shouldResume)
// =============================================================================

func TestRunLoopChainedEffectShortCircuit(t *testing.T) {
	// EffectFrame inside a chain where handler short-circuits
	comp := kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(x int) int { return x * 2 })
	h := kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 77, false // short-circuit
	})
	result := kont.HandleExpr(comp, h)
	if result != 77 {
		t.Fatalf("got %d, want 77", result)
	}
}

// =============================================================================
// Coverage: evalFrames[stepProcessor] — chained BindFrame path
// =============================================================================

func TestStepExprChainedBindFrame(t *testing.T) {
	// Nested ExprBind with effect: after resume, hits chained BindFrame path
	comp := kont.ExprBind(
		kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprReturn(s + 10)
		}),
		func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 2)
		},
	)
	_, susp := kont.StepExpr(comp)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(5)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 30 {
		t.Fatalf("got %d, want 30", result)
	}
}

// =============================================================================
// Coverage: evalFrames[stepProcessor] — chained ThenFrame path
// =============================================================================

func TestStepExprChainedThenFramePath(t *testing.T) {
	// Nested ExprThen with effect: after resume, hits chained ThenFrame path
	comp := kont.ExprThen(
		kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprReturn("ignored")),
		kont.ExprReturn(42),
	)
	_, susp := kont.StepExpr(comp)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(0)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	if result != 42 {
		t.Fatalf("got %d, want 42", result)
	}
}

// =============================================================================
// Coverage: evalFrames[reflectProcessor] — chained ThenFrame path (bridge.go)
// =============================================================================

func TestReflectChainedThenFrame(t *testing.T) {
	// Reflect on nested ExprThen+ExprPerform: after effect resume, evalFrames[reflectProcessor]
	// hits chained ThenFrame path
	expr := kont.ExprThen(
		kont.ExprThen(kont.ExprPerform(kont.Get[int]{}), kont.ExprReturn(10)),
		kont.ExprReturn(20),
	)
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](5, cont)
	if result != 20 {
		t.Fatalf("got result %d, want 20", result)
	}
	if state != 5 {
		t.Fatalf("got state %d, want 5", state)
	}
}

// =============================================================================
// Coverage: evalFrames[reflectProcessor] — chained ReturnFrame path (bridge.go)
// =============================================================================

func TestReflectChainedReturnFrame(t *testing.T) {
	// Build a chain where first is ReturnFrame through ChainFrames internal path
	// After processing a BindFrame whose F returns completed Expr, the frame
	// chain may have ReturnFrame as first in a chain.
	expr := kont.ExprBind(
		kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprReturn(s + 1)
		}),
		func(x int) kont.Expr[int] {
			return kont.ExprReturn(x * 2)
		},
	)
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](10, cont)
	if result != 22 {
		t.Fatalf("got result %d, want 22", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// =============================================================================
// Coverage: evalFrames[stepProcessor] — chainedFrame continue after processing (line 154)
// =============================================================================

func TestStepExprTripleMapChainedContinue(t *testing.T) {
	// Triple-nested ExprMap: after effect resume, processing MapFrame in a chain
	// produces another chainedFrame, hitting the continue path in evalFrames[stepProcessor].
	comp := kont.ExprMap(
		kont.ExprMap(
			kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(x int) int { return x + 1 }),
			func(x int) int { return x * 2 },
		),
		func(x int) int { return x + 100 },
	)
	_, susp := kont.StepExpr(comp)
	if susp == nil {
		t.Fatal("expected suspension")
	}
	result, susp := susp.Resume(10)
	if susp != nil {
		t.Fatal("expected nil suspension")
	}
	// (10+1)*2+100 = 122
	if result != 122 {
		t.Fatalf("got %d, want 122", result)
	}
}

// =============================================================================
// Coverage: evalFrames[handlerProcessor] — chained ReturnFrame + chainedFrame continue
// =============================================================================

func TestRunLoopTripleMapChainedContinue(t *testing.T) {
	// Same pattern for evalFrames[handlerProcessor]: triple-nested ExprMap with effect
	comp := kont.ExprMap(
		kont.ExprMap(
			kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(x int) int { return x + 1 }),
			func(x int) int { return x * 2 },
		),
		func(x int) int { return x + 100 },
	)
	result, state := kont.RunStateExpr[int, int](10, comp)
	// (10+1)*2+100 = 122
	if result != 122 {
		t.Fatalf("got result %d, want 122", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// =============================================================================
// Coverage: evalFrames[reflectProcessor] — chainedFrame continue after processing
// =============================================================================

func TestReflectTripleMapChainedContinue(t *testing.T) {
	// Same pattern for evalFrames[reflectProcessor]: triple-nested ExprMap with effect
	expr := kont.ExprMap(
		kont.ExprMap(
			kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), func(x int) int { return x + 1 }),
			func(x int) int { return x * 2 },
		),
		func(x int) int { return x + 100 },
	)
	cont := kont.Reflect(expr)
	result, state := kont.RunState[int, int](10, cont)
	// (10+1)*2+100 = 122
	if result != 122 {
		t.Fatalf("got result %d, want 122", result)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// =============================================================================
// Coverage: Expr runner unhandled-effect panics
// =============================================================================

type exprUnhandledOp struct{}

func (exprUnhandledOp) OpResult() int { panic("phantom") }

func TestRunErrorExprUnhandledPanic(t *testing.T) {
	comp := kont.ExprPerform(exprUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "kont: unhandled effect in ErrorHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_ = kont.RunErrorExpr[string, int](comp)
}

func TestRunStateErrorExprUnhandledPanic(t *testing.T) {
	comp := kont.ExprPerform(exprUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "kont: unhandled effect in StateErrorHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _ = kont.RunStateErrorExpr[int, string, int](0, comp)
}

func TestRunReaderStateErrorExprUnhandledPanic(t *testing.T) {
	comp := kont.ExprPerform(exprUnhandledOp{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if r != "kont: unhandled effect in ReaderStateErrorHandler" {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _ = kont.RunReaderStateErrorExpr[int, int, string, int](0, 0, comp)
}

// =============================================================================
// Coverage: RunStateError nil result path
// =============================================================================

func TestRunStateErrorNilResult(t *testing.T) {
	nilComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		return nil
	})
	either, state := kont.RunStateError[int, string, int](10, nilComp)
	if !either.IsRight() {
		t.Fatal("expected Right with zero value")
	}
	v, _ := either.GetRight()
	if v != 0 {
		t.Fatalf("got %d, want 0", v)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}

// =============================================================================
// Coverage: RunReaderStateError nil result path
// =============================================================================

func TestRunReaderStateErrorNilResult(t *testing.T) {
	nilComp := kont.Suspend[kont.Resumed, int](func(k func(int) kont.Resumed) kont.Resumed {
		return nil
	})
	either, state := kont.RunReaderStateError[string, int, string, int]("env", 10, nilComp)
	if !either.IsRight() {
		t.Fatal("expected Right with zero value")
	}
	v, _ := either.GetRight()
	if v != 0 {
		t.Fatalf("got %d, want 0", v)
	}
	if state != 10 {
		t.Fatalf("got state %d, want 10", state)
	}
}
