// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// unhandledEffect panics with a descriptive message for unmatched operations.
// Extracted as a noinline function so that Dispatch methods remain inlineable.
//
//go:noinline
func unhandledEffect(handler string) {
	panic("kont: unhandled effect in " + handler)
}

// Operation is the interface for effect operations in handler dispatch.
// All values passed as the op parameter to Handler.Dispatch implement this interface.
type Operation any

// Resumed is the interface for values flowing through effect suspension and resumption.
// Effectful computations use Cont[Resumed, A] as their continuation type.
// Handler resume callbacks accept and return Resumed.
type Resumed any

// Op is the F-bounded interface for effect operations.
// Each effect defines concrete types implementing Op with the appropriate
// result type parameter. The self-referencing constraint gives the compiler
// knowledge of both the concrete operation type and its result type.
//
// Example:
//
//	type Ask[E any] struct{ kont.Phantom[E] }
type Op[O Op[O, A], A any] interface {
	OpResult() A // phantom type marker for result
}

// Phantom is an embeddable zero-size type that provides the [Op] result marker.
// Embed Phantom[A] in an operation struct to satisfy [Op] without writing
// a manual OpResult method.
//
// Example:
//
//	type Ask[E any] struct{ kont.Phantom[E] }
//	// Ask[E] satisfies Op[Ask[E], E] via promoted OpResult() E
type Phantom[A any] struct{}

// OpResult implements the phantom type marker for [Op].
func (Phantom[A]) OpResult() A { panic("phantom") }

// Handler is the F-bounded interface for effect handlers.
// The self-referencing constraint H Handler[H, R] gives the compiler
// knowledge of the concrete handler type at compile time.
//
// The Dispatch method returns (resumeValue, true) to continue the computation,
// or (finalResult, false) to short-circuit and return immediately.
type Handler[H Handler[H, R], R any] interface {
	Dispatch(op Operation) (Resumed, bool)
}

// handlerFunc wraps a dispatch function as a concrete Handler.
// Returns (resumeValue, true) to continue, or (finalResult, false) to short-circuit.
type handlerFunc[R any] struct {
	f func(op Operation) (Resumed, bool)
}

func (h *handlerFunc[R]) Dispatch(op Operation) (Resumed, bool) {
	return h.f(op)
}

// HandleFunc creates a handler from a dispatch function.
// The function receives each effect operation and returns (resumeValue, true)
// to continue the computation, or (finalResult, false) to short-circuit.
//
// Example:
//
//	HandleFunc[int](func(op Operation) (Resumed, bool) {
//	    switch e := op.(type) {
//	    case Ask[int]:
//	        return 42, true  // resume with value
//	    case Tell[int]:
//	        fmt.Println(e.Value)
//	        return struct{}{}, true
//	    default:
//	        panic("unhandled effect")
//	    }
//	})
func HandleFunc[R any](f func(op Operation) (Resumed, bool)) *handlerFunc[R] {
	return &handlerFunc[R]{f: f}
}

// effectSuspension represents a suspended effect operation.
// Single interface dispatch covers all marker types (effectMarker, bindMarker,
// thenMarker, mapMarker). Same allocation profile (1 heap alloc per Perform).
type effectSuspension interface {
	Op() Operation
	Resume(Resumed) Resumed
}

// effectMarker represents a suspended effect operation.
// Uses a typed continuation to avoid closure allocation in Perform.
type effectMarker[A any] struct {
	op Operation
	k  func(A) Resumed
}

// Op returns the operation for effectSuspension interface.
func (m effectMarker[A]) Op() Operation { return m.op }

// Resume invokes the typed continuation with proper type assertion.
func (m effectMarker[A]) Resume(v Resumed) Resumed {
	return m.k(v.(A))
}

// bindMarker fuses Bind(Perform(op), f) into a single suspension.
// Resume applies f to the dispatched value, then passes k.
type bindMarker[A, B any] struct {
	op Operation
	f  func(A) Cont[Resumed, B]
	k  func(B) Resumed
}

