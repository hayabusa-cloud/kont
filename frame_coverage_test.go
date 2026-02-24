package kont

import "testing"

var sink []Frame

func TestFrameMethods(t *testing.T) {
	sink = []Frame{
		ReturnFrame{},
		(*BindFrame[any, any])(nil),
		(*MapFrame[any, any])(nil),
		(*ThenFrame[any, any])(nil),
		(*UnwindFrame)(nil),
		(*EffectFrame[any])(nil),
	}
	for _, f := range sink {
		f.frame()
	}
}
