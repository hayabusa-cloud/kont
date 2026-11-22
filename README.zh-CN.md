[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Benchmarks](https://github.com/hayabusa-cloud/kont/actions/workflows/benchmarks.yml/badge.svg)](https://github.com/hayabusa-cloud/kont/actions/workflows/benchmarks.yml)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | **简体中文** | [Español](README.es.md) | [日本語](README.ja.md) | [Français](README.fr.md)

# kont

基于 F 有界多态的 Go 语言限界续体和代数效果。

## 概述

kont 提供：
- 用于续体、控制流和效果的最小但完整的接口
- 编译时分发和去虚拟化的 F 有界多态
- 以迭代方式求值显式续体帧

### 理论基础

| 概念 | 参考文献 | 实现 |
|------|----------|------|
| 续体单子 | Moggi (1989) | `Cont[R, A]` |
| 限界续体 | Danvy & Filinski (1990) | `Shift`, `Reset` |
| 代数效果 | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| 仿射类型 | Walker & Watkins (2001) | `Affine[R, A]` |
| 单子表示 | Filinski (1994) | `Reify`, `Reflect` |
| 去函数化 | Reynolds (1972) | `Expr[A]`, `Frame` |

## 安装

```bash
go get code.hybscloud.com/kont
```

需要 Go 1.27+。

## 核心类型

| 类型                            | 用途                               |
|-------------------------------|----------------------------------|
| `Cont[R, A]`                  | CPS 计算：`func(func(A) R) R`       |
| `Eff[A]`                      | 带效果的计算: `Cont[Resumed, A]` 的类型别名 |
| `Pure`                        | 以完全类型推断将值提升为 `Eff`               |
| `Expr[A]`                     | 使用显式续体帧的去函数化计算                      |
| `Shift`/`Reset`               | 限界控制运算符                          |
| `Op[O Op[O, A], A]`           | F 有界效果操作接口                       |
| `Handler[H Handler[H, R], R]` | F 有界效果处理器接口                      |
| `Either[E, A]`                | 用于错误处理的和类型                       |
| `Affine[R, A]`                | 一次性续体                            |
| `Erased`                      | 标记帧链中类型擦除值的 `any` 类型别名           |
| `Reify`/`Reflect`             | 桥接：Cont ↔ Expr（Filinski 1994）    |
| `StepIndex`                   | 步进索引解释的有限近似层级                    |

## 基本用法

如果你是第一次使用 `kont`，建议先用 `Return`/`Bind`/`Run` 熟悉组合方式，再使用标准效果运行器（`State`、`Reader`、`Writer`、`Error`），最后在分配敏感的热点路径或外部驱动运行时时切换到 `Expr`/`Step` API。

### Return 和 Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind（单子组合）

```go
m := kont.Bind(
    kont.Return[int](21),
    func(x int) kont.Cont[int, int] {
        return kont.Return[int](x * 2)
    },
)
result := kont.Run(m) // 42
```

### Shift 和 Reset

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

## 标准效果

### State（状态）

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader（读取器）

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer（写入器）

```go
comp := kont.TellWriter("log message", kont.Pure(42))
result, logs := kont.RunWriter[string, int](comp)
```

### Error（错误）

```go
comp := kont.CatchError[string, int](
    kont.ThrowError[string, int]("error"),
    func(err string) kont.Eff[int] {
        return kont.Pure(0)
    },
)
result := kont.RunError[string, int](comp)
```

## 步进

Step 和 StepExpr 为外部运行时提供逐效果求值。`StepIndex` 是用于把该边界的有限前缀解释为步进索引模型的显式 fuel 见证；它不改变
`Step`、`StepExpr` 或仿射 `Suspension` 的运行时行为。

最终的 `Resumed` 为 `nil` 时，返回结果类型的零值。对于指针或接口，`nil` 是有效的结果。当 `Step` 或 `StepExpr` 返回的挂起句柄为 `nil` 时，计算已经完成，即使结果本身也是 `nil`。

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // 观察挂起的操作
    v := execute(op)        // 外部运行时处理操作
    result, susp = susp.Resume(v) // 推进到下一个挂起点
}
// result 是最终值
```

Expr 版本：

```go
result, susp := kont.StepExpr(exprComputation)
```

等待回复时，可以反复调用 `Op()` 查看待处理的操作。首次调用 `Resume`、`TryResume` 或 `Discard` 会消耗该挂起续体。此后再调用 `Resume` 会触发 panic。[步进与所有权证明](proofs/README.md)

## 复合效果

复合运行器从单个处理器分发多个效果族。

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error（即使出错，state 始终可用）
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

所有复合运行器都有 Expr 版本（`RunStateReaderExpr`、`RunStateErrorExpr`、`RunStateWriterExpr`、`RunReaderStateErrorExpr`）。

## 资源安全

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

## 去函数化求值

`Expr` 用帧表示续体。求值器以迭代方式遍历内置帧之间的链接，丢弃嵌套的 `Reify`/`Reflect` 链时也是如此。跨越嵌套桥接层恢复执行时，Go 调用栈可能增长。构建帧、执行回调、将值装箱或未命中对象池时，都可能发生堆分配。

### Return 和 Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind（单子链）

```go
c := kont.ExprBind(kont.ExprReturn(10), func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### 多阶段管道

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

### Then（丢弃结果的顺序执行）

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Expr 效果

Expr 计算通过 `HandleExpr` 和专用运行器支持相同的标准效果。直接组合 `ExprBind`/`ExprThen`/`ExprMap` 与 `ExprPerform`：

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

### 直接帧构建

高级用法：直接构建和求值帧链：

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

## 桥接：Reify / Reflect

`Reify(m)` 使用恒等续体运行 `m`。`StepExpr(Reify(m))` 与 `Step(m)` 观察到的第一个返回值或操作相同。如果 `Reify` 遇到操作，返回的 `Expr` 及其所有副本共享这一次执行。要开始新的执行，请对可复用的 `Cont` 再次调用 `Reify`。

```go
// Cont → Expr
cont := kont.GetState(func(s int) kont.Eff[int] {
	return kont.Pure(s * 2)
})
expr := kont.Reify(cont)
result, state := kont.RunStateExpr[int, int](5, expr)
```

`Reflect` 将表达式的最终值传给调用方的续体。传给 `Shift` 或 `Suspend` 的函数可以跳过续体、多次调用续体，或使用续体的返回值。在这些情况下，`Reflect(Reify(m))` 和 `m` 在 `Map` 或 `Bind` 中的行为可能不同。

```go
// Expr → Cont
expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
	return kont.ExprReturn(s * 2)
})
cont := kont.Reflect(expr)
result, state := kont.RunState[int, int](5, cont)
```

## 实用范式

典型的端到端模式将 `Expr` 计算、stepping API 与资源安全组合在一起：

```go
// 1. 构造一个会触发效果的去函数化计算。
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

// 2. 单步推进一次。若发生挂起，调用方拥有该一次性的 resume。
v, susp := kont.StepExpr[int](prog)
for susp != nil {
    // 3. 由外部驱动该挂起 —— 例如来自 proactor 循环 ——
    //    并以分发到的结果值恢复执行（仅一次）。
    v, susp = susp.Resume(handle(susp.Op()))
}
_ = v
```

当 `use` 正常返回或抛出类型匹配的 `Throw[E]` 时，`Bracket` 会调用 `release`。如果 Go 的 panic 从 `use` 向外传播，`Bracket` 会跳过 `release`。`acquire` 和 `release` 的效果由外层处理器解释。[Bracket 示例](#bracket)

```go
outcome, pending := kont.Step(kont.Bracket[error](acquire, release, use))
for pending != nil {
    outcome, pending = pending.Resume(handle(pending.Op()))
}
_ = outcome
```

外部运行时丢弃挂起时，负责释放仍由它持有的资源。`Discard` 释放续体帧，不会调用待执行的回调。

## 基准测试

在模块目录中以 `GOMAXPROCS=1` 运行基准测试。

```sh
go test -run='^$' -bench='^(BenchmarkWarmCycle|BenchmarkStepProtocol)$' -benchmem -cpu=1 -benchtime=200ms -count=10 -timeout=120s .
```

`WarmCycle` 每轮使用同一个预热后的池运行四种表达式，分别测量复用、构建并执行、处理器提前返回和丢弃挂起续体的开销。`StepProtocol` 每轮由单一所有者完成一次 `Get → Put → Get` 交互，分别使用手写 Go、`Step(cont)`、`StepExpr(Reify(cont))`，或对可复用的 `Expr` 调用 `StepExpr`。比较不同版本时，请保持机器、Go 版本和 GC 设置一致，并用 `benchstat` 比较输出。

## 参考文献

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

## 许可证

MIT 许可证。详见 [LICENSE](LICENSE)。

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)