func (m bindMarker[A, B]) Op() Operation            { return m.op }
func (m bindMarker[A, B]) Resume(v Resumed) Resumed { return m.f(v.(A))(m.k) }

// thenMarker fuses Then(Perform(op), next) into a single suspension.
// Resume ignores the dispatch value and runs next with k.
type thenMarker[B any] struct {
	op   Operation
	next Cont[Resumed, B]
	k    func(B) Resumed
}

func (m thenMarker[B]) Op() Operation            { return m.op }
func (m thenMarker[B]) Resume(_ Resumed) Resumed { return m.next(m.k) }

// mapMarker fuses Map(Perform(op), f) into a single suspension.
// Resume applies the pure function f, then passes to k.
type mapMarker[A, B any] struct {
	op Operation
	f  func(A) B
	k  func(B) Resumed
}

func (m mapMarker[A, B]) Op() Operation            { return m.op }
func (m mapMarker[A, B]) Resume(v Resumed) Resumed { return m.k(m.f(v.(A))) }

// Perform triggers an effect operation within a computation.
// The computation suspends until a handler resumes it with a result value.
// Zero-allocation: stores the typed continuation directly without closure wrapper.
//
// Type inference handles calls: Perform(Get[int]{}) infers O=Get[int], A=int.
//
// Example:
//
//	func ask[E any]() Eff[E] {
//	    return Perform(Ask[E]{})
//	}
func Perform[O Op[O, A], A any](op O) Cont[Resumed, A] {
	return func(k func(A) Resumed) Resumed {
		return effectMarker[A]{op: op, k: k}
	}
}

// identityResume is the identity resume function for ExprPerform and ExprThrowError.
// Named function produces a static function value, consistent with toResumed[A] and identity[A].
func identityResume(v Erased) Erased { return v }

// toResumed is the identity continuation for CPS entry points (Handle, Step,
// Reify). Named generic function produces a static function value per type
// instantiation, avoiding the heap allocation that anonymous closures incur.
func toResumed[A any](a A) Resumed { return a }

// ExprPerform creates a defunctionalized computation that performs an effect operation.
// This is the Expr counterpart of [Perform] for closure-based [Cont].
//
// The computation suspends at an [EffectFrame] carrying the operation.
// Use [HandleExpr] to evaluate computations containing effect frames.
//
// Type inference handles calls: ExprPerform(Get[int]{}) infers O=Get[int], A=int.
func ExprPerform[O Op[O, A], A any](op O) Expr[A] {
	var zero A
	return Expr[A]{
		Value: zero,
		Frame: &EffectFrame[Erased]{
			Operation: op,
			Resume:    identityResume,
			Next:      ReturnFrame{},
		},
	}
}

// Handle runs a computation with an F-bounded effect handler.
// The handler intercepts effect operations and determines how to resume.
//
// Example:
//
//	result := Handle(computation, HandleFunc[int](func(op Operation) (Resumed, bool) {
//	    switch op.(type) {
//	    case Ask[int]:
//	        return 42, true
//	    default:
//	        panic("unhandled effect")
//	    }
//	}))
func Handle[H Handler[H, R], R any](m Cont[Resumed, R], h H) R {
	result := m(toResumed[R])
	return handleDispatch[H, R](result, h)
}

// handleDispatch is the zero-allocation trampoline loop.
// Uses single effectSuspension interface dispatch to resume or short-circuit.
func handleDispatch[H Handler[H, R], R any](result Resumed, h H) R {
	for {
		if s, ok := result.(effectSuspension); ok {
			v, shouldResume := h.Dispatch(s.Op())
			if !shouldResume {
				return v.(R)
			}
			result = s.Resume(v)
			continue
		}
		// Final value - return it
		if result == nil {
			var zero R
			return zero
		}
		return result.(R)
	}
}
