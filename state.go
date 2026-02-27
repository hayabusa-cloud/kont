// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// State effect operations.
// State[S] provides mutable state threading through computations.

// Get is the effect operation for reading state.
// Perform(Get[S]{}) returns the current state of type S.
type Get[S any] struct{}

func (Get[S]) OpResult() S { panic("phantom") }

// DispatchState handles Get in State handler dispatch.
func (Get[S]) DispatchState(state *S) (Resumed, bool) {
	return *state, true
}

// Put is the effect operation for writing state.
// Perform(Put[S]{Value: s}) replaces the current state.
type Put[S any] struct{ Value S }

func (Put[S]) OpResult() struct{} { panic("phantom") }

// DispatchState handles Put in State handler dispatch.
func (o Put[S]) DispatchState(state *S) (Resumed, bool) {
	*state = o.Value
	return struct{}{}, true
}

// Modify is the effect operation for modifying state.
// Perform(Modify[S]{F: f}) applies f to state and returns the new state.
type Modify[S any] struct{ F func(S) S }

func (Modify[S]) OpResult() S { panic("phantom") }

// DispatchState handles Modify in State handler dispatch.
func (o Modify[S]) DispatchState(state *S) (Resumed, bool) {
	*state = o.F(*state)
	return *state, true
}

// GetState fuses Get + Bind: performs Get, passes state to f.
func GetState[S, B any](f func(S) Cont[Resumed, B]) Cont[Resumed, B] {
	resume := bindMarkerResume[S, B]
	return func(k func(B) Resumed) Resumed {
		m := acquireMarker()
		m.op = Get[S]{}
		m.f = f
		m.k = k
		m.resume = resume
		return m
	}
}

// PutState fuses Put + Then: performs Put, then runs next.
func PutState[S, B any](s S, next Cont[Resumed, B]) Cont[Resumed, B] {
	resume := thenMarkerResume[B]
	return func(k func(B) Resumed) Resumed {
		m := acquireMarker()
		m.op = Put[S]{Value: s}
		m.f = next
		m.k = k
		m.resume = resume
		return m
	}
}

// ModifyState fuses Modify + Bind: performs Modify, passes new state to f.
func ModifyState[S, B any](f func(S) S, then func(S) Cont[Resumed, B]) Cont[Resumed, B] {
	resume := bindMarkerResume[S, B]
	return func(k func(B) Resumed) Resumed {
		m := acquireMarker()
		m.op = Modify[S]{F: f}
		m.f = then
		m.k = k
		m.resume = resume
		return m
	}
}

// stateHandler implements Handler for zero-allocation state handling.
type stateHandler[S, R any] struct {
	state *S
}

// Dispatch implements Handler for zero-allocation handling.
func (h *stateHandler[S, R]) Dispatch(op Operation) (Resumed, bool) {
	switch o := op.(type) {
	case Get[S]:
		return o.DispatchState(h.state)
	case Put[S]:
		return o.DispatchState(h.state)
	case Modify[S]:
		return o.DispatchState(h.state)
	default:
		if sop, ok := op.(interface {
			DispatchState(state *S) (Resumed, bool)
		}); ok {
			return sop.DispatchState(h.state)
		}
	}
	unhandledEffect("StateHandler")
	return nil, false
}

// StateHandler creates a handler for State effects with the given initial state.
// Returns a concrete handler and a function to retrieve the current state.
func StateHandler[S, R any](initial S) (*stateHandler[S, R], func() S) {
	state := initial
	h := &stateHandler[S, R]{state: &state}
	return h, func() S { return state }
}

func dispatchState[S any](op Operation, state *S) (Resumed, bool) {
	switch o := op.(type) {
	case Get[S]:
		return o.DispatchState(state)
	case Put[S]:
		return o.DispatchState(state)
	case Modify[S]:
		return o.DispatchState(state)
	default:
		if sop, ok := op.(interface {
			DispatchState(state *S) (Resumed, bool)
		}); ok {
			return sop.DispatchState(state)
		}
	}
	unhandledEffect("StateHandler")
	return nil, false
}

// RunState runs a stateful computation and returns both the result and final state.
func RunState[S, A any](initial S, m Cont[Resumed, A]) (A, S) {
	state := initial
	result := m(toResumed[A])
	for {
		if susp, ok := result.(effectSuspension); ok {
			v, shouldResume := dispatchState(susp.Op(), &state)
			if !shouldResume {
				susp.release()
				return v.(A), state
			}
			result = susp.Resume(v)
			continue
		}
		if result == nil {
			var zero A
			return zero, state
		}
		return result.(A), state
	}
}

// EvalState runs a stateful computation and returns only the result.
func EvalState[S, A any](initial S, m Cont[Resumed, A]) A {
	result, _ := RunState[S, A](initial, m)
	return result
}

// ExecState runs a stateful computation and returns only the final state.
func ExecState[S, A any](initial S, m Cont[Resumed, A]) S {
	_, state := RunState[S, A](initial, m)
	return state
}

// RunStateExpr runs a stateful Expr computation.
func RunStateExpr[S, A any](initial S, m Expr[A]) (A, S) {
	state := initial
	result := evalFrames(Erased(m.Value), m.Frame, handlerProcessor[*stateHandlerInline[S, A], A]{h: &stateHandlerInline[S, A]{state: &state}})
	return result, state
}

type stateHandlerInline[S, R any] struct {
	state *S
}

func (h *stateHandlerInline[S, R]) Dispatch(op Operation) (Resumed, bool) {
	return dispatchState(op, h.state)
}
