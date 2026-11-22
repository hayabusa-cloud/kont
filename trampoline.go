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
	processEffect(op Operation, f *EffectFrame[Erased], rest Frame) (Erased, Frame, R, bool)
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

// evalFrames runs frame followed by rest. A chained frame moves its right
// operand onto rest. Only an effect yield transfers the complete suffix to a
// processor. Transient chain nodes are recycled as they are visited.
func evalFrames[P frameProcessor[P, R], R any](current Erased, frame Frame, p P) R {
	var rest Frame = ReturnFrame{}
	for {
		var op Operation
		var ef *EffectFrame[Erased]
		switch f := frame.(type) {
		case ReturnFrame:
			if _, done := rest.(ReturnFrame); done {
				return p.processReturn(current)
			}
			frame, rest = rest, ReturnFrame{}
			continue
		case *chainedFrame:
			first, tail := f.first, f.rest
			releaseChain(f)
			frame, rest = first, chainFromPool(tail, rest)
			continue
		case *BindFrame[Erased, Erased]:
			next := f.F(current)
			tail := f.Next
			releaseBindFrame(f)
			current, frame, rest = next.Value, next.Frame, chainFromPool(tail, rest)
			continue
		case *MapFrame[Erased, Erased]:
			current = f.F(current)
			frame = f.Next
			continue
		case *ThenFrame[Erased, Erased]:
			second, tail := f.Second, f.Next
			releaseThenFrame(f)
			current, frame, rest = second.Value, second.Frame, chainFromPool(tail, rest)
			continue
		case *UnwindFrame:
			current, frame = f.Unwind(f.Data1, f.Data2, f.Data3, current)
			releaseUnwindFrame(f)
			continue
		case *EffectFrame[Erased]:
			op, ef, rest = f.Operation, f, chainFromPool(f.Next, rest)
		case mapRunner:
			current, frame = f.runMaps(current, rest)
			rest = ReturnFrame{}
			continue
		case interface{ Unwind(Erased) (Erased, Frame) }:
			current, frame = f.Unwind(current)
			continue
		case effectInstruction:
			op, rest = f.splitEffect(rest)
		default:
			panic("kont: frame type does not implement Unwind")
		}
		next, tail, result, ok := p.processEffect(op, ef, rest)
		if !ok {
			return result
		}
		current, frame, rest = next, tail, ReturnFrame{}
	}
}

// handlerProcessor adapts an F-bounded Handler for use with evalFrames.
// Dispatches EffectFrame operations to the handler and resumes or short-circuits.
// Nil interface results are normalized only on the terminal exit paths; the
// inner resume path stays branch-free.
type handlerProcessor[H Handler[H, R], R any] struct{ h H }

func (p handlerProcessor[H, R]) processEffect(op Operation, f *EffectFrame[Erased], rest Frame) (Erased, Frame, R, bool) {
	v, shouldResume := p.h.Dispatch(op)
	if !shouldResume {
		if f != nil {
			releaseEffectFrame(f)
		}
		releaseFrameChain(rest)
		return nil, nil, valueOrZero[R](v), false
	}
	if f != nil {
		v = resumeEffectFrame(f, v)
	}
	var zero R
	return v, rest, zero, true
}

func (p handlerProcessor[H, R]) processReturn(current Erased) R {
	return valueOrZero[R](current)
}

// HandleExpr evaluates a defunctionalized computation with an effect handler.
// This is the Expr counterpart of [Handle] for closure-based [Cont].
//
// Like [RunPure], it follows built-in frame links iteratively.
// A primitive perform or an explicit [EffectFrame] dispatches to the handler.
// The handler returns (resumeValue, true) to continue, or (finalResult, false)
// to short-circuit. HandleExpr releases pooled frames along the abandoned
// continuation's built-in links without invoking its callbacks.
func HandleExpr[H Handler[H, R], R any](m Expr[R], h H) R {
	switch f := m.Frame.(type) {
	case ReturnFrame:
		return m.Value
	case performFrame:
		v, _ := h.Dispatch(f.operation())
		return valueOrZero[R](v)
	}
	return handleExprFrames(m, h)
}

// A root capture can transfer its entire residual to the CPS handler.
// Check for it on the path that already needs the frame evaluator.
func handleExprFrames[H Handler[H, R], R any](m Expr[R], h H) R {
	if f, ok := m.Frame.(*capturedCPS[R]); ok {
		result, _ := handleDispatch[H, R](f.take(), h)
		return valueOrZero[R](result)
	}
	return evalFrames(m.initialValue(), m.Frame, handlerProcessor[H, R]{h: h})
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
	return &chainedFrame{first: first, rest: second, ignoresInput: frameIgnoresInput(first)}
}

