// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Composed effect handlers for multi-effect computations.
// These avoid nesting Run* calls by dispatching multiple effect families
// from a single handler via handleDispatch/HandleExpr.

// stateReaderHandler handles both State and Reader effects.
type stateReaderHandler[S, E, R any] struct {
	state *S
	env   *E
}

// Dispatch implements Handler for the composed State+Reader handler.
func (h *stateReaderHandler[S, E, R]) Dispatch(op Operation) (Resumed, bool) {
	if sop, ok := op.(interface {
		DispatchState(state *S) (Resumed, bool)
	}); ok {
		return sop.DispatchState(h.state)
	}
	if rop, ok := op.(interface {
		DispatchReader(env *E) (Resumed, bool)
	}); ok {
		return rop.DispatchReader(h.env)
	}
	unhandledEffect("StateReaderHandler")
	return nil, false
}

// RunStateReaderExpr runs an Expr with both State and Reader effects.
func RunStateReaderExpr[S, E, A any](initial S, env E, m Expr[A]) (A, S) {
	state := initial
	e := env
	h := &stateReaderHandler[S, E, A]{state: &state, env: &e}
	result := HandleExpr(m, h)
	return result, state
}

// RunStateReader runs a computation with both State and Reader effects.
// Returns the result and final state.
func RunStateReader[S, E, A any](initial S, env E, m Cont[Resumed, A]) (A, S) {
	state := initial
	e := env
	h := &stateReaderHandler[S, E, A]{state: &state, env: &e}
	result := Handle(m, h)
	return result, state
}

// stateErrorHandler handles both State and Error effects.
type stateErrorHandler[S, E, A any] struct {
	state *S
	ctx   *ErrorContext[E]
}

// Dispatch implements Handler for the composed State+Error handler.
// Dispatch order: State → Error.
// Catch runs body with error-only handler internally (like Listen/Censor).
func (h *stateErrorHandler[S, E, A]) Dispatch(op Operation) (Resumed, bool) {
	if sop, ok := op.(interface {
		DispatchState(state *S) (Resumed, bool)
	}); ok {
		return sop.DispatchState(h.state)
	}
	if eop, ok := op.(interface {
		DispatchError(ctx *ErrorContext[E]) (Resumed, bool)
	}); ok {
		v, _ := eop.DispatchError(h.ctx)
		if h.ctx.HasErr {
			return Left[E, A](h.ctx.Err), false
		}
		return v, true
	}
	unhandledEffect("StateErrorHandler")
	return nil, false
}

// RunStateError runs a computation with both State and Error effects.
// Returns (Either[E, A], S) — state is always available, even on error.
func RunStateError[S, E, A any](initial S, m Cont[Resumed, A]) (Either[E, A], S) {
	state := initial
	var ctx ErrorContext[E]
	h := &stateErrorHandler[S, E, A]{state: &state, ctx: &ctx}
	result := m(rightCont[E, A])
	if result == nil {
		var zero A
		return Right[E, A](zero), state
	}
	either := handleDispatch[*stateErrorHandler[S, E, A], Either[E, A]](result, h)
	return either, state
}

// EvalStateError runs a State+Error computation and returns only the Either result.
func EvalStateError[S, E, A any](initial S, m Cont[Resumed, A]) Either[E, A] {
	result, _ := RunStateError[S, E, A](initial, m)
	return result
}

// ExecStateError runs a State+Error computation and returns only the final state.
func ExecStateError[S, E, A any](initial S, m Cont[Resumed, A]) S {
	_, state := RunStateError[S, E, A](initial, m)
	return state
}

// RunStateErrorExpr runs an Expr with both State and Error effects.
// Handles Throw and Catch. Catch runs body with error-only handler internally.
func RunStateErrorExpr[S, E, A any](initial S, m Expr[A]) (Either[E, A], S) {
	wrapped := ExprMap(m, func(a A) Either[E, A] { return Right[E, A](a) })
	state := initial
	var ctx ErrorContext[E]
	h := &stateErrorHandler[S, E, A]{state: &state, ctx: &ctx}
	result := HandleExpr(wrapped, h)
	return result, state
}

// stateWriterHandler handles both State and Writer effects.
type stateWriterHandler[S, W, R any] struct {
	state *S
	ctx   *WriterContext[W]
}

