package kont

import "testing"

func TestFrameMethods(t *testing.T) {
	pending := Reify(Perform(Get[int]{}))
	frames := []Frame{
		ReturnFrame{},
		(*BindFrame[any, any])(nil),
		(*MapFrame[any, any])(nil),
		(*ThenFrame[any, any])(nil),
		(*UnwindFrame)(nil),
		(*EffectFrame[any])(nil),
		(&BindFrame[int, string]{}).AsFrame(),
		(&MapFrame[int, string]{}).AsFrame(),
		(&ThenFrame[int, string]{}).AsFrame(),
		ExprPerform(Get[int]{}).Frame,
		ExprPerform(Put[int]{Value: 1}).Frame,
		pending.Frame,
		cpsEffect[int]{},
		cpsReply[int]{},
		reflectedCallback{},
		(*chainedFrame)(nil),
		bindFunc[int, string](nil),
		mapFunc[int, string](nil),
	}
	for _, f := range frames {
		f.frame(func(v Erased) Erased {
			t.Errorf("%T.frame called its argument", f)
			return v
		})
	}
	if got, _ := RunStateExpr(42, pending); got != 42 {
		t.Errorf("reified Get after frame marker call = %d, want 42", got)
	}
}