// chainedFrame represents a frame followed by more frames.
// This enables composing frame chains without mutation.
// The pooled flag tracks whether this node was acquired from chainPool.
type chainedFrame struct {
	first        Frame
	rest         Frame
	pooled       bool
	ignoresInput bool // source links can omit an unused initial value
	callbackTail bool // the persistent tail is a map or bind function with no frame children
}

func (*chainedFrame) frame(frameSignature) {}

// RunPure evaluates a pure defunctionalized computation
// to completion. It follows built-in frame links iteratively until it
// reaches ReturnFrame.
//
// Panics on an effect operation. Use [HandleExpr]
// for computations with effects.
func RunPure[A any](c Expr[A]) A {
	if _, done := c.Frame.(ReturnFrame); done {
		return c.Value
	}
	return evalFrames(c.initialValue(), c.Frame, handlerProcessor[pureEval[A], A]{h: pureEval[A]{}})
}

// frameIgnoresInput reports whether the first instruction supplies its own value.
func frameIgnoresInput(frame Frame) bool {
	switch f := frame.(type) {
	case *EffectFrame[Erased], *ThenFrame[Erased, Erased], performFrame:
		return true
	case *chainedFrame:
		return f.ignoresInput
	default:
		return false
	}
}

func (m Expr[A]) initialValue() Erased {
	if frameIgnoresInput(m.Frame) {
		return nil
	}
	return m.Value
}

// appendExpr preserves the first frame's input even when A and B differ.
func appendExpr[A, B any](m Expr[A], next Frame, callbackTail bool) Expr[B] {
	if frameIgnoresInput(m.Frame) {
		return ExprSuspend[B](&chainedFrame{first: m.Frame, rest: next, ignoresInput: true, callbackTail: callbackTail})
	}
	first := Expr[Erased]{Value: m.Value, Frame: m.Frame}
	return ExprSuspend[B](&ThenFrame[Erased, Erased]{Second: first, Next: next})
}

// With no explicit Next, the callback itself is a reusable frame.
type bindFunc[A, B any] func(A) Expr[B]

func (bindFunc[A, B]) frame(frameSignature) {}

func (f bindFunc[A, B]) Unwind(v Erased) (Erased, Frame) {
	next := f(operandValue[A](v))
	return next.Value, next.Frame
}

type mapFunc[A, B any] func(A) B

func (mapFunc[A, B]) frame(frameSignature) {}

// A run of maps that keep the same result type can pass values directly.
type mapRunner interface {
	runMaps(Erased, Frame) (Erased, Frame)
}

func (f mapFunc[A, B]) runMaps(v Erased, rest Frame) (Erased, Frame) {
	value := f(operandValue[A](v))
	for {
		switch tail := rest.(type) {
		case *chainedFrame:
			next, ok := tail.first.(mapFunc[B, B])
			if !ok {
				return value, rest
			}
			value = next(value)
			rest = tail.rest
			releaseChain(tail)
		case mapFunc[B, B]:
			return tail(value), ReturnFrame{}
		default:
			return value, rest
		}
	}
}

// ExprBind links computation m to function f.
func ExprBind[A, B any](m Expr[A], f func(A) Expr[B]) Expr[B] {
	if _, ok := m.Frame.(ReturnFrame); ok {
		return f(m.Value)
	}
	return appendExpr[A, B](m, bindFunc[A, B](f), true)
}

// ExprMap transforms the result of computation m with f.
func ExprMap[A, B any](m Expr[A], f func(A) B) Expr[B] {
	if _, ok := m.Frame.(ReturnFrame); ok {
		return ExprReturn(f(m.Value))
	}
	return appendExpr[A, B](m, mapFunc[A, B](f), true)
}

// ExprThen creates a then frame sequencing m before n (discarding m's result).
func ExprThen[A, B any](m Expr[A], n Expr[B]) Expr[B] {
	if _, ok := m.Frame.(ReturnFrame); ok {
		// Optimization: if m is already completed, just return n
		return n
	}

	if frameIgnoresInput(n.Frame) {
		return appendExpr[A, B](m, n.Frame, false)
	}

	thenFrame := &ThenFrame[Erased, Erased]{
		Second: Expr[Erased]{
			Value: Erased(n.Value),
			Frame: n.Frame,
		},
		Next: ReturnFrame{},
	}

	return appendExpr[A, B](m, thenFrame, false)
}
