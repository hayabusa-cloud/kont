// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Erased represents a type-erased value in the defunctionalized frame chain.
// Frame types use Erased parameters to process heterogeneous value types
// through a homogeneous evaluation pipeline. Concrete types are recovered
// via type assertions at frame boundaries.
type Erased = any

// valueOrZero recovers a concrete value from an erased/interface carrier.
// Nil interface payloads decode to the zero value of T, preserving kont's
// nil-as-zero convention on completion and short-circuit exit paths.
func valueOrZero[T any](v any) T {
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

// An erased nil interface carries no dynamic type for a Go type assertion.
// Recover it when the expected operand is itself an interface.
func operandValue[T any](v Erased) T {
	if v == nil {
		var zero T
		if any(zero) == nil {
			return zero
		}
	}
	return v.(T)
}

// The marker admits a raw map, bind or sequencing frame when its operand
// and result are both erased. Other instantiations enter through AsFrame.
type frameSignature = func(Erased) Erased

// Frame is the interface for defunctionalized continuation frames.
// Implementations carry the data needed to continue computation.
// A custom frame can embed ReturnFrame and implement Unwind(Erased) (Erased, Frame).
// Use [MapFrame.AsFrame], [BindFrame.AsFrame], or [ThenFrame.AsFrame] for a
// descriptor whose operand or result type differs from [Erased].
type Frame interface {
	frame(frameSignature)
}

// ReturnFrame signals computation completion.
// The evaluator returns the current value as the final result.
type ReturnFrame struct{}

func (ReturnFrame) frame(frameSignature) { return }

// Erased descriptors keep their concrete identity, including pool ownership.
// A typed view contains only the original descriptor pointer.
func asFrame(raw any, typed Frame) Frame {
	if f, ok := raw.(Frame); ok {
		return f
	}
	return typed
}

// BindFrame represents monadic bind: Bind(m, f)
// Type parameters:
//   - A: input type (value from previous computation)
//   - B: output type (result of applying F)
type BindFrame[A, B any] struct {
	// F is the continuation function to apply to the input value.
	F func(A) Expr[B]

	// Next is the continuation frame after F completes.
	Next Frame

	pooled bool
}

// Unwind performs a single step of reduction for the BindFrame.
func (f *BindFrame[A, B]) Unwind(current Erased) (Erased, Frame) {
	next := f.F(operandValue[A](current))
	return Erased(next.Value), ChainFrames(next.Frame, f.Next)
}

// AsFrame returns a frame backed by f. The continuation owns Next.
func (f *BindFrame[A, B]) AsFrame() Frame { return asFrame(f, typedBindFrame[A, B]{f}) }

func (*BindFrame[A, B]) frame(func(A) B) {}

type typedBindFrame[A, B any] struct{ *BindFrame[A, B] }

func (typedBindFrame[A, B]) frame(frameSignature) {}

// MapFrame represents functor mapping: Map(m, f)
// Type parameters:
//   - A: input type (value to transform)
//   - B: output type (result of transformation)
type MapFrame[A, B any] struct {
	// F is the transformation function.
	F func(A) B

	// Next is the continuation frame after transformation.
	Next Frame
}

// Unwind performs a single step of reduction for the MapFrame.
func (f *MapFrame[A, B]) Unwind(current Erased) (Erased, Frame) {
	return Erased(f.F(operandValue[A](current))), f.Next
}

// AsFrame returns a frame backed by f. The continuation owns Next.
func (f *MapFrame[A, B]) AsFrame() Frame { return asFrame(f, typedMapFrame[A, B]{f}) }

func (*MapFrame[A, B]) frame(func(A) B) {}

type typedMapFrame[A, B any] struct{ *MapFrame[A, B] }

func (typedMapFrame[A, B]) frame(frameSignature) {}

// ThenFrame represents sequencing with discard: Then(m, n)
// Type parameters:
//   - A: discarded type (result of first computation, unused)
//   - B: output type (result of second computation)
type ThenFrame[A, B any] struct {
	// Second is the computation to evaluate after discarding first result.
	Second Expr[B]

	// Next is the continuation frame after Second completes.
	Next Frame

	pooled bool
}

// Unwind performs a single step of reduction for the ThenFrame.
func (f *ThenFrame[A, B]) Unwind(current Erased) (Erased, Frame) {
	return Erased(f.Second.Value), ChainFrames(f.Second.Frame, f.Next)
}

// AsFrame returns a frame backed by f. The continuation owns Second.Frame and Next.
func (f *ThenFrame[A, B]) AsFrame() Frame { return asFrame(f, typedThenFrame[A, B]{f}) }

func (*ThenFrame[A, B]) frame(func(A) B) {}

type typedThenFrame[A, B any] struct{ *ThenFrame[A, B] }

func (typedThenFrame[A, B]) frame(frameSignature) {}

// UnwindFrame represents an unrolled continuation frame that avoids closure allocation.
// It stores up to three type-erased variables alongside a function pointer, and is evaluated
// directly in the trampoline fast-path without interface type assertions.
// The 3 data fields (any, 16 B each), function pointer (8 B), and pooled flag (1 B + 7 B pad)
// total 64 bytes on amd64 — exactly one cache line.
type UnwindFrame struct {
	Data1 Erased
	Data2 Erased
	Data3 Erased
	// Unwind computes the next value and frame using the stored data and current value.
	Unwind func(Data1, Data2, Data3, current Erased) (Erased, Frame)

	pooled bool
}

func (*UnwindFrame) frame(frameSignature) { return }

// An effect instruction exposes its operation and the frames that receive the reply.
type effectInstruction interface {
	Frame
	splitEffect(rest Frame) (Operation, Frame)
}

// A perform instruction forwards the handler reply unchanged.
type performFrame interface {
	Frame
	operation() Operation
}

type storedOp struct{ op Operation }

func (storedOp) frame(frameSignature)   {}
func (f storedOp) operation() Operation { return f.op }

func (f storedOp) splitEffect(rest Frame) (Operation, Frame) { return f.op, rest }

type zeroSizeOp[O any] struct{}

func (zeroSizeOp[O]) frame(frameSignature) {}
func (zeroSizeOp[O]) operation() Operation {
	var op O
	return op
}

func (f zeroSizeOp[O]) splitEffect(rest Frame) (Operation, Frame) { return f.operation(), rest }

// resumeEffectFrame applies the explicit callback before recycling its frame.
func resumeEffectFrame(f *EffectFrame[Erased], v Resumed) Erased {
	current := f.Resume(v)
	releaseEffectFrame(f)
	return current
}

// EffectFrame represents a suspended effect operation.
// The handler dispatches on the operation and resumes with a value.
// Type parameters:
//   - A: the type the operation produces when resumed
type EffectFrame[A any] struct {
	// Operation is the effect operation for handler dispatch.
	Operation Operation

	// Resume is called with the handler's response value.
	Resume func(A) Erased

	// Next is the continuation frame after resumption.
	Next Frame

	pooled bool
}

func (*EffectFrame[A]) frame(frameSignature) { return }

// Expr is a defunctionalized continuation represented by explicit frames.
// Reuse an Expr only when its frames and callbacks support repeated evaluation.
// A pending [Reify] result and its copies share one evaluation.
type Expr[A any] struct {
	// Value is the result of a completed expression or the input to its first frame.
	Value A

	// Frame holds the next continuation frame.
	Frame Frame
}

// ExprReturn creates a completed computation with the given value.
func ExprReturn[A any](a A) Expr[A] {
	return Expr[A]{
		Value: a,
		Frame: ReturnFrame{},
	}
}

// ExprSuspend creates a computation suspended at the given frame.
func ExprSuspend[A any](frame Frame) Expr[A] {
	var zero A
	return Expr[A]{
		Value: zero,
		Frame: frame,
	}
}
