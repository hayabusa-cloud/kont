// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

// BenchmarkHandleSingleState measures allocation for single State effect.
func BenchmarkHandleSingleState(b *testing.B) {
	for b.Loop() {
		_ = kont.EvalState[int, int](0, kont.Perform(kont.Get[int]{}))
	}
}

// BenchmarkHandleMultipleState measures allocation for multiple State effects.
func BenchmarkHandleMultipleState(b *testing.B) {
	computation := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(x+1, kont.GetState(func(y int) kont.Cont[kont.Resumed, int] {
			return kont.PutState(y*2, kont.Perform(kont.Get[int]{}))
		}))
	})

	for b.Loop() {
		_ = kont.EvalState[int, int](0, computation)
	}
}

// BenchmarkBindChain measures allocation for Bind chain composition.
func BenchmarkBindChain(b *testing.B) {
	pure := func(x int) kont.Cont[int, int] {
		return kont.Return[int](x)
	}
	inc := func(x int) kont.Cont[int, int] {
		return kont.Return[int](x + 1)
	}

	// Chain of 10 binds
	chain := kont.Bind(pure(0), func(x int) kont.Cont[int, int] {
		return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
			return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
				return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
					return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
						return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
							return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
								return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
									return kont.Bind(inc(x), func(x int) kont.Cont[int, int] {
										return inc(x)
									})
								})
							})
						})
					})
				})
			})
		})
	})

	for b.Loop() {
		_ = kont.Run(chain)
	}
}

// BenchmarkStateGetPut measures Get/Put cycle allocation.
func BenchmarkStateGetPut(b *testing.B) {
	computation := kont.GetState(func(x int) kont.Cont[kont.Resumed, struct{}] {
		return kont.Perform(kont.Put[int]{Value: x + 1})
	})

	for b.Loop() {
		_, _ = kont.RunState[int, struct{}](0, computation)
	}
}

// BenchmarkReturn measures pure Return allocation (baseline).
func BenchmarkReturn(b *testing.B) {
	m := kont.Return[int](42)
	for b.Loop() {
		_ = kont.Run(m)
	}
}

// BenchmarkMap measures Map allocation.
func BenchmarkMap(b *testing.B) {
	m := kont.Map(kont.Return[int](42), func(x int) int { return x * 2 })
	for b.Loop() {
		_ = kont.Run(m)
	}
}

// BenchmarkReaderAsk measures Reader effect allocation.
func BenchmarkReaderAsk(b *testing.B) {
	computation := kont.AskReader(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.Return[kont.Resumed](x)
	})
	for b.Loop() {
		_ = kont.RunReader[int, int](42, computation)
	}
}

// BenchmarkWriterTell measures Writer effect allocation.
func BenchmarkWriterTell(b *testing.B) {
	computation := kont.TellWriter[int, struct{}](42, kont.Return[kont.Resumed](struct{}{}))
	for b.Loop() {
		_, _ = kont.RunWriter[int, struct{}](computation)
	}
}

// BenchmarkThenChain measures allocation for Then chain composition.
// Then avoids the transformation function closure capture that Bind requires.
func BenchmarkThenChain(b *testing.B) {
	unit := kont.Return[int](struct{}{})

	// Chain of 10 thens (no value passing, just sequencing)
	chain := kont.Then(unit, kont.Then(unit, kont.Then(unit, kont.Then(unit, kont.Then(unit,
		kont.Then(unit, kont.Then(unit, kont.Then(unit, kont.Then(unit,
			kont.Return[int](42))))))))))

	for b.Loop() {
		_ = kont.Run(chain)
	}
}

// BenchmarkMapReader measures allocation for MapReader (optimized with Map).
func BenchmarkMapReader(b *testing.B) {
	computation := kont.MapReader[int, int](func(x int) int { return x * 2 })
	for b.Loop() {
		_ = kont.RunReader[int, int](42, computation)
	}
}

// BenchmarkShiftReset measures Shift/Reset delimited continuation.
func BenchmarkShiftReset(b *testing.B) {
	m := kont.Reset[int](
		kont.Bind(kont.Shift[int, int](func(k func(int) int) int {
			return k(21) + k(21)
		}), func(x int) kont.Cont[int, int] {
			return kont.Return[int](x)
		}),
	)
	for b.Loop() {
		_ = kont.Run(m)
	}
}

// BenchmarkRunError measures Error effect handler (success path).
func BenchmarkRunError(b *testing.B) {
	computation := kont.Return[kont.Resumed](42)
	for b.Loop() {
		_ = kont.RunError[string, int](computation)
	}
}

