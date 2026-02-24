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
// Suspension enforces affine semantics: Resume may be called at most once.
// Calling Resume twice panics. Use Discard to explicitly abandon a suspension.
type Suspension[A any] struct {
	used atomic.Uintptr
	op   Operation
	cont effectSuspension     // Cont path: resume via classifyResumed(cont.Resume(v))
	ef   *EffectFrame[Erased] // Expr path: resume via evalFrames[stepProcessor](ef.Resume(v), rest)
	rest Frame                // Expr path: remaining frames after ef
}

// Op returns the effect operation that caused the suspension.
func (s *Suspension[A]) Op() Operation { return s.op }

// Resume advances the computation with the given value.
// Returns either a completed value (with nil suspension) or the next suspension.
// Panics if the suspension has already been resumed or discarded.
//
// On the Expr path, the returned suspension reuses the receiver's memory
// when possible, avoiding one allocation per step.
func (s *Suspension[A]) Resume(v Resumed) (A, *Suspension[A]) {
	if s.used.Add(1) != 1 {
		panic("kont: suspension resumed twice")
	}
	if s.cont != nil {
		return classifyResumed[A](s.cont.Resume(v))
	}
	return classifyStepResult[A](
		evalFrames[stepProcessor[A], Erased](s.ef.Resume(v), s.rest, stepProcessor[A]{reuse: s}),
	)
}

// TryResume attempts to advance the computation.
// Returns (value, suspension, true) on success, or (zero, nil, false) if already used.
func (s *Suspension[A]) TryResume(v Resumed) (A, *Suspension[A], bool) {
	if s.used.Add(1) != 1 {
		var zero A
		return zero, nil, false
	}
	if s.cont != nil {
		a, next := classifyResumed[A](s.cont.Resume(v))
		return a, next, true
	}
	a, next := classifyStepResult[A](
		evalFrames[stepProcessor[A], Erased](s.ef.Resume(v), s.rest, stepProcessor[A]{reuse: s}),
	)
	return a, next, true
}

// Discard marks the suspension as consumed without resuming.
func (s *Suspension[A]) Discard() {
	s.used.Store(1)
	if s.cont != nil {
		s.cont.release()
	}
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
	if s, ok := result.(effectSuspension); ok {
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
	return classifyStepResult[A](
		evalFrames[stepProcessor[A], Erased](Erased(m.Value), m.Frame, stepProcessor[A]{}),
	)
}

// stepProcessor yields at EffectFrame instead of dispatching.
// Returns *Suspension[A] or the final value via Erased.
// Reuses the previous Suspension when available, saving one allocation per step.
type stepProcessor[A any] struct {
	reuse *Suspension[A]
}

func (p stepProcessor[A]) processEffect(f *EffectFrame[Erased], rest Frame) (Erased, Frame, Erased, bool) {
	s := p.reuse
	if s == nil {
		s = &Suspension[A]{}
	} else {
		s.used.Store(0)
		s.cont = nil
		releaseEffectFrame(s.ef)
	}
	s.op = f.Operation
	s.ef = f
	s.rest = rest
	return nil, nil, s, false
}

func (stepProcessor[A]) processReturn(current Erased) Erased {
	return current
}

// classifyStepResult unpacks the Erased result from evalFrames[stepProcessor]:
// *Suspension[A] → suspended; otherwise → completed value.
func classifyStepResult[A any](result Erased) (A, *Suspension[A]) {
	if susp, ok := result.(*Suspension[A]); ok {
		var zero A
		return zero, susp
	}
	return result.(A), nil
}
