// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// pureEval is a sentinel handler for RunPure.
// Its Dispatch method unconditionally panics on any effect operation.
type pureEval[R any] struct{}

func (pureEval[R]) Dispatch(Operation) (Resumed, bool) {
	panic("kont: unhandled effect frame in pure computation - use HandleExpr")
}

// frameProcessor is an F-bounded interface for the three Expr evaluation strategies.
// The type parameter P is the concrete processor (self-referential bound), R is the
// result type. Shared frame iteration lives in evalFrames; processors define only
// the EffectFrame and ReturnFrame handling that diverges between use cases.
type frameProcessor[P frameProcessor[P, R], R any] interface {
	processEffect(f *EffectFrame[Erased], rest Frame) (Erased, Frame, R, bool)
	processReturn(current Erased) R
}

// evalFrames is the unified F-bounded iterative evaluator for Expr frame chains.
// The processor type P is known at monomorphization time, enabling the compiler to
// devirtualize processEffect/processReturn calls. Three processors:
//   - handlerProcessor[H, R]: dispatches EffectFrame to handler (HandleExpr/RunPure)
//   - stepProcessor[A]: yields Suspension at EffectFrame (StepExpr)
//   - reflectProcessor[A]: emits effectMarker at EffectFrame (Reflect)
func evalFrames[P frameProcessor[P, R], R any](current Erased, frame Frame, p P) R {
	for {
		// Flatten chained frames
		for {
			cf, ok := frame.(*chainedFrame)
			if !ok {
				break
			}
			if nested, ok := cf.first.(*chainedFrame); ok {
				frame = &chainedFrame{
					first: nested.first,
					rest:  ChainFrames(nested.rest, cf.rest),
				}
				continue
			}
			switch f := cf.first.(type) {
			case ReturnFrame:
				frame = cf.rest
			case *BindFrame[Erased, Erased]:
				next := f.F(current)
				current = Erased(next.Value)
				frame = ChainFrames(ChainFrames(next.Frame, f.Next), cf.rest)
			case *MapFrame[Erased, Erased]:
				current = f.F(current)
				frame = ChainFrames(f.Next, cf.rest)
			case *ThenFrame[Erased, Erased]:
				current = Erased(f.Second.Value)
				frame = ChainFrames(ChainFrames(f.Second.Frame, f.Next), cf.rest)
			case *EffectFrame[Erased]:
				newCurrent, newFrame, result, ok := p.processEffect(f, cf.rest)
				if !ok {
					return result
				}
				current = newCurrent
				frame = newFrame
			default:
				if u, ok := f.(interface{ Unwind(Erased) (Erased, Frame) }); ok {
					var next Frame
					current, next = u.Unwind(current)
					frame = ChainFrames(next, cf.rest)
					continue
				}
				panic("kont: unknown frame type in chain")
			}
			break
		}
		if _, ok := frame.(*chainedFrame); ok {
			continue
		}

		switch f := frame.(type) {
		case ReturnFrame:
			return p.processReturn(current)
		case *BindFrame[Erased, Erased]:
			next := f.F(current)
			current = Erased(next.Value)
			frame = ChainFrames(next.Frame, f.Next)
		case *MapFrame[Erased, Erased]:
			current = f.F(current)
			frame = f.Next
		case *ThenFrame[Erased, Erased]:
			current = Erased(f.Second.Value)
			frame = ChainFrames(f.Second.Frame, f.Next)
		case *EffectFrame[Erased]:
			newCurrent, newFrame, result, ok := p.processEffect(f, f.Next)
			if !ok {
				return result
			}
			current = newCurrent
			frame = newFrame
		default:
			if u, ok := frame.(interface{ Unwind(Erased) (Erased, Frame) }); ok {
				current, frame = u.Unwind(current)
				continue
			}
			panic("kont: unknown frame type")
		}
	}
}

// handlerProcessor adapts an F-bounded Handler for use with evalFrames.
// Dispatches EffectFrame operations to the handler and resumes or short-circuits.
type handlerProcessor[H Handler[H, R], R any] struct{ h H }