// Dispatch implements Handler for the composed State+Writer handler.
// Listen/Censor with State operations inside is not supported and will panic.
func (h *stateWriterHandler[S, W, R]) Dispatch(op Operation) (Resumed, bool) {
	if sop, ok := op.(interface {
		DispatchState(state *S) (Resumed, bool)
	}); ok {
		return sop.DispatchState(h.state)
	}
	if wop, ok := op.(interface {
		DispatchWriter(ctx *WriterContext[W]) (Resumed, bool)
	}); ok {
		return wop.DispatchWriter(h.ctx)
	}
	unhandledEffect("StateWriterHandler")
	return nil, false
}

// RunStateWriter runs a computation with both State and Writer effects.
// Returns (A, S, []W). Both effects always resume — no short-circuit.
func RunStateWriter[S, W, A any](initial S, m Cont[Resumed, A]) (A, S, []W) {
	state := initial
	var output []W
	ctx := &WriterContext[W]{Output: &output}
	h := &stateWriterHandler[S, W, A]{state: &state, ctx: ctx}
	result := Handle(m, h)
	return result, state, output
}

// RunStateWriterExpr runs an Expr with both State and Writer effects.
func RunStateWriterExpr[S, W, A any](initial S, m Expr[A]) (A, S, []W) {
	state := initial
	var output []W
	ctx := &WriterContext[W]{Output: &output}
	h := &stateWriterHandler[S, W, A]{state: &state, ctx: ctx}
	result := HandleExpr(m, h)
	return result, state, output
}

// readerStateErrorHandler handles Reader, State, and Error effects.
type readerStateErrorHandler[Env, S, Err, A any] struct {
	env   *Env
	state *S
	ctx   *ErrorContext[Err]
}

// Dispatch implements Handler for the composed Reader+State+Error handler.
// Dispatch order: Reader → State → Error.
// Catch runs body with error-only handler internally (like Listen/Censor).
func (h *readerStateErrorHandler[Env, S, Err, A]) Dispatch(op Operation) (Resumed, bool) {
	if rop, ok := op.(interface {
		DispatchReader(env *Env) (Resumed, bool)
	}); ok {
		return rop.DispatchReader(h.env)
	}
	if sop, ok := op.(interface {
		DispatchState(state *S) (Resumed, bool)
	}); ok {
		return sop.DispatchState(h.state)
	}
	if eop, ok := op.(interface {
		DispatchError(ctx *ErrorContext[Err]) (Resumed, bool)
	}); ok {
		v, _ := eop.DispatchError(h.ctx)
		if h.ctx.HasErr {
			return Left[Err, A](h.ctx.Err), false
		}
		return v, true
	}
	unhandledEffect("ReaderStateErrorHandler")
	return nil, false
}

// RunReaderStateError runs a computation with Reader, State, and Error effects.
// Dispatch order: Reader → State → Error.
// Returns (Either[Err, A], S).
func RunReaderStateError[Env, S, Err, A any](env Env, initial S, m Cont[Resumed, A]) (Either[Err, A], S) {
	e := env
	state := initial
	var ctx ErrorContext[Err]
	h := &readerStateErrorHandler[Env, S, Err, A]{env: &e, state: &state, ctx: &ctx}
	result := m(rightCont[Err, A])
	if result == nil {
		var zero A
		return Right[Err, A](zero), state
	}
	either := handleDispatch[*readerStateErrorHandler[Env, S, Err, A], Either[Err, A]](result, h)
	return either, state
}

// RunReaderStateErrorExpr runs an Expr with Reader, State, and Error effects.
// Handles Throw and Catch. Catch runs body with error-only handler internally.
func RunReaderStateErrorExpr[Env, S, Err, A any](env Env, initial S, m Expr[A]) (Either[Err, A], S) {
	wrapped := ExprMap(m, func(a A) Either[Err, A] { return Right[Err, A](a) })
	e := env
	state := initial
	var ctx ErrorContext[Err]
	h := &readerStateErrorHandler[Env, S, Err, A]{env: &e, state: &state, ctx: &ctx}
	result := HandleExpr(wrapped, h)
	return result, state
}
