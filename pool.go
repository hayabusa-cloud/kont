// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync"

// Frame pools for type-erased frames in the Expr evaluation pipeline.
// evalFrames releases pooled frames after consumption, zeroing all fields.
// Pooled frames require affine (at-most-once) evaluation; reuse dereferences nil.
// Callers that expose reusable Expr values must use reusable frames.

var effectFramePool = sync.Pool{New: func() any { return new(EffectFrame[Erased]) }}
var bindFramePool = sync.Pool{New: func() any { return new(BindFrame[Erased, Erased]) }}
var thenFramePool = sync.Pool{New: func() any { return new(ThenFrame[Erased, Erased]) }}
var unwindFramePool = sync.Pool{New: func() any { return new(UnwindFrame) }}

// AcquireEffectFrame acquires a pooled single-use EffectFrame[Erased] whose
// Operation, Resume, and Next fields must be filled before evaluation.
func AcquireEffectFrame() *EffectFrame[Erased] {
	f := effectFramePool.Get().(*EffectFrame[Erased])
	f.pooled = true
	return f
}

// AcquireBindFrame acquires a pooled single-use BindFrame[Erased, Erased] whose
// F and Next fields must be filled before evaluation.
func AcquireBindFrame() *BindFrame[Erased, Erased] {
	f := bindFramePool.Get().(*BindFrame[Erased, Erased])
	f.pooled = true
	return f
}

// AcquireThenFrame acquires a pooled single-use ThenFrame[Erased, Erased] whose
// Second and Next fields must be filled before evaluation.
func AcquireThenFrame() *ThenFrame[Erased, Erased] {
	f := thenFramePool.Get().(*ThenFrame[Erased, Erased])
	f.pooled = true
	return f
}

// AcquireUnwindFrame acquires a pooled single-use UnwindFrame whose
// Unwind field (and any necessary Data fields) must be filled before evaluation.
func AcquireUnwindFrame() *UnwindFrame {
	f := unwindFramePool.Get().(*UnwindFrame)
	f.pooled = true
	return f
}

// releaseEffectFrame zeroes and returns f to the pool; no-op if not pooled.
func releaseEffectFrame(f *EffectFrame[Erased]) {
	if !f.pooled {
		return
	}
	f.Operation = nil
	f.Resume = nil
	f.Next = nil
	f.pooled = false
	effectFramePool.Put(f)
}

// releaseBindFrame zeroes and returns f to the pool; no-op if not pooled.
func releaseBindFrame(f *BindFrame[Erased, Erased]) {
	if !f.pooled {
		return
	}
	f.F = nil
	f.Next = nil
	f.pooled = false
	bindFramePool.Put(f)
}

// releaseThenFrame zeroes and returns f to the pool; no-op if not pooled.
func releaseThenFrame(f *ThenFrame[Erased, Erased]) {
	if !f.pooled {
		return
	}
	f.Second = Expr[Erased]{}
	f.Next = nil
	f.pooled = false
	thenFramePool.Put(f)
}

// releaseUnwindFrame zeroes and returns f to the pool; no-op if not pooled.
func releaseUnwindFrame(f *UnwindFrame) {
	if !f.pooled {
		return
	}
	f.Data1 = nil
	f.Data2 = nil
	f.Data3 = nil
	f.Unwind = nil
	f.pooled = false
	unwindFramePool.Put(f)
}

// Only package-owned typed views have this method. Custom frame embedding
// cannot expose the caller's fields to disposal.
type typedFrameLinks interface {
	discardLinks() (Frame, Frame)
}

func (f typedBindFrame[A, B]) discardLinks() (Frame, Frame) { return f.Next, nil }
func (f typedMapFrame[A, B]) discardLinks() (Frame, Frame)  { return f.Next, nil }
func (f typedThenFrame[A, B]) discardLinks() (Frame, Frame) { return f.Second.Frame, f.Next }

func releaseFrameChain(frame Frame) {
	var rest Frame = ReturnFrame{}
	frame = releaseLinearFrames(frame)
	for frame != nil {
		var first, next Frame
		switch f := frame.(type) {
		case *chainedFrame:
			first, next = f.first, f.rest
			releaseChain(f)
		case *ThenFrame[Erased, Erased]:
			first, next = f.Second.Frame, f.Next
			releaseThenFrame(f)
		case typedFrameLinks:
			first, next = f.discardLinks()
		}

		// A linear side can be released before deferring either branch.
		frame = releaseLinearFrames(first)
		next = releaseLinearFrames(next)
		switch {
		case frame == nil && next == nil:
			if _, done := rest.(ReturnFrame); done {
				return
			}
			frame, rest = rest, ReturnFrame{}
		case frame == nil:
			frame = next
		case next != nil:
			rest = chainFromPool(next, rest)
		}
	}
}

// releaseLinearFrames clears frames until it reaches a fork or the end of a branch.
func releaseLinearFrames(frame Frame) Frame {
	for {
		switch f := frame.(type) {
		case *chainedFrame:
			if f.callbackTail {
				frame = f.first
				continue
			}
			return f
		case *BindFrame[Erased, Erased]:
			frame = f.Next
			releaseBindFrame(f)
		case *MapFrame[Erased, Erased]:
			frame = f.Next
		case *ThenFrame[Erased, Erased]:
			if _, done := f.Next.(ReturnFrame); !done {
				return f
			}
			frame = f.Second.Frame
			releaseThenFrame(f)
		case *EffectFrame[Erased]:
			frame = f.Next
			releaseEffectFrame(f)
		case *UnwindFrame:
			releaseUnwindFrame(f)
			return nil
		case typedFrameLinks:
			first, next := f.discardLinks()
			switch next.(type) {
			case nil, ReturnFrame:
				frame = first
			default:
				return frame
			}
		case interface{ discardFrame() Frame }:
			frame = f.discardFrame()
		default:
			return nil
		}
	}
}
