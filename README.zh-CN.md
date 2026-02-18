[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | **简体中文** | [Español](README.es.md) | [日本語](README.ja.md) | [Français](README.fr.md)

# kont

基于 F 有界多态的 Go 语言限界续体和代数效果。

## 概述

kont 提供：
- 用于续体、控制流和效果的最小但完整的接口
- 编译时分发和去虚拟化的 F 有界多态
- 具有无分配求值循环的去函数化求值

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

需要 Go 1.26+。

## 核心类型

| 类型 | 用途 |
|------|------|
| `Cont[R, A]` | CPS 计算：`func(func(A) R) R` |
| `Eff[A]` | 带效果的计算: `Cont[Resumed, A]` 的类型别名 |
| `Pure` | 以完全类型推断将值提升为 `Eff` |
| `Expr[A]` | 去函数化计算（无分配求值循环） |
| `Shift`/`Reset` | 限界控制运算符 |
| `Op[O Op[O, A], A]` | F 有界效果操作接口 |
| `Handler[H Handler[H, R], R]` | F 有界效果处理器接口 |
| `Either[E, A]` | 用于错误处理的和类型 |
| `Affine[R, A]` | 一次性续体 |
| `Erased` | 标记帧链中类型擦除值的 `any` 类型别名 |
| `Reify`/`Reflect` | 桥接：Cont ↔ Expr（Filinski 1994） |

## 基本用法

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

Step 和 StepExpr 为外部运行时提供逐效果求值。

nil 完成约定：stepping 边界与 effect runner 将 nil 的 `Resumed` 视为“以零值完成”。
因此，当最终结果类型是指针或接口时，无法把 nil 当作有意义的结果值。
如果需要区分“以 nil 完成”和“以零值完成”，请用和类型/Option（例如 `Either`）进行包装。

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

每个挂起点是一次性的（仿射）：重复调用 Resume 会 panic。

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

闭包转化为带标签的帧数据结构。迭代蹦床求值器无栈增长地处理帧。求值循环无分配；帧构建时可能产生分配。

### Return 和 Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind（单子链）

```go
c := kont.ExprReturn(10)
c = kont.ExprBind(c, func(x int) kont.Expr[string] {
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

在运行时转换两种表示（Filinski 1994）。

```go
// Cont → Expr（闭包转化为帧）
cont := kont.GetState(func(s int) kont.Eff[int] {
    return kont.Pure(s * 2)
})
expr := kont.Reify(cont)
result, state := kont.RunStateExpr[int, int](5, expr)

// Expr → Cont（帧转化为闭包）
expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
    return kont.ExprReturn(s * 2)
})
cont := kont.Reflect(expr)
result, state := kont.RunState[int, int](5, cont)
```

往返转换保持语义：`Reify ∘ Reflect ≡ id` 且 `Reflect ∘ Reify ≡ id`。

## 许可证

MIT 许可证。详见 [LICENSE](LICENSE)。

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)

## 参考文献

- E. Moggi. "Computational Lambda-Calculus and Monads." In *Proc. LICS*, 1989.
- O. Danvy and A. Filinski. "Abstracting Control." In *Proc. LFP*, 1990.
- A. Filinski. "Representing Monads." In *Proc. POPL*, 1994.
- G. D. Plotkin and M. Pretnar. "Handlers of Algebraic Effects." In *Proc. ESOP*, 2009.
- D. Walker and K. Watkins. "On Regions and Linear Types." In *Proc. ICFP*, 2001.
- J. C. Reynolds. "Definitional Interpreters for Higher-Order Programming Languages." In *Proc. ACM Annual Conference*, 1972.
