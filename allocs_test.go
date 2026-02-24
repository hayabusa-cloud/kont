// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"code.hybscloud.com/kont"
	"testing"
)

func TestExprAllocationsPure(t *testing.T) {
	expr := kont.ExprReturn(42)
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = kont.StepExpr(expr)
	})
	if allocs > 0 {
		t.Errorf("StepExpr(ExprReturn) allocs = %v; want 0", allocs)
	}

	expr2 := kont.ExprMap(kont.ExprReturn(42), func(x int) int { return x + 1 })
	allocs2 := testing.AllocsPerRun(100, func() {
		_, _ = kont.StepExpr(expr2)
	})
	if allocs2 > 0 {
		t.Errorf("StepExpr(ExprMap) allocs = %v; want 0", allocs2)
	}
}
