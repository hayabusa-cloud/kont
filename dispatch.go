// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// WriterContext holds the state needed for Writer effect dispatch.
type WriterContext[W any] struct {
	Output *[]W
}

// ErrorContext holds the state needed for Error effect dispatch.
type ErrorContext[E any] struct {
	Err    E
	HasErr bool
}
