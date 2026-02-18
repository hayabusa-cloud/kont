// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"math/rand/v2"
	"testing"

	"code.hybscloud.com/kont"
)

const propertyN = 1000

// randInt returns a random int in [-1000, 1000].
func randInt(rng *rand.Rand) int {
	return rng.IntN(2001) - 1000
}

// randString returns a random ASCII string of length [0, 8].
func randString(rng *rand.Rand) string {
	n := rng.IntN(9)
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rng.IntN(95) + 32) // printable ASCII
	}
	return string(b)
}

// --- Group 1: Cont Monad Laws ---

// TestPropertyContLeftIdentity: Bind(Return(a), f) ≡ f(a)
func TestPropertyContLeftIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		f := func(x int) kont.Cont[int, int] { return kont.Return[int](x * 3) }
		left := kont.Run(kont.Bind(kont.Return[int](a), f))
		right := kont.Run(f(a))
		if left != right {
			t.Fatalf("left identity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// TestPropertyContRightIdentity: Bind(m, Return) ≡ m
func TestPropertyContRightIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.Return[int](a)
		left := kont.Run(kont.Bind(m, func(x int) kont.Cont[int, int] {
			return kont.Return[int](x)
		}))
		right := kont.Run(m)
		if left != right {
			t.Fatalf("right identity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// TestPropertyContAssociativity: Bind(Bind(m, f), g) ≡ Bind(m, func(x) Bind(f(x), g))
func TestPropertyContAssociativity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.Return[int](a)
		f := func(x int) kont.Cont[int, int] { return kont.Return[int](x + 3) }
		g := func(x int) kont.Cont[int, int] { return kont.Return[int](x * 2) }
		left := kont.Run(kont.Bind(kont.Bind(m, f), g))
		right := kont.Run(kont.Bind(m, func(x int) kont.Cont[int, int] {
			return kont.Bind(f(x), g)
		}))
		if left != right {
			t.Fatalf("associativity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// --- Group 2: Expr Monad Laws ---

// TestPropertyExprLeftIdentity: ExprBind(ExprReturn(a), f) ≡ f(a)
func TestPropertyExprLeftIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		f := func(x int) kont.Expr[int] { return kont.ExprReturn(x * 3) }
		left := kont.RunPure(kont.ExprBind(kont.ExprReturn(a), f))
		right := kont.RunPure(f(a))
		if left != right {
			t.Fatalf("expr left identity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// TestPropertyExprRightIdentity: ExprBind(m, ExprReturn) ≡ m
func TestPropertyExprRightIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.ExprReturn(a)
		left := kont.RunPure(kont.ExprBind(m, func(x int) kont.Expr[int] {
			return kont.ExprReturn(x)
		}))
		right := kont.RunPure(m)
		if left != right {
			t.Fatalf("expr right identity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// TestPropertyExprAssociativity: ExprBind(ExprBind(m, f), g) ≡ ExprBind(m, func(x) ExprBind(f(x), g))
func TestPropertyExprAssociativity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.ExprReturn(a)
		f := func(x int) kont.Expr[int] { return kont.ExprReturn(x + 3) }
		g := func(x int) kont.Expr[int] { return kont.ExprReturn(x * 2) }
		left := kont.RunPure(kont.ExprBind(kont.ExprBind(m, f), g))
		right := kont.RunPure(kont.ExprBind(m, func(x int) kont.Expr[int] {
			return kont.ExprBind(f(x), g)
		}))
		if left != right {
			t.Fatalf("expr associativity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// --- Group 3: Cont Functor Laws ---

// TestPropertyContFunctorIdentity: Map(m, id) ≡ m
func TestPropertyContFunctorIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.Return[int](a)
		left := kont.Run(kont.Map(m, func(x int) int { return x }))
		right := kont.Run(m)
		if left != right {
			t.Fatalf("cont functor identity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// TestPropertyContFunctorComposition: Map(m, f∘g) ≡ Map(Map(m, g), f)
func TestPropertyContFunctorComposition(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	f := func(x int) int { return x * 2 }
	g := func(x int) int { return x + 3 }
	fg := func(x int) int { return f(g(x)) }
	for range propertyN {
		a := randInt(rng)
		m := kont.Return[int](a)
		left := kont.Run(kont.Map(m, fg))
		right := kont.Run(kont.Map(kont.Map(m, g), f))
		if left != right {
			t.Fatalf("cont functor composition: %d != %d (a=%d)", left, right, a)
		}
	}
}

// --- Group 4: Expr Functor Laws ---

// TestPropertyExprFunctorIdentity: ExprMap(m, id) ≡ m
func TestPropertyExprFunctorIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.ExprReturn(a)
		left := kont.RunPure(kont.ExprMap(m, func(x int) int { return x }))
		right := kont.RunPure(m)
		if left != right {
			t.Fatalf("expr functor identity: %d != %d (a=%d)", left, right, a)
		}
	}
}

// TestPropertyExprFunctorComposition: ExprMap(m, f∘g) ≡ ExprMap(ExprMap(m, g), f)
func TestPropertyExprFunctorComposition(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	f := func(x int) int { return x * 2 }
	g := func(x int) int { return x + 3 }
	fg := func(x int) int { return f(g(x)) }
	for range propertyN {
		a := randInt(rng)
		m := kont.ExprReturn(a)
		left := kont.RunPure(kont.ExprMap(m, fg))
		right := kont.RunPure(kont.ExprMap(kont.ExprMap(m, g), f))
		if left != right {
			t.Fatalf("expr functor composition: %d != %d (a=%d)", left, right, a)
		}
	}
}

// --- Group 5: Bridge Round-Trip ---

// TestPropertyBridgeReflectReify: RunState(s, Reflect(Reify(cont))) ≡ RunState(s, cont)
func TestPropertyBridgeReflectReify(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		initial := randInt(rng)
		delta := randInt(rng)
		// Bind(Get, func(s) Then(Put(s+delta), Get))
		cont := kont.GetState[int, int](func(s int) kont.Eff[int] {
			return kont.PutState[int, int](s+delta, kont.Perform(kont.Get[int]{}))
		})
		leftVal, leftState := kont.RunState[int, int](initial, kont.Reflect(kont.Reify(cont)))
		rightVal, rightState := kont.RunState[int, int](initial, cont)
		if leftVal != rightVal || leftState != rightState {
			t.Fatalf("reflect∘reify: (%d,%d) != (%d,%d) (init=%d delta=%d)",
				leftVal, leftState, rightVal, rightState, initial, delta)
		}
	}
}

// TestPropertyBridgeReifyReflect: RunStateExpr(s, Reify(Reflect(expr))) ≡ RunStateExpr(s, expr)
func TestPropertyBridgeReifyReflect(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		initial := randInt(rng)
		delta := randInt(rng)
		// ExprBind(ExprPerform(Get), func(s) ExprThen(ExprPerform(Put{s+delta}), ExprPerform(Get)))
		expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
			return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + delta}), kont.ExprPerform(kont.Get[int]{}))
		})
		leftVal, leftState := kont.RunStateExpr[int, int](initial, kont.Reify(kont.Reflect(expr)))
		rightVal, rightState := kont.RunStateExpr[int, int](initial, expr)
		if leftVal != rightVal || leftState != rightState {
			t.Fatalf("reify∘reflect: (%d,%d) != (%d,%d) (init=%d delta=%d)",
				leftVal, leftState, rightVal, rightState, initial, delta)
		}
	}
}

// --- Group 6: Handler Coherence ---

// TestPropertyHandlerCoherence: same program gives identical result via RunState vs RunStateExpr
func TestPropertyHandlerCoherence(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		initial := randInt(rng)
		delta := randInt(rng)
		// Bind(Get, func(s) Then(Put(s+delta), Get))
		cont := kont.GetState[int, int](func(s int) kont.Eff[int] {
			return kont.PutState[int, int](s+delta, kont.Perform(kont.Get[int]{}))
		})
		expr := kont.Reify(cont)
		contVal, contState := kont.RunState[int, int](initial, cont)
		exprVal, exprState := kont.RunStateExpr[int, int](initial, expr)
		if contVal != exprVal || contState != exprState {
			t.Fatalf("handler coherence: cont(%d,%d) != expr(%d,%d) (init=%d delta=%d)",
				contVal, contState, exprVal, exprState, initial, delta)
		}
	}
}

// --- Group 7: Either Monad Laws ---

// TestPropertyEitherLeftIdentity: FlatMapEither(Right(a), f) ≡ f(a)
func TestPropertyEitherLeftIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		f := func(x int) kont.Either[string, int] { return kont.Right[string](x * 3) }
		left := kont.FlatMapEither(kont.Right[string](a), f)
		right := f(a)
		lv, _ := left.GetRight()
		rv, _ := right.GetRight()
		if lv != rv {
			t.Fatalf("either left identity: %d != %d (a=%d)", lv, rv, a)
		}
	}
}

// TestPropertyEitherRightIdentity: FlatMapEither(m, Right) ≡ m
func TestPropertyEitherRightIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.Right[string](a)
		left := kont.FlatMapEither(m, func(x int) kont.Either[string, int] {
			return kont.Right[string](x)
		})
		lv, _ := left.GetRight()
		rv, _ := m.GetRight()
		if lv != rv {
			t.Fatalf("either right identity: %d != %d (a=%d)", lv, rv, a)
		}
	}
}

