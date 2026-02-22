// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync"

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

// chainPool is a global pool for chainedFrame nodes used during evaluation.
// Nodes are acquired in chainFromPool and released after consumption in evalFrames.
var chainPool = sync.Pool{New: func() any { return new(chainedFrame) }}

// chainFromPool links two frame chains, acquiring from the pool.
// Semantics are identical to ChainFrames.
func chainFromPool(first, second Frame) Frame {
	if _, ok := first.(ReturnFrame); ok {
		return second
	}
	if _, ok := second.(ReturnFrame); ok {
		return first
	}
	cf := chainPool.Get().(*chainedFrame)
	cf.first = first
	cf.rest = second
	cf.pooled = true
	return cf
}

// releaseChain returns a pool-acquired chainedFrame to the pool.
// Nodes not acquired from the pool (created by ChainFrames) are left for GC.
func releaseChain(cf *chainedFrame) {
	if !cf.pooled {
		return
	}
	cf.first = nil
	cf.rest = nil
	cf.pooled = false
	chainPool.Put(cf)
}

// evalFrames is the unified F-bounded iterative evaluator for Expr frame chains.
// The processor type P is known at monomorphization time, enabling the compiler to
// devirtualize processEffect/processReturn calls. Three processors:
//   - handlerProcessor[H, R]: dispatches EffectFrame to handler (HandleExpr/RunPure)
//   - stepProcessor[A]: yields Suspension at EffectFrame (StepExpr)
//   - reflectProcessor[A]: emits effectMarker at EffectFrame (Reflect)
//
// Transient chainedFrame nodes are acquired from a sync.Pool and released
// after their fields are extracted, avoiding per-evaluation heap allocation.
func evalFrames[P frameProcessor[P, R], R any](current Erased, frame Frame, p P) R {
	for {
		// Flatten chained frames
		for {
			cf, ok := frame.(*chainedFrame)
			if !ok {
				break
			}
			if nested, ok := cf.first.(*chainedFrame); ok {
				rest := cf.rest
				releaseChain(cf)
				first := nested.first
				nrest := nested.rest
				releaseChain(nested)
				frame = chainFromPool(first, chainFromPool(nrest, rest))
				continue
			}
			switch f := cf.first.(type) {
			case ReturnFrame:
				frame = cf.rest
				releaseChain(cf)
			case *BindFrame[Erased, Erased]:
				next := f.F(current)
				current = Erased(next.Value)
				fNext := f.Next
				rest := cf.rest
				releaseChain(cf)
				releaseBindFrame(f)
				frame = chainFromPool(chainFromPool(next.Frame, fNext), rest)
			case *MapFrame[Erased, Erased]:
				current = f.F(current)
				fNext := f.Next
				rest := cf.rest
				releaseChain(cf)
				frame = chainFromPool(fNext, rest)
			case *ThenFrame[Erased, Erased]:
				current = f.Second.Value
				secondFrame := f.Second.Frame
				fNext := f.Next
				rest := cf.rest
				releaseChain(cf)
				releaseThenFrame(f)
				frame = chainFromPool(chainFromPool(secondFrame, fNext), rest)
			case *EffectFrame[Erased]:
				rest := cf.rest
				releaseChain(cf)
				newCurrent, newFrame, result, ok := p.processEffect(f, chainFromPool(f.Next, rest))
				if !ok {
					return result
				}
				current = newCurrent
				frame = newFrame
			default:
				if u, ok := f.(interface{ Unwind(Erased) (Erased, Frame) }); ok {
					var next Frame
					current, next = u.Unwind(current)
					rest := cf.rest
					releaseChain(cf)
					frame = chainFromPool(next, rest)
					continue
				}
				panic("kont: frame type does not implement Unwind")
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
			fNext := f.Next
			releaseBindFrame(f)
			frame = chainFromPool(next.Frame, fNext)
		case *MapFrame[Erased, Erased]:
			current = f.F(current)
			frame = f.Next
		case *ThenFrame[Erased, Erased]:
			current = f.Second.Value
			secondFrame := f.Second.Frame
			fNext := f.Next
			releaseThenFrame(f)
			frame = chainFromPool(secondFrame, fNext)
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
			panic("kont: frame type does not implement Unwind")
		}
	}
}

// handlerProcessor adapts an F-bounded Handler for use with evalFrames.
// Dispatches EffectFrame operations to the handler and resumes or short-circuits.
type handlerProcessor[H Handler[H, R], R any] struct{ h H }

func (p handlerProcessor[H, R]) processEffect(f *EffectFrame[Erased], rest Frame) (Erased, Frame, R, bool) {
	v, shouldResume := p.h.Dispatch(f.Operation)
	if !shouldResume {
		releaseEffectFrame(f)
		return nil, nil, v.(R), false
	}
	resumed := f.Resume(v)
	releaseEffectFrame(f)
	var zero R
	return resumed, rest, zero, true
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
// The pooled flag tracks whether this node was acquired from chainPool.
type chainedFrame struct {
	first  Frame
	rest   Frame
	pooled bool
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
