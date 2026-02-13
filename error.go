// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Error effect operations.
// Error[E] provides exception-like error handling.

// Throw is the effect operation for raising an error.
// Perform(Throw[E]{Err: e}) aborts the computation with error e.
type Throw[E any] struct{ Err E }

func (Throw[E]) OpResult() Resumed { panic("phantom") }

// DispatchError handles Throw in Error handler dispatch.
// Sets the error in the context and returns (struct{}{}, true) — uniform
// with State/Reader/Writer. The handler inspects ctx.HasErr to short-circuit.
func (o Throw[E]) DispatchError(ctx *ErrorContext[E]) (Resumed, bool) {
	ctx.Err = o.Err
	ctx.HasErr = true
	return struct{}{}, true
}

// Catch is the effect operation for handling errors.
// Perform(Catch[E, A]{Body: m, Handler: h}) runs m, catching errors with h.
//
// Like Listen/Censor, Catch runs the body with an error-only handler internally.
// Other effects (State, Reader, Writer) in the catch body are not handled.
type Catch[E, A any] struct {
	Body    Cont[Resumed, A]
	Handler func(E) Cont[Resumed, A]
}

func (Catch[E, A]) OpResult() A { panic("phantom") }

// DispatchError handles Catch in Error handler dispatch.
// Runs the body with RunError internally (like Listen/Censor pattern).
// Other effects in the catch body/handler are not handled.
func (o Catch[E, A]) DispatchError(ctx *ErrorContext[E]) (Resumed, bool) {
	bodyResult := RunError[E, A](o.Body)
	if bodyResult.IsLeft() {
		errVal, _ := bodyResult.GetLeft()
		handlerResult := RunError[E, A](o.Handler(errVal))
		if handlerResult.IsLeft() {
			e, _ := handlerResult.GetLeft()
			ctx.Err = e
			ctx.HasErr = true
			return struct{}{}, true
		}
		v, _ := handlerResult.GetRight()
		return v, true
	}
	v, _ := bodyResult.GetRight()
	return v, true
}

// ThrowError performs the Throw effect to raise an error.
// This aborts the current computation — the continuation k is never called.
func ThrowError[E, A any](err E) Cont[Resumed, A] {
	return func(k func(A) Resumed) Resumed {
		return effectMarker[A]{op: Throw[E]{Err: err}, k: k}
	}
}

// CatchError wraps a computation with an error handler.
func CatchError[E, A any](body Cont[Resumed, A], handler func(E) Cont[Resumed, A]) Cont[Resumed, A] {
	return Perform(Catch[E, A]{Body: body, Handler: handler})
}

// ExprThrowError creates an Expr that throws an error.
// Constructs EffectFrame directly because Throw[E].OpResult() returns
// Resumed, not A — ExprPerform would produce Expr[Resumed].
func ExprThrowError[E, A any](err E) Expr[A] {
	var zero A
	return Expr[A]{
		Value: zero,
		Frame: &EffectFrame[Erased]{
			Operation: Throw[E]{Err: err},
			Resume:    identityResume,
			Next:      ReturnFrame{},
		},
	}
}

// errorHandler implements Handler for zero-allocation error handling.
type errorHandler[E, A any] struct {
	ctx *ErrorContext[E]
}

// Dispatch implements Handler for error handling.
// Dispatches via structural interface assertion, then checks ctx.HasErr
// to determine whether to short-circuit with Left.
func (h *errorHandler[E, A]) Dispatch(op Operation) (Resumed, bool) {
	if eop, ok := op.(interface {
		DispatchError(ctx *ErrorContext[E]) (Resumed, bool)
	}); ok {
		v, _ := eop.DispatchError(h.ctx)
		if h.ctx.HasErr {
			return Left[E, A](h.ctx.Err), false
		}
		return v, true
	}
	unhandledEffect("ErrorHandler")
	return nil, false
}

// rightCont is the identity continuation for error runners.
// Named generic function produces a static funcval per type instantiation,
// avoiding the heap allocation that anonymous closures incur.
func rightCont[E, A any](a A) Resumed { return Right[E, A](a) }

// RunErrorExpr runs an Expr that may throw errors, returning Either.
// Handles Throw and Catch. Catch runs body with error-only handler internally.
func RunErrorExpr[E, A any](m Expr[A]) Either[E, A] {
	wrapped := ExprMap(m, func(a A) Either[E, A] { return Right[E, A](a) })
	var ctx ErrorContext[E]
	h := &errorHandler[E, A]{ctx: &ctx}
	return HandleExpr(wrapped, h)
}

// Either represents a value that is either Left (error) or Right (success).
type Either[E, A any] struct {
	isRight bool
	left    E
	right   A
}

// Left creates a Left (error) value.
func Left[E, A any](e E) Either[E, A] {
	return Either[E, A]{isRight: false, left: e}
}

// Right creates a Right (success) value.
func Right[E, A any](a A) Either[E, A] {
	return Either[E, A]{isRight: true, right: a}
}

// IsRight returns true if this is a Right value.
func (e Either[E, A]) IsRight() bool {
	return e.isRight
}

// IsLeft returns true if this is a Left value.
func (e Either[E, A]) IsLeft() bool {
	return !e.isRight
}

// GetRight returns the Right value and true, or zero and false.
func (e Either[E, A]) GetRight() (A, bool) {
	if e.isRight {
		return e.right, true
	}
	var zero A
	return zero, false
}

// GetLeft returns the Left value and true, or zero and false.
func (e Either[E, A]) GetLeft() (E, bool) {
	if !e.isRight {
		return e.left, true
	}
	var zero E
	return zero, false
}

// MatchEither pattern matches on the Either, calling onLeft or onRight.
func MatchEither[E, A, T any](e Either[E, A], onLeft func(E) T, onRight func(A) T) T {
	if e.isRight {
		return onRight(e.right)
	}
	return onLeft(e.left)
}

// RunError runs an error-capable computation and returns Either.
func RunError[E, A any](m Cont[Resumed, A]) Either[E, A] {
	var ctx ErrorContext[E]
	h := &errorHandler[E, A]{ctx: &ctx}
	result := m(rightCont[E, A])
	if result == nil {
		var zero A
		return Right[E, A](zero)
	}
	return handleDispatch[*errorHandler[E, A], Either[E, A]](result, h)
}

// MapEither applies a function to the Right value.
func MapEither[E, A, B any](e Either[E, A], f func(A) B) Either[E, B] {
	if e.isRight {
		return Right[E](f(e.right))
	}
	return Left[E, B](e.left)
}

// FlatMapEither sequences two Either computations.
func FlatMapEither[E, A, B any](e Either[E, A], f func(A) Either[E, B]) Either[E, B] {
	if e.isRight {
		return f(e.right)
	}
	return Left[E, B](e.left)
}

// MapLeftEither applies a function to the Left value.
func MapLeftEither[E, F, A any](e Either[E, A], f func(E) F) Either[F, A] {
	if e.isRight {
		return Right[F](e.right)
	}
	return Left[F, A](f(e.left))
}
