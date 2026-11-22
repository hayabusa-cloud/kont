// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync/atomic"

// Reify converts a closure-based computation to an [Expr].
//
// Reify runs m to its first operation or return. A completed result is
// reusable. If m yields an operation, the Expr and all its copies share one
// evaluation. Each reply advances the captured invocation to its next
// operation or return. Call Reify again on a reusable Cont to start a new run.
// [StepExpr] consumes a pending result before exposing its operation through
// [Suspension.Op]. Stepping that result again panics. Use [Step] directly on m
// when you only need the first value or suspension.
//
// The name follows Filinski (1994): reify converts a semantic value
// (functional Cont) into its syntactic representation (data Expr).
//
// Example:
//
//	cont := GetState(func(s int) Eff[int] {
//	    return Pure(s * 2)
//	})
//	expr := Reify(cont)
//	result, state := RunStateExpr[int, int](0, expr)
func Reify[A any](m Cont[Resumed, A]) Expr[A] {
	result := m(toResumed[A])
	return fromResumed[A](result)
}

// A public Expr can be copied. Transfer the captured marker once before its
// operation becomes observable, so a stale Expr cannot reach a recycled marker.
type capturedCPS[A any] struct{ marker atomic.Pointer[genericMarker] }

func (*capturedCPS[A]) frame(frameSignature) {}

func (f *capturedCPS[A]) take() *genericMarker {
	m := f.marker.Swap(nil)
	if m == nil {
		panic("kont: reified computation already consumed")
	}
	return m
}

func (f *capturedCPS[A]) splitEffect(rest Frame) (Operation, Frame) {
	return cpsEffect[A]{f.take()}.splitEffect(rest)
}

func (f *capturedCPS[A]) discardFrame() Frame {
	if m := f.marker.Swap(nil); m != nil {
		return takeMarkerSuffix(m)
	}
	return nil
}

func fromResumed[A any](r Resumed) Expr[A] {
	if m, ok := r.(*genericMarker); ok {
		f := new(capturedCPS[A])
		f.marker.Store(m)
		return ExprSuspend[A](f)
	}
	return ExprReturn(valueOrZero[A](r))
}

// Successors stay inside the evaluator and already have a unique owner.
type cpsEffect[A any] struct{ m *genericMarker }

func (cpsEffect[A]) frame(frameSignature) {}
func (f cpsEffect[A]) splitEffect(rest Frame) (Operation, Frame) {
	return f.m.op, chainFromPool(cpsReply[A]{f.m}, rest)
}

type cpsReply[A any] struct{ m *genericMarker }

func (cpsReply[A]) frame(frameSignature)  {}
func (f cpsReply[A]) discardFrame() Frame { return takeMarkerSuffix(f.m) }
func (f cpsReply[A]) Unwind(v Erased) (Erased, Frame) {
	r := f.m.Resume(v)
	if next, ok := r.(*genericMarker); ok {
		return nil, cpsEffect[A]{next}
	}
	if r == nil {
		var zero A
		return zero, ReturnFrame{}
	}
	// Keep the existing box when passing the checked answer to an outer frame.
	_ = r.(A)
	return r, ReturnFrame{}
}

// Reflect converts an [Expr] to a closure-based computation. The returned
// computation may be run again whenever its input Expr is reusable.
//
// The resulting Cont[Resumed, A] can be used with Handle, RunState,
// RunReader, and all other Cont-world runners.
//
// The name follows Filinski (1994): reflect converts a syntactic
// representation (data Expr) into a semantic value (functional Cont).
//
// Example:
//
//	expr := ExprBind(ExprPerform(Get[int]{}), func(s int) Expr[int] {
//	    return ExprReturn(s * 2)
//	})
//	cont := Reflect(expr)
//	result, state := RunState[int, int](0, cont)
func Reflect[A any](m Expr[A]) Cont[Resumed, A] {
	if _, done := m.Frame.(ReturnFrame); done {
		return Return[Resumed](m.Value)
	}
	return func(k func(A) Resumed) Resumed {
		return evalFrames(m.initialValue(), m.Frame, reflectProcessor{reflectReturn[A](k)})
	}
}

// The concrete function retains the answer type. Its interface lets all
// reflected operations share one non-generic marker resumption function.
type reflectReturn[A any] func(A) Resumed

func (k reflectReturn[A]) processReturn(current Erased) Resumed {
	return k(valueOrZero[A](current))
}

type exprReturn interface{ processReturn(Erased) Resumed }

type reflectProcessor struct{ exprReturn }

func (p reflectProcessor) processEffect(op Operation, f *EffectFrame[Erased], rest Frame) (Erased, Frame, Resumed, bool) {
	if f != nil {
		rest = chainFromPool(reflectedCallback{f}, rest)
	}
	m := acquireMarker()
	m.op, m.f, m.k, m.resume = op, rest, p.exprReturn, resumeReflected
	return nil, nil, m, false
}

func resumeReflected(m *genericMarker, v Resumed) Resumed {
	rest := m.f.(Frame)
	p := reflectProcessor{m.k.(exprReturn)}
	releaseMarker(m)
	if _, done := rest.(ReturnFrame); done {
		return p.processReturn(v)
	}
	return evalFrames(Erased(v), rest, p)
}

// The evaluator already put f.Next in the suffix. This instruction owns only
// the current effect frame and never dispatches the same operation again.
type reflectedCallback struct{ f *EffectFrame[Erased] }

func (reflectedCallback) frame(frameSignature) {}
func (c reflectedCallback) discardFrame() Frame {
	releaseEffectFrame(c.f)
	return nil
}
func (c reflectedCallback) Unwind(v Erased) (Erased, Frame) {
	return resumeEffectFrame(c.f, v), ReturnFrame{}
}
