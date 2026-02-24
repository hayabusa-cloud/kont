// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Writer effect operations.
// Writer[W] provides accumulating output (logging, tracing).

// Tell is the effect operation for appending output.
// Perform(Tell[W]{Value: w}) appends w to the accumulated output.
type Tell[W any] struct{ Value W }

func (Tell[W]) OpResult() struct{} { panic("phantom") }

// DispatchWriter handles Tell in Writer handler dispatch.
func (o Tell[W]) DispatchWriter(ctx *WriterContext[W]) (Resumed, bool) {
	*ctx.Output = append(*ctx.Output, o.Value)
	return struct{}{}, true
}

// Listen is the effect operation for observing output.
// Perform(Listen[W, A]{Body: m}) runs m and returns its output alongside result.
//
// Note: Listen[W, A] for all A implements DispatchWriter through structural interface assertion.
// This fixes the type switch limitation where case Listen[W, Resumed] won't match Listen[W, int].
type Listen[W, A any] struct{ Body Cont[Resumed, A] }

func (Listen[W, A]) OpResult() Pair[A, []W] { panic("phantom") }

// DispatchWriter handles Listen in Writer handler dispatch.
// Listen[W, A] for all A dispatches through structural interface assertion.
func (o Listen[W, A]) DispatchWriter(ctx *WriterContext[W]) (Resumed, bool) {
	startLen := len(*ctx.Output)
	// Run the body with the same context, using correct type parameter
	result := Handle(o.Body, writerDispatchHandler[W, A](ctx))
	// Capture what was written during body execution
	written := make([]W, len(*ctx.Output)-startLen)
	copy(written, (*ctx.Output)[startLen:])
	// Return Pair[A, []W] to match the expected type from Perform
	return Pair[A, []W]{Fst: result, Snd: written}, true
}

// Censor is the effect operation for modifying output.
// Perform(Censor[W, A]{F: f, Body: m}) runs m and applies f to its output.
//
// Note: Like Listen, Censor[W, A] for all A implements DispatchWriter.
type Censor[W, A any] struct {
	F    func([]W) []W
	Body Cont[Resumed, A]
}

func (Censor[W, A]) OpResult() A { panic("phantom") }

// DispatchWriter handles Censor in Writer handler dispatch.
func (o Censor[W, A]) DispatchWriter(ctx *WriterContext[W]) (Resumed, bool) {
	startLen := len(*ctx.Output)
	// Run the body with the same context, using correct type parameter
	result := Handle(o.Body, writerDispatchHandler[W, A](ctx))
	// Apply censor function to the new output
	newOutput := o.F((*ctx.Output)[startLen:])
	*ctx.Output = append((*ctx.Output)[:startLen], newOutput...)
	return result, true
}

// Pair holds two values.
type Pair[A, B any] struct {
	Fst A
	Snd B
}

// TellWriter fuses Tell + Then: performs Tell, then runs next.
func TellWriter[W, B any](w W, next Cont[Resumed, B]) Cont[Resumed, B] {
	return func(k func(B) Resumed) Resumed {
		m := acquireMarker()
		m.op = Tell[W]{Value: w}
		m.f = next
		m.k = k
		m.resume = thenMarkerResume[B]
		return m
	}
}

// ListenWriter runs a computation and returns its output alongside the result.
func ListenWriter[W, A any](body Cont[Resumed, A]) Cont[Resumed, Pair[A, []W]] {
	return Perform(Listen[W, A]{Body: body})
}

// CensorWriter runs a computation and modifies its output.
func CensorWriter[W, A any](f func([]W) []W, body Cont[Resumed, A]) Cont[Resumed, A] {
	return Perform(Censor[W, A]{F: f, Body: body})
}

// writerHandler implements Handler for zero-allocation writer handling.
type writerHandler[W, R any] struct {
	ctx *WriterContext[W]
}

// Dispatch implements Handler for zero-allocation handling.
func (h *writerHandler[W, R]) Dispatch(op Operation) (Resumed, bool) {
	if wop, ok := op.(interface {
		DispatchWriter(ctx *WriterContext[W]) (Resumed, bool)
	}); ok {
		return wop.DispatchWriter(h.ctx)
	}
	unhandledEffect("WriterHandler")
	return nil, false
}

// writerDispatchHandler creates a handler using the dispatch interface.
// This is an internal helper used by WriterHandler and Listen/Censor dispatch.
func writerDispatchHandler[W, R any](ctx *WriterContext[W]) *writerHandler[W, R] {
	return &writerHandler[W, R]{ctx: ctx}
}

// WriterHandler creates a handler for Writer effects.
// Returns a concrete handler and a function to retrieve accumulated output.
func WriterHandler[W, R any]() (*writerHandler[W, R], func() []W) {
	var output []W
	ctx := &WriterContext[W]{Output: &output}
	return writerDispatchHandler[W, R](ctx), func() []W { return output }
}

// RunWriter runs a writer computation and returns both result and output.
func RunWriter[W, A any](m Cont[Resumed, A]) (A, []W) {
	var output []W
	ctx := &WriterContext[W]{Output: &output}
	h := &writerHandler[W, A]{ctx: ctx}
	result := Handle(m, h)
	return result, output
}

// ExecWriter runs a writer computation and returns only the output.
func ExecWriter[W, A any](m Cont[Resumed, A]) []W {
	_, output := RunWriter[W, A](m)
	return output
}

// RunWriterExpr runs an Expr writer computation.
func RunWriterExpr[W, A any](m Expr[A]) (A, []W) {
	var output []W
	ctx := &WriterContext[W]{Output: &output}
	h := &writerHandler[W, A]{ctx: ctx}
	result := HandleExpr(m, h)
	return result, output
}
