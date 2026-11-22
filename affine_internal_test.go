// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

import "testing"

func TestAffineReleasesCallback(t *testing.T) {
	callback := func(v int) int { return v }

	a := Once(callback)
	a.Resume(0)
	if a.resume != nil {
		t.Error("Resume retained its callback")
	}

	a = Once(callback)
	a.TryResume(0)
	if a.resume != nil {
		t.Error("TryResume retained its callback")
	}

	a = Once(callback)
	a.Discard()
	if a.resume != nil {
		t.Error("Discard retained its callback")
	}
}