// BenchmarkThrowCatch measures Error effect with Throw and Catch.
func BenchmarkThrowCatch(b *testing.B) {
	computation := kont.CatchError[string](
		kont.ThrowError[string, int]("err"),
		func(e string) kont.Cont[kont.Resumed, int] {
			return kont.Return[kont.Resumed](0)
		},
	)
	for b.Loop() {
		_ = kont.RunError[string, int](computation)
	}
}

// BenchmarkRunStateDirect measures the specialized RunState trampoline.
func BenchmarkRunStateDirect(b *testing.B) {
	computation := kont.GetState(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.PutState(x+1, kont.Perform(kont.Get[int]{}))
	})

	for b.Loop() {
		_, _ = kont.RunState[int, int](0, computation)
	}
}

// BenchmarkRunReaderDirect measures the specialized RunReader trampoline.
func BenchmarkRunReaderDirect(b *testing.B) {
	computation := kont.AskReader(func(x int) kont.Cont[kont.Resumed, int] {
		return kont.AskReader(func(y int) kont.Cont[kont.Resumed, int] {
			return kont.Return[kont.Resumed](x + y)
		})
	})

	for b.Loop() {
		_ = kont.RunReader[int, int](21, computation)
	}
}

// BenchmarkRunWriterDirect measures the specialized RunWriter trampoline.
func BenchmarkRunWriterDirect(b *testing.B) {
	computation := kont.TellWriter(1, kont.TellWriter(2, kont.Perform(kont.Tell[int]{Value: 3})))

	for b.Loop() {
		_, _ = kont.RunWriter[int, struct{}](computation)
	}
}

// BenchmarkRunStateExprDirect measures the Expr State runner with Get+Put cycle.
func BenchmarkRunStateExprDirect(b *testing.B) {
	computation := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(x int) kont.Expr[int] {
		return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: x + 1}), kont.ExprPerform(kont.Get[int]{}))
	})

	for b.Loop() {
		_, _ = kont.RunStateExpr[int, int](0, computation)
	}
}

// BenchmarkRunReaderExprDirect measures the Expr Reader runner with Ask+Ask chain.
func BenchmarkRunReaderExprDirect(b *testing.B) {
	computation := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(x int) kont.Expr[int] {
		return kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(y int) kont.Expr[int] {
			return kont.ExprReturn(x + y)
		})
	})

	for b.Loop() {
		_ = kont.RunReaderExpr[int, int](21, computation)
	}
}

// BenchmarkRunWriterExprDirect measures the Expr Writer runner with Tell chain.
func BenchmarkRunWriterExprDirect(b *testing.B) {
	computation := kont.ExprThen(kont.ExprPerform(kont.Tell[int]{Value: 1}),
		kont.ExprThen(kont.ExprPerform(kont.Tell[int]{Value: 2}),
			kont.ExprPerform(kont.Tell[int]{Value: 3})))

	for b.Loop() {
		_, _ = kont.RunWriterExpr[int, struct{}](computation)
	}
}

// BenchmarkRunErrorExprSuccess measures the Expr Error runner on the success path.
func BenchmarkRunErrorExprSuccess(b *testing.B) {
	computation := kont.ExprReturn[int](42)
	for b.Loop() {
		_ = kont.RunErrorExpr[string, int](computation)
	}
}

// BenchmarkRunErrorExprThrow measures the Expr Error runner on the throw path.
func BenchmarkRunErrorExprThrow(b *testing.B) {
	computation := kont.ExprThrowError[string, int]("err")
	for b.Loop() {
		_ = kont.RunErrorExpr[string, int](computation)
	}
}

// BenchmarkRunStateReaderExpr measures the composed Expr State+Reader runner.
func BenchmarkRunStateReaderExpr(b *testing.B) {
	comp := kont.ExprBind(kont.ExprPerform(kont.Ask[int]{}), func(env int) kont.Expr[int] {
		return kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + env}), kont.ExprPerform(kont.Get[int]{}))
		})
	})

	for b.Loop() {
		_, _ = kont.RunStateReaderExpr[int, int, int](0, 1, comp)
	}
}

// BenchmarkBracket measures resource acquisition pattern.
func BenchmarkBracket(b *testing.B) {
	acquire := kont.Return[kont.Resumed](42)
	release := func(_ int) kont.Cont[kont.Resumed, struct{}] {
		return kont.Return[kont.Resumed](struct{}{})
	}
	use := func(r int) kont.Cont[kont.Resumed, int] {
		return kont.Return[kont.Resumed](r * 2)
	}

	for b.Loop() {
		_ = kont.Handle(kont.Bracket[string](acquire, release, use),
			kont.HandleFunc[kont.Either[string, int]](func(_ kont.Operation) (kont.Resumed, bool) {
				panic("unreachable")
			}))
	}
}
