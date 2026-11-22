// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "sync"

var genericMarkerPool = sync.Pool{
	New: func() any { return new(genericMarker) },
}

type genericMarker struct {
	op     Operation
	resume func(*genericMarker, Resumed) Resumed
	f      any
	k      any
}

func (m *genericMarker) Op() Operation            { return m.op }
func (m *genericMarker) Resume(v Resumed) Resumed { return m.resume(m, v) }

func acquireMarker() *genericMarker {
	return genericMarkerPool.Get().(*genericMarker)
}

func discardMarker(m *genericMarker) {
	if rest := takeMarkerSuffix(m); rest != nil {
		releaseFrameChain(rest)
	}
}

// A reflected marker owns the Expr suffix in f. Take that link before
// recycling the marker so the caller can continue disposal iteratively.
func takeMarkerSuffix(m *genericMarker) Frame {
	rest, _ := m.f.(Frame)
	releaseMarker(m)
	return rest
}

func releaseMarker(m *genericMarker) {
	m.op = nil
	m.resume = nil
	m.f = nil
	m.k = nil
	genericMarkerPool.Put(m)
}
