[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | [简体中文](README.zh-CN.md) | [Español](README.es.md) | **日本語** | [Français](README.fr.md)

# kont

F 有界多相を用いた Go 言語のための限定継続と代数的エフェクト。

## 概要

kont が提供するもの：
- 継続、制御フロー、エフェクトのための最小限だが完全なインターフェース
- コンパイル時ディスパッチと脱仮想化のための F 有界多相
- アロケーションフリーな評価ループを持つ脱関数化評価

### 理論的基盤

| 概念 | 参考文献 | 実装 |
|------|----------|------|
| 継続モナド | Moggi (1989) | `Cont[R, A]` |
| 限定継続 | Danvy & Filinski (1990) | `Shift`, `Reset` |
| 代数的エフェクト | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| アフィン型 | Walker & Watkins (2001) | `Affine[R, A]` |
| モナドの表現 | Filinski (1994) | `Reify`, `Reflect` |
| 脱関数化 | Reynolds (1972) | `Expr[A]`, `Frame` |

## インストール

```bash
go get code.hybscloud.com/kont
```

Go 1.26+ が必要です。

## コア型

| 型 | 用途 |
|----|------|
| `Cont[R, A]` | CPS 計算：`func(func(A) R) R` |
| `Eff[A]` | エフェクトフル計算: `Cont[Resumed, A]` の型エイリアス |
| `Pure` | 完全な型推論で値を `Eff` にリフト |
| `Expr[A]` | 脱関数化計算（アロケーションフリーな評価ループ） |
| `Shift`/`Reset` | 限定制御演算子 |
| `Op[O Op[O, A], A]` | F 有界エフェクト操作インターフェース |
| `Handler[H Handler[H, R], R]` | F 有界エフェクトハンドラインターフェース |
| `Either[E, A]` | エラー処理のための直和型 |
| `Affine[R, A]` | 一回限りの継続 |
| `Erased` | フレームチェーンの型消去された値を示す `any` の型エイリアス |
| `Reify`/`Reflect` | ブリッジ：Cont ↔ Expr（Filinski 1994） |

## 基本的な使い方

`kont` を初めて使う場合は、まず `Return`/`Bind`/`Run` で合成の流れをつかみ、次に標準エフェクトランナー（`State`、`Reader`、`Writer`、`Error`）へ進み、最後にアロケーション感度の高いホットパスや外部駆動ランタイム向けに `Expr`/`Step` API を使うのがおすすめです。

### Return と Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind（モナド合成）

```go
m := kont.Bind(
    kont.Return[int](21),
    func(x int) kont.Cont[int, int] {
        return kont.Return[int](x * 2)
    },
)
result := kont.Run(m) // 42
```

### Shift と Reset

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

## 標準エフェクト

### State（状態）

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader（リーダー）

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer（ライター）

```go
comp := kont.TellWriter("log message", kont.Pure(42))
result, logs := kont.RunWriter[string, int](comp)
```

### Error（エラー）

```go
comp := kont.CatchError[string, int](
    kont.ThrowError[string, int]("error"),
    func(err string) kont.Eff[int] {
        return kont.Pure(0)
    },
)
result := kont.RunError[string, int](comp)
```

## ステッピング

Step と StepExpr は外部ランタイム向けにエフェクトごとの逐次評価を提供します。

nil 完了規約：stepping 境界と effect runner は、nil の `Resumed` を「ゼロ値で完了」として扱います。
このため、最終結果型がポインタ型やインターフェース型の計算では、nil を意味のある結果値として使えません。
「nil で完了」と「ゼロ値で完了」を区別したい場合は、`Either` などの和型/Option に包んでください。

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // 保留中の操作を観察
    v := execute(op)        // 外部ランタイムが操作を処理
    result, susp = susp.Resume(v) // 次のサスペンションまで進行
}
// result は最終値
```

Expr 版：

```go
result, susp := kont.StepExpr(exprComputation)
```

各サスペンションは一回限り（アフィン）：Resume の再利用はパニックします。

## 複合エフェクト

複合ランナーは単一のハンドラから複数のエフェクトファミリをディスパッチします。

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error（エラー時も state は常に利用可能）
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

すべての複合ランナーには Expr 版があります（`RunStateReaderExpr`、`RunStateErrorExpr`、`RunStateWriterExpr`、`RunReaderStateErrorExpr`）。

## リソース安全性

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

## 脱関数化評価

クロージャをタグ付きフレームデータ構造に変換。反復トランポリン評価器がスタック成長なしにフレームを処理。評価ループはアロケーションフリー、フレーム構築時にはアロケーションが発生する場合があります。

### Return と Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind（モナドチェーン）

```go
c := kont.ExprReturn(10)
c = kont.ExprBind(c, func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### マルチステージパイプライン

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

### Then（結果を破棄する逐次実行）

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Expr エフェクト

Expr 計算は `HandleExpr` と専用ランナーを通じて同じ標準エフェクトをサポート。`ExprBind`/`ExprThen`/`ExprMap` と `ExprPerform` を直接合成：

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

### 直接フレーム構築

上級者向け：フレームチェーンを直接構築・評価：

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

## ブリッジ：Reify / Reflect

実行時に2つの表現を相互変換（Filinski 1994）。

```go
// Cont → Expr（クロージャがフレームに変換）
cont := kont.GetState(func(s int) kont.Eff[int] {
    return kont.Pure(s * 2)
})
expr := kont.Reify(cont)
result, state := kont.RunStateExpr[int, int](5, expr)

// Expr → Cont（フレームがクロージャに変換）
expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
    return kont.ExprReturn(s * 2)
})
cont := kont.Reflect(expr)
result, state := kont.RunState[int, int](5, cont)
```

往復変換はセマンティクスを保存：`Reify ∘ Reflect ≡ id` および `Reflect ∘ Reify ≡ id`。

## 参考文献

- Eugenio Moggi. "Computational Lambda-Calculus and Monads." In *LICS 1989*, pp. 14-23. https://doi.org/10.1109/LICS.1989.39155
- Olivier Danvy and Andrzej Filinski. "Abstracting Control." In *LISP and Functional Programming 1990*, pp. 151-160. https://doi.org/10.1145/91556.91622
- Andrzej Filinski. "Representing Monads." In *POPL 1994*, pp. 446-457. https://doi.org/10.1145/174675.178047
- Gordon D. Plotkin and Matija Pretnar. "Handlers of Algebraic Effects." In *ESOP 2009*, pp. 80-94. https://doi.org/10.1007/978-3-642-00590-9_7
- David Walker and Kevin Watkins. "On Regions and Linear Types (Extended Abstract)." In *ICFP 2001*, pp. 181-192. https://doi.org/10.1145/507635.507658
- John C. Reynolds. "Definitional Interpreters for Higher-Order Programming Languages." In *ACM '72*, pp. 717-740. https://doi.org/10.1145/800194.805852

## ライセンス

MIT ライセンス。詳細は [LICENSE](LICENSE) を参照。

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)