// TestPropertyEitherAssociativity: FlatMapEither(FlatMapEither(m, f), g) ≡ FlatMapEither(m, func(x) FlatMapEither(f(x), g))
func TestPropertyEitherAssociativity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		m := kont.Right[string](a)
		f := func(x int) kont.Either[string, int] { return kont.Right[string](x + 3) }
		g := func(x int) kont.Either[string, int] { return kont.Right[string](x * 2) }
		left := kont.FlatMapEither(kont.FlatMapEither(m, f), g)
		right := kont.FlatMapEither(m, func(x int) kont.Either[string, int] {
			return kont.FlatMapEither(f(x), g)
		})
		lv, _ := left.GetRight()
		rv, _ := right.GetRight()
		if lv != rv {
			t.Fatalf("either associativity: %d != %d (a=%d)", lv, rv, a)
		}
	}
}

// TestPropertyEitherLeftPropagation: FlatMapEither(Left(e), f) ≡ Left(e)
func TestPropertyEitherLeftPropagation(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		e := randString(rng)
		m := kont.Left[string, int](e)
		result := kont.FlatMapEither(m, func(x int) kont.Either[string, int] {
			return kont.Right[string](x * 2)
		})
		if result.IsRight() {
			t.Fatalf("left should propagate (e=%q)", e)
		}
		got, _ := result.GetLeft()
		if got != e {
			t.Fatalf("left propagation: %q != %q", got, e)
		}
	}
}

// --- Group 8: Either Functor Laws ---

// TestPropertyEitherFunctorIdentity: MapEither(e, id) ≡ e
func TestPropertyEitherFunctorIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range propertyN {
		a := randInt(rng)
		e := kont.Right[string](a)
		result := kont.MapEither(e, func(x int) int { return x })
		lv, _ := result.GetRight()
		rv, _ := e.GetRight()
		if lv != rv {
			t.Fatalf("either functor identity: %d != %d (a=%d)", lv, rv, a)
		}
	}
}

// TestPropertyEitherFunctorComposition: MapEither(e, f∘g) ≡ MapEither(MapEither(e, g), f)
func TestPropertyEitherFunctorComposition(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	f := func(x int) int { return x * 2 }
	g := func(x int) int { return x + 3 }
	fg := func(x int) int { return f(g(x)) }
	for range propertyN {
		a := randInt(rng)
		e := kont.Right[string](a)
		left := kont.MapEither(e, fg)
		right := kont.MapEither(kont.MapEither(e, g), f)
		lv, _ := left.GetRight()
		rv, _ := right.GetRight()
		if lv != rv {
			t.Fatalf("either functor composition: %d != %d (a=%d)", lv, rv, a)
		}
	}
}
