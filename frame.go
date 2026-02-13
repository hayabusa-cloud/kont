// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Erased represents a type-erased value in the defunctionalized frame chain.
// Frame types use Erased parameters to process heterogeneous value types
// through a homogeneous evaluation pipeline. Concrete types are recovered
// via type assertions at frame boundaries.
type Erased = any

// Frame is the interface for defunctionalized continuation frames.
// Implementations carry the data needed to continue computation.
// Dispatch uses type switches, not tags — Frame is a pure marker interface.
type Frame interface {
	frame() // unexported marker method
}

// ReturnFrame signals computation completion.
// The evaluator returns the current value as the final result.
type ReturnFrame struct{}

func (ReturnFrame) frame() {}

// BindFrame represents monadic bind: Bind(m, f)
// Type parameters:
//   - A: input type (value from previous computation)
//   - B: output type (result of applying F)
type BindFrame[A, B any] struct {
	// F is the continuation function to apply to the input value.
	F func(A) Expr[B]

	// Next is the continuation frame after F completes.
	Next Frame
}

func (*BindFrame[A, B]) frame() {}

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

func (*MapFrame[A, B]) frame() {}

// ThenFrame represents sequencing with discard: Then(m, n)
// Type parameters:
//   - A: discarded type (result of first computation, unused)
//   - B: output type (result of second computation)
type ThenFrame[A, B any] struct {
	// Second is the computation to evaluate after discarding first result.
	Second Expr[B]

	// Next is the continuation frame after Second completes.
	Next Frame
}

func (*ThenFrame[A, B]) frame() {}

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
}

func (*EffectFrame[A]) frame() {}

// Expr is a defunctionalized continuation.
// Unlike the closure-based Cont[R, A], this carries explicit frame data.
type Expr[A any] struct {
	// Value holds the current value if this is a completed computation.
	// Valid when Frame is ReturnFrame.
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
