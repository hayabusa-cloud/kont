// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Reader effect operations.
// Reader[E] provides read-only access to an environment.

// Ask is the effect operation for reading the environment.
// Perform(Ask[E]{}) returns the current environment of type E.
type Ask[E any] struct{}

func (Ask[E]) OpResult() E { panic("phantom") }

// DispatchReader handles Ask in Reader handler dispatch.
func (Ask[E]) DispatchReader(env *E) (Resumed, bool) {
	return *env, true
}

// AskReader fuses Ask + Bind: performs Ask, passes environment to f.
func AskReader[E, B any](f func(E) Cont[Resumed, B]) Cont[Resumed, B] {
	return func(k func(B) Resumed) Resumed {
		return bindMarker[E, B]{op: Ask[E]{}, f: f, k: k}
	}
}

// MapReader fuses Ask + Map: performs Ask, applies projection f.
func MapReader[E, A any](f func(E) A) Cont[Resumed, A] {
	return func(k func(A) Resumed) Resumed {
		return mapMarker[E, A]{op: Ask[E]{}, f: f, k: k}
	}
}

// readerHandler implements Handler for zero-allocation reader handling.
type readerHandler[E, R any] struct {
	env *E
}

// Dispatch implements Handler for zero-allocation handling.
func (h *readerHandler[E, R]) Dispatch(op Operation) (Resumed, bool) {
	if rop, ok := op.(interface{ DispatchReader(env *E) (Resumed, bool) }); ok {
		return rop.DispatchReader(h.env)
	}
	unhandledEffect("ReaderHandler")
	return nil, false
}

// ReaderHandler creates a handler for Reader effects with the given environment.
// Returns a concrete handler.
func ReaderHandler[E, R any](env E) *readerHandler[E, R] {
	e := env
	return &readerHandler[E, R]{env: &e}
}

// RunReaderExpr runs an Expr computation with the given environment.
func RunReaderExpr[E, A any](env E, m Expr[A]) A {
	h := ReaderHandler[E, A](env)
	return HandleExpr(m, h)
}

// RunReader runs a computation with the given environment.
func RunReader[E, A any](env E, m Cont[Resumed, A]) A {
	h := ReaderHandler[E, A](env)
	return Handle(m, h)
}
