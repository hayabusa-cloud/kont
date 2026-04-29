// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync"

// Frame pools for type-erased frames in the Expr evaluation pipeline.
// evalFrames releases pooled frames after consumption, zeroing all fields.
// Pooled frames require affine (at-most-once) evaluation; reuse dereferences nil.
// Safe for external consumers that guarantee single-use (affine) evaluation,
// not for kont's public constructors whose output is reusable.

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

func releaseFrameChain(frame Frame) {
	for frame != nil {
		if cf, ok := frame.(*chainedFrame); ok {
			first, rest := cf.first, cf.rest
			releaseChain(cf)
			releaseFrameChain(first)
			frame = rest
			continue
		}
		switch f := frame.(type) {
		case ReturnFrame:
			return
		case *BindFrame[Erased, Erased]:
			next := f.Next
			releaseBindFrame(f)
			frame = next
		case *MapFrame[Erased, Erased]:
			frame = f.Next
		case *ThenFrame[Erased, Erased]:
			second, next := f.Second.Frame, f.Next
			releaseThenFrame(f)
			releaseFrameChain(second)
			frame = next
		case *EffectFrame[Erased]:
			next := f.Next
			releaseEffectFrame(f)
			frame = next
		case *UnwindFrame:
			releaseUnwindFrame(f)
			return
		default:
			return
		}
	}
}
