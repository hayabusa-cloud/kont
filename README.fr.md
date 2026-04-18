[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | [简体中文](README.zh-CN.md) | [Español](README.es.md) | [日本語](README.ja.md) | **Français**

# kont

Continuations délimitées et effets algébriques pour Go via polymorphisme F-borné.

## Présentation

kont fournit :

- Des interfaces minimales mais complètes pour les continuations, le contrôle et les effets
- Polymorphisme F-borné pour la répartition à la compilation et la dévirtualisation
- Évaluation défonctionnalisée avec une boucle d'évaluation sans allocation

### Fondements théoriques

| Concept                    | Référence                | Implémentation                       |
|----------------------------|--------------------------|--------------------------------------|
| Monade de continuation     | Moggi (1989)             | `Cont[R, A]`                         |
| Continuations délimitées   | Danvy & Filinski (1990)  | `Shift`, `Reset`                     |
| Effets algébriques         | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| Types affines              | Walker & Watkins (2001)  | `Affine[R, A]`                       |
| Représentation des monades | Filinski (1994)          | `Reify`, `Reflect`                   |
| Défonctionnalisation       | Reynolds (1972)          | `Expr[A]`, `Frame`                   |

## Installation

```bash
go get code.hybscloud.com/kont
```

Nécessite Go 1.26+.

## Types principaux

| Type                          | Rôle                                                                        |
|-------------------------------|-----------------------------------------------------------------------------|
| `Cont[R, A]`                  | Calcul CPS : `func(func(A) R) R`                                            |
| `Eff[A]`                      | Calcul avec effets : alias de type pour `Cont[Resumed, A]`                  |
| `Pure`                        | Élève une valeur dans `Eff` avec inférence de types complète                |
| `Expr[A]`                     | Calcul défonctionnalisé (boucle d'évaluation sans allocation)               |
| `Shift`/`Reset`               | Opérateurs de contrôle délimité                                             |
| `Op[O Op[O, A], A]`           | Interface d'opération d'effet F-bornée                                      |
| `Handler[H Handler[H, R], R]` | Interface de gestionnaire d'effets F-bornée                                 |
| `Either[E, A]`                | Type somme pour la gestion des erreurs                                      |
| `Affine[R, A]`                | Continuation à usage unique                                                 |
| `Erased`                      | Alias de type pour `any` marquant les valeurs à type effacé dans les cadres |
| `Reify`/`Reflect`             | Pont : Cont ↔ Expr (Filinski 1994)                                          |

## Utilisation de base

Si vous débutez avec `kont`, commencez par `Return`/`Bind`/`Run` pour apprendre la composition, puis adoptez les
exécuteurs d'effets standards (`State`, `Reader`, `Writer`, `Error`), et passez enfin aux APIs `Expr`/`Step` pour les
chemins critiques sensibles aux allocations ou les runtimes pilotés de l'extérieur.

### Return et Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind (composition monadique)

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

## Effets standards

### State (état)

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader (lecteur)

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer (écrivain)

```go
comp := kont.TellWriter("log message", kont.Pure(42))
result, logs := kont.RunWriter[string, int](comp)
```

### Error (erreur)

```go
comp := kont.CatchError[string, int](
    kont.ThrowError[string, int]("error"),
    func(err string) kont.Eff[int] {
        return kont.Pure(0)
    },
)
result := kont.RunError[string, int](comp)
```

## Évaluation pas à pas

`Step` et `StepExpr` fournissent une évaluation effet par effet pour les runtimes externes.

Convention de complétion par nil : la frontière de stepping et les exécuteurs d'effets traitent
un `Resumed` nil comme « terminé avec la valeur zéro ». Cela implique que les calculs dont le
type de résultat final est un pointeur ou une interface ne peuvent pas utiliser nil comme une
valeur de résultat significative ; encapsulez ces résultats dans un type somme/option (par ex.
`Either`) si vous devez les distinguer.

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // observer l'opération en attente
    v := execute(op)        // le runtime externe traite l'opération
    result, susp = susp.Resume(v) // avancer jusqu'à la prochaine suspension
}
// result est la valeur finale
```

Équivalent Expr :

```go
result, susp := kont.StepExpr(exprComputation)
```

Chaque suspension est à usage unique (affine) : Resume panique en cas de réutilisation.

## Effets composés

Les exécuteurs composés répartissent plusieurs familles d'effets depuis un unique gestionnaire.

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error (l'état reste disponible, même en cas d'erreur)
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

Tous les exécuteurs composés disposent d'équivalents Expr (`RunStateReaderExpr`, `RunStateErrorExpr`,
`RunStateWriterExpr`, `RunReaderStateErrorExpr`).

## Sécurité des ressources

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

## Évaluation défonctionnalisée

Les closures deviennent des structures de cadres étiquetés. Un évaluateur trampoline itératif les traite sans croissance
de pile. La boucle d'évaluation n'alloue pas ; la construction des cadres peut allouer.

### Return et Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind (chaînage monadique)

```go
c := kont.ExprReturn(10)
c = kont.ExprBind(c, func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### Pipeline multi-étapes

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

### Then (séquencement avec abandon)

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Effets Expr

Les calculs Expr supportent les mêmes effets standards via `HandleExpr` et des exécuteurs dédiés. Composez `ExprBind`/
`ExprThen`/`ExprMap` directement avec `ExprPerform` :

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

### Construction directe des cadres

Pour un usage avancé : construire et évaluer directement des chaînes de cadres :

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

Convertir entre les deux représentations à l'exécution (Filinski 1994).

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

L'aller-retour préserve la sémantique : `Reify ∘ Reflect ≡ id` et `Reflect ∘ Reify ≡ id`.

## Schémas pratiques

Un schéma typique de bout en bout combine un calcul `Expr` avec l'API de stepping et la sécurité des ressources :

```go
// 1. Construire un calcul défonctionnalisé qui déclenche un effet.
prog := kont.ExprBind(
    kont.ExprReturn(0),
    func(seed int) kont.Expr[int] {
        return kont.ExprPerform[int](Op{Seed: seed})
    },
)

// 2. Le faire avancer d'un pas. S'il suspend, l'appelant détient le resume affine.
v, susp := kont.StepExpr[int](prog)
if susp != nil {
    // 3. Piloter la suspension depuis l'extérieur — par ex. depuis une boucle proactor —
    //    et la reprendre une seule fois avec la valeur dispatchée.
    v = susp.Resume(handle(susp.Operation()))
}
_ = v
```

Pour les calculs qui possèdent des ressources, encadrez le corps avec `Bracket` afin que `release` s'exécute à chaque
sortie terminale (succès, exception ou court-circuit) :

```go
prog := kont.Bracket(
    func() (handle, error) { return acquire() },
    func(h handle) error    { return h.Close() },
    func(h handle) kont.Eff[kont.Either[error, result]] {
        return useResource(h)
    },
)
```

Chaque section ci-dessus (`Effets standards`, `Évaluation pas à pas`, `Sécurité des ressources`,
`Évaluation défonctionnalisée`) se compose ; l'ordre présenté ici — *construire avec `Expr*`, stepper depuis
l'extérieur, envelopper dans `Bracket`* — est l'unique schéma porteur utilisé par `takt` et `sess` pour s'intégrer aux
runtimes proactor.

## Références

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

## Licence

Licence MIT. Voir [LICENSE](LICENSE) pour les détails.

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)
