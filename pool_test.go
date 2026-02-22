// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestAcquireEffectFrame(t *testing.T) {
	ef := kont.AcquireEffectFrame()
	ef.Operation = kont.Get[int]{}
	ef.Resume = func(v any) any { return v }
	ef.Next = kont.ReturnFrame{}

	expr := kont.Expr[int]{Frame: ef}
	result := kont.HandleExpr(expr, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
		return 42, true
	}))
	if result != 42 {
		t.Fatalf("got %v, want 42", result)
	}
}

func TestAcquireBindFrame(t *testing.T) {
	bf := kont.AcquireBindFrame()
	bf.F = func(a any) kont.Expr[any] {
		return kont.ExprReturn[any](a.(int) * 2)
	}
	bf.Next = kont.ReturnFrame{}

	expr := kont.Expr[int]{Value: 21, Frame: bf}
	result := kont.RunPure(expr)
	if result != 42 {
		t.Fatalf("got %v, want 42", result)
	}
}

func TestAcquireThenFrame(t *testing.T) {
	tf := kont.AcquireThenFrame()
	tf.Second = kont.Expr[any]{Value: 99, Frame: kont.ReturnFrame{}}
	tf.Next = kont.ReturnFrame{}

	expr := kont.Expr[int]{Value: 0, Frame: tf}
	result := kont.RunPure(expr)
	if result != 99 {
		t.Fatalf("got %v, want 99", result)
	}
}
