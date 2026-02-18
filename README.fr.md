[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | [简体中文](README.zh-CN.md) | [Español](README.es.md) | [日本語](README.ja.md) | **Français**

# kont

Continuations delimitees et effets algebriques pour Go via polymorphisme F-borne.

## Presentation

kont fournit :
- Des interfaces minimales mais completes pour les continuations, le controle et les effets
- Polymorphisme F-borne pour la repartition a la compilation et la devirtualisation
- Evaluation defonctionnalisee avec une boucle d'evaluation sans allocation

### Fondements Theoriques

| Concept | Reference | Implementation |
|---------|-----------|----------------|
| Monade de Continuation | Moggi (1989) | `Cont[R, A]` |
| Continuations Delimitees | Danvy & Filinski (1990) | `Shift`, `Reset` |
| Effets Algebriques | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| Types Affines | Walker & Watkins (2001) | `Affine[R, A]` |
| Representation des Monades | Filinski (1994) | `Reify`, `Reflect` |
| Defonctionnalisation | Reynolds (1972) | `Expr[A]`, `Frame` |

## Installation

```bash
go get code.hybscloud.com/kont
```

Necessite Go 1.26+.

## Types Principaux

| Type | Objectif |
|------|----------|
| `Cont[R, A]` | Calcul CPS : `func(func(A) R) R` |
| `Eff[A]` | Calcul avec effets : alias de type pour `Cont[Resumed, A]` |
| `Pure` | Eleve une valeur dans `Eff` avec inference de types complete |
| `Expr[A]` | Calcul defonctionnalise (boucle d'evaluation sans allocation) |
| `Shift`/`Reset` | Operateurs de controle delimite |
| `Op[O Op[O, A], A]` | Interface d'operation d'effet F-bornee |
| `Handler[H Handler[H, R], R]` | Interface de gestionnaire d'effets F-bornee |
| `Either[E, A]` | Type somme pour la gestion des erreurs |
| `Affine[R, A]` | Continuation a usage unique |
| `Erased` | Alias de type pour `any` marquant les valeurs a type efface dans les cadres |
| `Reify`/`Reflect` | Pont : Cont ↔ Expr (Filinski 1994) |

## Utilisation de Base

### Return et Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind (Composition Monadique)

```go
m := kont.Bind(
    kont.Return[int](21),
    func(x int) kont.Cont[int, int] {
        return kont.Return[int](x * 2)
    },
)
result := kont.Run(m) // 42
```

### Shift et Reset

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

## Effets Standards

### State (Etat)

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader (Lecteur)

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer (Ecrivain)

```go
comp := kont.TellWriter("log message", kont.Pure(42))
result, logs := kont.RunWriter[string, int](comp)
```

### Error (Erreur)

```go
comp := kont.CatchError[string, int](
    kont.ThrowError[string, int]("error"),
    func(err string) kont.Eff[int] {
        return kont.Pure(0)
    },
)
result := kont.RunError[string, int](comp)
```

## Evaluation Pas a Pas

Step et StepExpr fournissent une evaluation effet-par-effet pour les runtimes externes.

Convention de completion avec nil : la frontiere de stepping et les runners d'effets traitent
un `Resumed` nil comme « termine avec la valeur zero ». Cela implique que les calculs dont le
type de resultat final est un pointeur ou une interface ne peuvent pas utiliser nil comme une
valeur de resultat significative ; encapsulez ces resultats dans un type somme/option (par ex.
`Either`) si vous devez les distinguer.

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // observer l'operation en attente
    v := execute(op)        // le runtime externe traite l'operation
    result, susp = susp.Resume(v) // avancer jusqu'a la prochaine suspension
}
// result est la valeur finale
```

Equivalent Expr :

```go
result, susp := kont.StepExpr(exprComputation)
```

Chaque suspension est a usage unique (affine) : Resume panique en cas de reutilisation.

## Effets Composes

Les executeurs composes distribuent plusieurs familles d'effets depuis un seul gestionnaire.

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error (l'etat est toujours disponible, meme en cas d'erreur)
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

Tous les executeurs composes ont des equivalents Expr (`RunStateReaderExpr`, `RunStateErrorExpr`, `RunStateWriterExpr`, `RunReaderStateErrorExpr`).

## Securite des Ressources

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

## Evaluation Defonctionnalisee

Les closures deviennent des structures de donnees de cadres etiquetes. Un evaluateur trampoline iteratif les traite sans croissance de pile. La boucle d'evaluation n'alloue pas ; la construction des cadres peut allouer.

### Return et Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind (Chainage Monadique)

```go
c := kont.ExprReturn(10)
c = kont.ExprBind(c, func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### Pipeline Multi-Etapes

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

### Then (Sequencement avec Abandon)

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Effets Expr

Les calculs Expr supportent les memes effets standards via `HandleExpr` et des executeurs dedies. Composer `ExprBind`/`ExprThen`/`ExprMap` avec `ExprPerform` directement :

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

### Construction Directe des Cadres

Utilisation avancee : construire et evaluer des chaines de cadres directement :

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

## Pont : Reify / Reflect

Convertir entre les deux representations a l'execution (Filinski 1994).

```go
// Cont → Expr (les closures deviennent des cadres)
cont := kont.GetState(func(s int) kont.Eff[int] {
    return kont.Pure(s * 2)
})
expr := kont.Reify(cont)
result, state := kont.RunStateExpr[int, int](5, expr)

// Expr → Cont (les cadres deviennent des closures)
expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
    return kont.ExprReturn(s * 2)
})
cont := kont.Reflect(expr)
result, state := kont.RunState[int, int](5, cont)
```

L'aller-retour preserve la semantique : `Reify ∘ Reflect ≡ id` et `Reflect ∘ Reify ≡ id`.

## Licence

Licence MIT. Voir [LICENSE](LICENSE) pour les details.

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)

## References

- E. Moggi. "Computational Lambda-Calculus and Monads." In *Proc. LICS*, 1989.
- O. Danvy and A. Filinski. "Abstracting Control." In *Proc. LFP*, 1990.
- A. Filinski. "Representing Monads." In *Proc. POPL*, 1994.
- G. D. Plotkin and M. Pretnar. "Handlers of Algebraic Effects." In *Proc. ESOP*, 2009.
- D. Walker and K. Watkins. "On Regions and Linear Types." In *Proc. ICFP*, 2001.
- J. C. Reynolds. "Definitional Interpreters for Higher-Order Programming Languages." In *Proc. ACM Annual Conference*, 1972.
