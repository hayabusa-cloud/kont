// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Reify converts a closure-based effectful computation into a
// defunctionalized frame chain. Closures become tagged data.
//
// The conversion is lazy: each effect step is converted on demand
// as the Expr is evaluated. Pure computations are converted eagerly.
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

// fromResumed converts a Resumed value (which may be an effectSuspension
// or a final value) into an Expr. For effectSuspensions, creates an
// EffectFrame with a lazy BindFrame that converts the next step on demand.
func fromResumed[A any](r Resumed) Expr[A] {
	s, ok := r.(effectSuspension)
	if !ok {
		if r == nil {
			var zero A
			return ExprReturn(zero)
		}
		return ExprReturn(r.(A))
	}
	var zero A
	return Expr[A]{
		Value: zero,
		Frame: &EffectFrame[Erased]{
			Operation: s.Op(),
			Resume:    func(v Erased) Erased { return s.Resume(v) },
			Next: &BindFrame[Erased, Erased]{
				F: func(v Erased) Expr[Erased] {
					e := fromResumed[A](v)
					return Expr[Erased]{Value: Erased(e.Value), Frame: e.Frame}
				},
				Next: ReturnFrame{},
			},
		},
	}
}

// Reflect converts a defunctionalized frame chain back into a
// closure-based effectful computation. Tagged data becomes closures.
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
	return func(k func(A) Resumed) Resumed {
		return evalFrames[reflectProcessor[A], Resumed](
			Erased(m.Value), m.Frame, reflectProcessor[A]{k: k},
		)
	}
}

// reflectProcessor converts EffectFrames to effectMarker suspensions
// and applies the final continuation k at ReturnFrame.
// One closure allocation per EffectFrame for lazy Cont reconstruction.
type reflectProcessor[A any] struct{ k func(A) Resumed }

func (p reflectProcessor[A]) processEffect(f *EffectFrame[Erased], rest Frame) (Erased, Frame, Resumed, bool) {
	capturedF := f
	return nil, nil, effectMarker[Erased]{
		op: capturedF.Operation,
		k: func(v Erased) Resumed {
			return evalFrames[reflectProcessor[A], Resumed](capturedF.Resume(v), rest, p)
		},
	}, false
}

func (p reflectProcessor[A]) processReturn(current Erased) Resumed {
	return p.k(current.(A))
}
