[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**English** | [简体中文](README.zh-CN.md) | [Español](README.es.md) | [日本語](README.ja.md) | [Français](README.fr.md)

# kont

Delimited continuations and algebraic effects for Go via F-bounded polymorphism.

## Overview

`kont` provides:
- Minimal but complete interfaces for continuations, control, and effects
- F-bounded polymorphism for compile-time dispatch and devirtualization
- Iterative evaluation of explicit continuation frames

### Theoretical Foundations

| Concept | Reference | Implementation |
|---------|-----------|----------------|
| Continuation Monad | Moggi (1989) | `Cont[R, A]` |
| Delimited Continuations | Danvy & Filinski (1990) | `Shift`, `Reset` |
| Algebraic Effects | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| Affine Types | Walker & Watkins (2001) | `Affine[R, A]` |
| Representing Monads | Filinski (1994) | `Reify`, `Reflect` |
| Defunctionalization | Reynolds (1972) | `Expr[A]`, `Frame` |

## Installation

```bash
go get code.hybscloud.com/kont
```

Requires Go 1.27+.

## Core Types

| Type                          | Purpose                                                        |
|-------------------------------|----------------------------------------------------------------|
| `Cont[R, A]`                  | CPS computation: `func(func(A) R) R`                           |
| `Eff[A]`                      | Effectful computation: type alias for `Cont[Resumed, A]`       |
| `Pure`                        | Lift a value into `Eff` with full type inference               |
| `Expr[A]`                     | Defunctionalized computation with explicit continuation frames |
| `Shift`/`Reset`               | Delimited control operators                                    |
| `Op[O Op[O, A], A]`           | F-bounded effect operation interface                           |
| `Handler[H Handler[H, R], R]` | F-bounded effect handler interface                             |
| `Either[E, A]`                | Sum type for error handling                                    |
| `Affine[R, A]`                | One-shot continuation                                          |
| `Erased`                      | Type alias for `any` marking type-erased frame values          |
| `Reify`/`Reflect`             | Bridge: Cont ↔ Expr (Filinski 1994)                            |
| `StepIndex`                   | Finite approximation level for step-indexed interpretations    |

## Basic Usage

If you are new to `kont`, start with `Return`/`Bind`/`Run` to learn composition, then adopt standard effect runners (`State`, `Reader`, `Writer`, `Error`), and finally move to `Expr`/`Step` APIs for allocation-sensitive hot paths or externally driven runtimes.

### Return and Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind (Monadic Composition)

```go
m := kont.Bind(
    kont.Return[int](21),
    func(x int) kont.Cont[int, int] {
        return kont.Return[int](x * 2)
    },
)
result := kont.Run(m) // 42
```

### Shift and Reset

```go
m := kont.Reset[int](
    kont.Bind(
        kont.Shift[int, int](func(k func(int) int) int {
            return k(1) + k(10)
        }),
        func(x int) kont.Cont[int, int] {
            return kont.Return[int](x * 2)
        },
    ),
)
result := kont.Run(m) // (1*2) + (10*2) = 22
```

## Standard Effects

### State

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer

```go
comp := kont.TellWriter("log message", kont.Pure(42))
result, logs := kont.RunWriter[string, int](comp)
```

### Error

```go
comp := kont.CatchError[string, int](
    kont.ThrowError[string, int]("error"),
    func(err string) kont.Eff[int] {
        return kont.Pure(0)
    },
)
result := kont.RunError[string, int](comp)
```

## Stepping

Step and StepExpr provide one-effect-at-a-time evaluation for external runtimes.
`StepIndex` is an explicit fuel witness for callers that interpret finite
prefixes of this boundary as a step-indexed model; it does not change the
runtime behavior of `Step`, `StepExpr`, or affine `Suspension`.

A nil final `Resumed` yields the zero value of the result type. A nil pointer or interface is a valid result. `Step` and `StepExpr` signal completion with a nil suspension, including when the result itself is nil.

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // observe pending operation
    v := execute(op)        // external runtime handles the operation
    result, susp = susp.Resume(v) // advance to next suspension
}
// result is the final value
```

Expr equivalent:

```go
result, susp := kont.StepExpr(exprComputation)
```

Use `Op()` to inspect the pending operation as often as needed. The first `Resume`, `TryResume`, or `Discard` consumes the suspension. A later `Resume` panics. [Stepping and ownership proofs](proofs/README.md)

## Composed Effects

Combined runners dispatch multiple effect families from a single handler.

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error (state always available, even on error)
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

All composed runners have Expr equivalents (`RunStateReaderExpr`, `RunStateErrorExpr`, `RunStateWriterExpr`, `RunReaderStateErrorExpr`).

## Resource Safety

### Bracket

```go
comp := kont.Bracket[error, *File, string](
    acquire,
    func(f *File) kont.Eff[struct{}] {
        f.Close()
        return kont.Pure(struct{}{})
    },
    func(f *File) kont.Eff[string] {
        return kont.Pure(f.ReadAll())
    },
)
```

### OnError

```go
comp := kont.OnError(riskyOp(), errorCleanup)
```

## Defunctionalized Evaluation

`Expr` represents continuations as frames. The evaluator follows built-in frame links iteratively, including when discarding nested `Reify`/`Reflect` chains. Resuming across nested bridges can use the Go call stack. Frame construction, callbacks, boxing, and pool misses can allocate.

### Return and Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind

```go
c := kont.ExprBind(kont.ExprReturn(10), func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### Multi-Stage Pipeline

```go
c := kont.ExprReturn(1)
c = kont.ExprBind(c, func(x int) kont.Expr[int] {
    return kont.ExprReturn(x + 1)
})
c = kont.ExprMap(c, func(x int) int { return x * 3 })
c = kont.ExprBind(c, func(x int) kont.Expr[int] {
    return kont.ExprMap(kont.ExprReturn(x), func(y int) int { return y + 10 })
})
result := kont.RunPure(c) // ((1+1)*3)+10 = 16
```

### Then

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Expr Effects

Expr computations support the same standard effects via `HandleExpr` and dedicated runners. Compose `ExprBind`/`ExprThen`/`ExprMap` with `ExprPerform` directly:

```go
// s := Get; Put(s+10); Get
comp := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
    return kont.ExprThen(kont.ExprPerform(kont.Put[int]{Value: s + 10}),
        kont.ExprPerform(kont.Get[int]{}))
})
result, state := kont.RunStateExpr[int, int](0, comp)
```

```go
// Reader
comp := kont.ExprBind(kont.ExprPerform(kont.Ask[string]{}), func(env string) kont.Expr[string] {
    return kont.ExprReturn(env)
})
result := kont.RunReaderExpr[string, string]("hello", comp)
```

```go
// Writer
comp := kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "log"}),
    kont.ExprReturn(42))
result, logs := kont.RunWriterExpr[string, int](comp)
```

```go
// Error
result := kont.RunErrorExpr[string, int](kont.ExprThrowError[string, int]("fail"))
// result.IsLeft() == true
```

### Direct Frame Construction

For advanced use, build and evaluate frame chains directly:

```go
expr := kont.Expr[int]{
    Value: 5,
    Frame: &kont.MapFrame[kont.Erased, kont.Erased]{
        F:    func(v kont.Erased) kont.Erased { return v.(int) * 10 },
        Next: kont.ReturnFrame{},
    },
}
result := kont.RunPure(expr) // 50
```

## Reify / Reflect

`Reify(m)` runs `m` with the identity continuation. `StepExpr(Reify(m))` observes the same first return or operation as `Step(m)`. If `Reify` reaches an operation, the returned `Expr` and its copies share one invocation. Call `Reify` again on a reusable `Cont` for another run.

```go
// Cont → Expr
cont := kont.GetState(func(s int) kont.Eff[int] {
	return kont.Pure(s * 2)
})
expr := kont.Reify(cont)
result, state := kont.RunStateExpr[int, int](5, expr)
```

`Reflect` passes the expression’s final value to the caller’s continuation. A `Shift` or `Suspend` callback can skip its continuation, call it more than once, or use the answer it returns. In those cases, `Reflect(Reify(m))` can behave differently from `m` inside `Map` or `Bind`.

```go
// Expr → Cont
expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
	return kont.ExprReturn(s * 2)
})
cont := kont.Reflect(expr)
result, state := kont.RunState[int, int](5, cont)
```

## Practical Recipes

A typical end-to-end pattern combines an `Expr` computation with the
stepping API and resource safety:

```go
// 1. Build a defunctionalized computation that performs an effect.
type Op struct {
    kont.Phantom[int]
    Seed int
}

prog := kont.ExprBind(
    kont.ExprReturn(0),
    func(seed int) kont.Expr[int] {
        return kont.ExprPerform(Op{Seed: seed})
    },
)

// 2. Step it once. If it suspends, the caller owns the affine resume.
v, susp := kont.StepExpr[int](prog)
for susp != nil {
    // 3. Drive the suspension externally — e.g. from a proactor loop —
    //    and resume it once with the dispatched value.
    v, susp = susp.Resume(handle(susp.Op()))
}
_ = v
```

After `use` returns or raises an `Error[E]`, `Bracket` runs `release`. Its `acquire`, `release`, and `use` arguments are effectful computations or functions returning them, as in the [Bracket example](#bracket).

```go
outcome, pending := kont.Step(kont.Bracket[error](acquire, release, use))
for pending != nil {
    outcome, pending = pending.Resume(handle(pending.Op()))
}
_ = outcome
```

An external runtime that discards a suspension takes responsibility for any resources it still holds. `Discard` releases continuation frames without invoking pending callbacks.

## Benchmarks

From the module directory, run the benchmarks with `GOMAXPROCS=1`.

```sh
go test -run='^$' -bench='^(BenchmarkWarmCycle|BenchmarkStepProtocol)$' -benchmem -cpu=1 -benchtime=200ms -count=10 -timeout=120s .
```

One `WarmCycle` iteration runs four expression shapes on a warm pool. It measures reuse, construction and execution, early handler return, and discard separately. One `StepProtocol` iteration completes a single-owner `Get → Put → Get` exchange using hand-written Go, `Step(cont)`, `StepExpr(Reify(cont))`, or `StepExpr` on a reusable `Expr`. To compare revisions, keep the machine, Go version, and GC settings fixed and compare the outputs with `benchstat`.

## References

- John C. Reynolds. 1972. Definitional Interpreters for Higher-Order Programming Languages. In *Proc. ACM Annual
  Conference (ACM '72)*. 717–740. https://doi.org/10.1145/800194.805852
- Eugenio Moggi. 1989. Computational Lambda-Calculus and Monads. In *Proc. 4th Annual Symposium on Logic in Computer
  Science (LICS '89)*. 14–23. https://doi.org/10.1109/LICS.1989.39155
- Olivier Danvy and Andrzej Filinski. 1990. Abstracting Control. In *Proc. 1990 ACM Conference on LISP and Functional
  Programming (LFP '90)*. 151–160. https://doi.org/10.1145/91556.91622
- Andrzej Filinski. 1994. Representing Monads. In *Proc. 21st ACM SIGPLAN-SIGACT Symposium on Principles of Programming
  Languages (POPL '94)*. 446–457. https://doi.org/10.1145/174675.178047
- David Walker and Kevin Watkins. 2001. On Regions and Linear Types (Extended Abstract). In *Proc. 6th ACM SIGPLAN
  International Conference on Functional Programming (ICFP '01)*. 181–192. https://doi.org/10.1145/507635.507658
- Gordon D. Plotkin and John Power. 2002. Notions of Computation Determine Monads. In *Proc. 5th International
  Conference on Foundations of Software Science and Computation Structures (FoSSaCS '02)*. LNCS 2303,
  342–356. https://doi.org/10.1007/3-540-45931-6_24
- Gordon D. Plotkin and Matija Pretnar. 2009. Handlers of Algebraic Effects. In *Proc. 18th European Symposium on
  Programming (ESOP '09)*. LNCS 5502, 80–94. https://doi.org/10.1007/978-3-642-00590-9_7
- Ohad Kammar, Sam Lindley, and Nicolas Oury. 2013. Handlers in Action. In *Proc. 18th ACM SIGPLAN International
  Conference on Functional Programming (ICFP '13)*. 145–158. https://doi.org/10.1145/2500365.2500590
- Gordon D. Plotkin and Matija Pretnar. 2013. Handling Algebraic Effects. *Logical Methods in Computer Science* 9, 4 (
  Dec. 2013), Paper 23, 36 pages. https://arxiv.org/abs/1312.1399
- Daniel Hillerström and Sam Lindley. 2018. Shallow Effect Handlers. In *Proc. 16th Asian Symposium on Programming
  Languages and Systems (APLAS '18)*. LNCS 11275,
  415–435. https://homepages.inf.ed.ac.uk/slindley/papers/shallow-extended.pdf
- Daniel Hillerström, Sam Lindley, and Robert Atkey. 2020. Effect Handlers via Generalised Continuations. *Journal of
  Functional Programming* 30 (2020), e5. https://bentnib.org/handlers-cps-journal.pdf
- Wenhao Tang and Sam Lindley. 2026. Rows and Capabilities as Modal Effects. In *Proc. 53rd ACM SIGPLAN Symposium on
  Principles of Programming Languages (POPL '26)*. https://arxiv.org/abs/2507.10301

## License

MIT License. See [LICENSE](LICENSE) for details.

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)