func (p handlerProcessor[H, R]) processEffect(f *EffectFrame[Erased], rest Frame) (Erased, Frame, R, bool) {
	v, shouldResume := p.h.Dispatch(f.Operation)
	if !shouldResume {
		return nil, nil, v.(R), false
	}
	var zero R
	return f.Resume(v), rest, zero, true
}

func (p handlerProcessor[H, R]) processReturn(current Erased) R {
	return current.(R)
}

// HandleExpr evaluates a defunctionalized computation with an effect handler.
// This is the Expr counterpart of [Handle] for closure-based [Cont].
//
// Like [RunPure], it processes frames iteratively without stack growth.
// When encountering an [EffectFrame], it dispatches the operation to the handler.
// The handler returns (resumeValue, true) to continue, or (finalResult, false)
// to short-circuit.
func HandleExpr[H Handler[H, R], R any](m Expr[R], h H) R {
	return evalFrames(Erased(m.Value), m.Frame, handlerProcessor[H, R]{h: h})
}

// ChainFrames links two frame chains together.
// Returns the other operand when either side is ReturnFrame (the identity element
// for frame composition), avoiding unnecessary chainedFrame allocation.
//
// Construction is O(1) in all cases: returns the other operand or creates one chainedFrame node.
func ChainFrames(first, second Frame) Frame {
	if _, ok := first.(ReturnFrame); ok {
		return second
	}
	if _, ok := second.(ReturnFrame); ok {
		return first
	}
	return &chainedFrame{first: first, rest: second}
}

// chainedFrame represents a frame followed by more frames.
// This enables composing frame chains without mutation.
type chainedFrame struct {
	first Frame
	rest  Frame
}

func (*chainedFrame) frame() {}

// RunPure evaluates a pure defunctionalized computation
// to completion. It iteratively processes frames until reaching
// ReturnFrame, avoiding stack growth from recursive calls.
//
// Panics if the computation contains [EffectFrame]. Use [HandleExpr]
// for computations with effects.
func RunPure[A any](c Expr[A]) A {
	return evalFrames(Erased(c.Value), c.Frame, handlerProcessor[pureEval[A], A]{h: pureEval[A]{}})
}

// ExprBind creates a bind frame linking computation m to function f.
func ExprBind[A, B any](m Expr[A], f func(A) Expr[B]) Expr[B] {
	if _, ok := m.Frame.(ReturnFrame); ok {
		// Optimization: if m is already completed, apply f directly
		return f(m.Value)
	}

	// Create a bind frame
	// We need to type-erase here for the generic frame chain
	bindFrame := &BindFrame[Erased, Erased]{
		F: func(a Erased) Expr[Erased] {
			result := f(a.(A))
			return Expr[Erased]{
				Value: Erased(result.Value),
				Frame: result.Frame,
			}
		},
		Next: ReturnFrame{},
	}

	var zero B
	return Expr[B]{
		Value: zero,
		Frame: ChainFrames(m.Frame, bindFrame),
	}
}

// ExprMap creates a map frame transforming computation m with function f.
func ExprMap[A, B any](m Expr[A], f func(A) B) Expr[B] {
	if _, ok := m.Frame.(ReturnFrame); ok {
		// Optimization: if m is already completed, apply f directly
		return ExprReturn(f(m.Value))
	}

	// Create a map frame
	mapFrame := &MapFrame[Erased, Erased]{
		F: func(a Erased) Erased {
			return f(a.(A))
		},
		Next: ReturnFrame{},
	}

	var zero B
	return Expr[B]{
		Value: zero,
		Frame: ChainFrames(m.Frame, mapFrame),
	}
}

// ExprThen creates a then frame sequencing m before n (discarding m's result).
func ExprThen[A, B any](m Expr[A], n Expr[B]) Expr[B] {
	if _, ok := m.Frame.(ReturnFrame); ok {
		// Optimization: if m is already completed, just return n
		return n
	}

	// Create a then frame
	thenFrame := &ThenFrame[Erased, Erased]{
		Second: Expr[Erased]{
			Value: Erased(n.Value),
			Frame: n.Frame,
		},
		Next: ReturnFrame{},
	}

	var zero B
	return Expr[B]{
		Value: zero,
		Frame: ChainFrames(m.Frame, thenFrame),
	}
}
