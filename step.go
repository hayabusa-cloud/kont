// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync/atomic"

// Stepping boundary for external runtimes.
// Step/StepExpr provide shallow one-effect-at-a-time evaluation,
// unlike Handle/HandleExpr which run a synchronous trampoline to completion.

// Suspension represents a computation suspended on an effect operation.
// It holds the pending operation and a one-shot resumption handle.
//
// Resume, TryResume and Discard may compete for the same handle.
// Only the winner consumes it. A successful resume can yield a new suspension.
// Share the pointer when passing the handle between callers.
type Suspension[A any] struct {
	used atomic.Bool
	op   Operation
	cont *genericMarker       // Pending CPS operation
	ef   *EffectFrame[Erased] // Optional Expr callback, nil for a primitive perform
	rest Frame                // Remaining Expr frames
}

// Op returns the effect operation that caused the suspension.
func (s *Suspension[A]) Op() Operation { return s.op }

// Resume advances the computation with the given value.
// Returns either a completed value (with nil suspension) or the next suspension.
// Panics if the suspension has already been resumed or discarded.
func (s *Suspension[A]) Resume(v Resumed) (A, *Suspension[A]) {
	if !s.used.CompareAndSwap(false, true) {
		panic("kont: suspension resumed twice")
	}
	return s.resume(v)
}

// TryResume attempts to advance the computation.
// Returns (value, suspension, true) on success, or (zero, nil, false) if already used.
func (s *Suspension[A]) TryResume(v Resumed) (A, *Suspension[A], bool) {
	if !s.used.CompareAndSwap(false, true) {
		var zero A
		return zero, nil, false
	}
	value, next := s.resume(v)
	return value, next, true
}

// resume owns the payload after a successful claim.
func (s *Suspension[A]) resume(v Resumed) (A, *Suspension[A]) {
	if m := s.cont; m != nil {
		s.cont = nil
		return classifyResumed[A](m.Resume(v))
	}
	f, rest := s.ef, s.rest
	s.ef, s.rest = nil, nil
	if f != nil {
		v = resumeEffectFrame(f, v)
	}
	if _, done := rest.(ReturnFrame); done {
		return valueOrZero[A](v), nil
	}
	return stepFrames[A](v, rest)
}

// Discard consumes a pending suspension without resuming it.
// Calling Discard on a consumed suspension has no effect.
func (s *Suspension[A]) Discard() {
	if !s.used.CompareAndSwap(false, true) {
		return
	}
	if m := s.cont; m != nil {
		s.cont = nil
		discardMarker(m)
		return
	}
	f, rest := s.ef, s.rest
	s.ef, s.rest = nil, nil
	if f != nil {
		releaseEffectFrame(f)
	}
	releaseFrameChain(rest)
}

// Step drives a Cont[Resumed, A] computation until it either completes or
// suspends on an effect operation.
// Returns (value, nil) if the computation completed, or (zero, suspension) if pending.
//
// Example:
//
//	result, susp := Step(computation)
//	for susp != nil {
//	    v := handleOp(susp.Op())
//	    result, susp = susp.Resume(v)
//	}
func Step[A any](m Cont[Resumed, A]) (A, *Suspension[A]) {
	result := m(toResumed[A])
	return classifyResumed[A](result)
}

// classifyResumed examines a Resumed value and classifies it as either
// a completed value or a suspension carrying the continuation state.
func classifyResumed[A any](result Resumed) (A, *Suspension[A]) {
	if s, ok := result.(*genericMarker); ok {
		var zero A
		return zero, &Suspension[A]{
			op:   s.Op(),
			cont: s,
		}
	}
	if result == nil {
		var zero A
		return zero, nil
	}
	return result.(A), nil
}

// StepExpr drives an Expr[A] computation until it either completes or
// suspends on an effect operation.
// Returns (value, nil) if the computation completed, or (zero, suspension) if pending.
func StepExpr[A any](m Expr[A]) (A, *Suspension[A]) {
	switch f := m.Frame.(type) {
	case ReturnFrame:
		return m.Value, nil
	case *chainedFrame:
		// A composed expression enters the frame evaluator below.
	case performFrame:
		var zero A
		return zero, &Suspension[A]{op: f.operation(), rest: ReturnFrame{}}
	case *EffectFrame[Erased]:
		var zero A
		return zero, &Suspension[A]{op: f.Operation, ef: f, rest: f.Next}
	case *capturedCPS[A]:
		marker := f.take()
		var zero A
		return zero, &Suspension[A]{op: marker.op, cont: marker}
	}
	return stepFrames[A](m.initialValue(), m.Frame)
}

// stepResult keeps a completed value distinct from the evaluator's control state.
type stepResult[A any] struct {
	value Erased
	next  *Suspension[A]
}

func stepFrames[A any](current Erased, frame Frame) (A, *Suspension[A]) {
	r := evalFrames(current, frame, stepProcessor[A]{})
	return valueOrZero[A](r.value), r.next
}

type stepProcessor[A any] struct{}

func (stepProcessor[A]) processEffect(op Operation, f *EffectFrame[Erased], rest Frame) (Erased, Frame, stepResult[A], bool) {
	return nil, nil, stepResult[A]{next: &Suspension[A]{op: op, ef: f, rest: rest}}, false
}

func (stepProcessor[A]) processReturn(current Erased) stepResult[A] {
	return stepResult[A]{value: current}
}
