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
