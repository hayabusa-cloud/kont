[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | [简体中文](README.zh-CN.md) | **Español** | [日本語](README.ja.md) | [Français](README.fr.md)

# kont

Continuaciones delimitadas y efectos algebraicos para Go via polimorfismo F-acotado.

## Descripcion General

kont proporciona:
- Interfaces minimas pero completas para continuaciones, control y efectos
- Polimorfismo F-acotado para despacho en tiempo de compilacion y desvirtualizacion
- Evaluacion desfuncionalizada con un bucle de evaluacion sin asignaciones

### Fundamentos Teoricos

| Concepto | Referencia | Implementacion |
|----------|------------|----------------|
| Monada de Continuacion | Moggi (1989) | `Cont[R, A]` |
| Continuaciones Delimitadas | Danvy & Filinski (1990) | `Shift`, `Reset` |
| Efectos Algebraicos | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| Tipos Afines | Walker & Watkins (2001) | `Affine[R, A]` |
| Representacion de Monadas | Filinski (1994) | `Reify`, `Reflect` |
| Desfuncionalizacion | Reynolds (1972) | `Expr[A]`, `Frame` |

## Instalacion

```bash
go get code.hybscloud.com/kont
```

Requiere Go 1.26+.

## Tipos Principales

| Tipo | Proposito |
|------|-----------|
| `Cont[R, A]` | Computacion CPS: `func(func(A) R) R` |
| `Eff[A]` | Computacion con efectos: alias de tipo para `Cont[Resumed, A]` |
| `Pure` | Eleva un valor a `Eff` con inferencia de tipos completa |
| `Expr[A]` | Computacion desfuncionalizada (bucle de evaluacion sin asignaciones) |
| `Shift`/`Reset` | Operadores de control delimitado |
| `Op[O Op[O, A], A]` | Interfaz de operacion de efecto F-acotada |
| `Handler[H Handler[H, R], R]` | Interfaz de manejador de efectos F-acotada |
| `Either[E, A]` | Tipo suma para manejo de errores |
| `Affine[R, A]` | Continuacion de un solo uso |
| `Erased` | Alias de tipo para `any` que marca valores con tipo borrado en marcos |
| `Reify`/`Reflect` | Puente: Cont ↔ Expr (Filinski 1994) |

## Uso Basico

Si es su primera vez con `kont`, empiece con `Return`/`Bind`/`Run` para aprender la composición, luego adopte los runners de efectos estándar (`State`, `Reader`, `Writer`, `Error`), y finalmente use las APIs `Expr`/`Step` para rutas críticas sensibles a asignaciones o runtimes dirigidos externamente.

### Return y Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind (Composicion Monadica)

```go
m := kont.Bind(
    kont.Return[int](21),
    func(x int) kont.Cont[int, int] {
        return kont.Return[int](x * 2)
    },
)
result := kont.Run(m) // 42
```

### Shift y Reset

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

## Efectos Estandar

### State (Estado)

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader (Lector)

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer (Escritor)

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

## Paso a Paso

Step y StepExpr proporcionan evaluacion efecto-por-efecto para runtimes externos.

Convencion de finalizacion con nil: el limite de stepping y los runners de efectos tratan
un `Resumed` nil como “completado con el valor cero”. Esto implica que las computaciones
con resultado final de tipo puntero o interfaz no pueden usar nil como un valor de resultado
significativo; envuelva estos resultados en un tipo suma/option (por ejemplo, `Either`) si
necesita distinguirlos.

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // observar operacion pendiente
    v := execute(op)        // el runtime externo maneja la operacion
    result, susp = susp.Resume(v) // avanzar a la siguiente suspension
}
// result es el valor final
```

Equivalente Expr:

```go
result, susp := kont.StepExpr(exprComputation)
```

Cada suspension es de un solo uso (afin): Resume entra en panic si se reutiliza.

## Efectos Compuestos

Los ejecutores compuestos despachan multiples familias de efectos desde un solo manejador.

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error (el estado siempre esta disponible, incluso en error)
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

Todos los ejecutores compuestos tienen equivalentes Expr (`RunStateReaderExpr`, `RunStateErrorExpr`, `RunStateWriterExpr`, `RunReaderStateErrorExpr`).

## Seguridad de Recursos

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

## Evaluacion Desfuncionalizada

Los closures se convierten en estructuras de datos de marcos etiquetados. Un evaluador trampolin iterativo los procesa sin crecimiento de pila. El bucle de evaluacion no asigna memoria; la construccion de marcos puede asignar.

### Return y Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind (Encadenamiento Monadico)

```go
c := kont.ExprReturn(10)
c = kont.ExprBind(c, func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### Pipeline Multi-Etapa

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

### Then (Secuenciacion con Descarte)

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Efectos Expr

Las computaciones Expr soportan los mismos efectos estandar via `HandleExpr` y ejecutores dedicados. Componer `ExprBind`/`ExprThen`/`ExprMap` con `ExprPerform` directamente:

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

### Construccion Directa de Marcos

Uso avanzado: construir y evaluar cadenas de marcos directamente:

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

## Puente: Reify / Reflect

Convierte entre las dos representaciones en tiempo de ejecucion (Filinski 1994).

```go
// Cont → Expr (los closures se convierten en marcos)
cont := kont.GetState(func(s int) kont.Eff[int] {
    return kont.Pure(s * 2)
})
expr := kont.Reify(cont)
result, state := kont.RunStateExpr[int, int](5, expr)

// Expr → Cont (los marcos se convierten en closures)
expr := kont.ExprBind(kont.ExprPerform(kont.Get[int]{}), func(s int) kont.Expr[int] {
    return kont.ExprReturn(s * 2)
})
cont := kont.Reflect(expr)
result, state := kont.RunState[int, int](5, cont)
```

La conversion ida y vuelta preserva la semantica: `Reify ∘ Reflect ≡ id` y `Reflect ∘ Reify ≡ id`.

## Referencias

- Eugenio Moggi. "Computational Lambda-Calculus and Monads." In *LICS 1989*, pp. 14-23. https://doi.org/10.1109/LICS.1989.39155
- Olivier Danvy and Andrzej Filinski. "Abstracting Control." In *LISP and Functional Programming 1990*, pp. 151-160. https://doi.org/10.1145/91556.91622
- Andrzej Filinski. "Representing Monads." In *POPL 1994*, pp. 446-457. https://doi.org/10.1145/174675.178047
- Gordon D. Plotkin and Matija Pretnar. "Handlers of Algebraic Effects." In *ESOP 2009*, pp. 80-94. https://doi.org/10.1007/978-3-642-00590-9_7
- David Walker and Kevin Watkins. "On Regions and Linear Types (Extended Abstract)." In *ICFP 2001*, pp. 181-192. https://doi.org/10.1145/507635.507658
- John C. Reynolds. "Definitional Interpreters for Higher-Order Programming Languages." In *ACM '72*, pp. 717-740. https://doi.org/10.1145/800194.805852

## Licencia

Licencia MIT. Ver [LICENSE](LICENSE) para detalles.

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)
