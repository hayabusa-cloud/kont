[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/kont.svg)](https://pkg.go.dev/code.hybscloud.com/kont)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/kont)](https://goreportcard.com/report/github.com/hayabusa-cloud/kont)
[![Coverage Status](https://codecov.io/gh/hayabusa-cloud/kont/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/kont)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | [简体中文](README.zh-CN.md) | **Español** | [日本語](README.ja.md) | [Français](README.fr.md)

# kont

Continuaciones delimitadas y efectos algebraicos para Go mediante polimorfismo F-acotado.

## Descripción general

kont proporciona:

- Interfaces mínimas pero completas para continuaciones, control y efectos
- Polimorfismo F-acotado para despacho en tiempo de compilación y desvirtualización
- Evaluación desfuncionalizada con un bucle de evaluación sin asignaciones

### Fundamentos teóricos

| Concepto                   | Referencia               | Implementación                       |
|----------------------------|--------------------------|--------------------------------------|
| Mónada de continuación     | Moggi (1989)             | `Cont[R, A]`                         |
| Continuaciones delimitadas | Danvy & Filinski (1990)  | `Shift`, `Reset`                     |
| Efectos algebraicos        | Plotkin & Pretnar (2009) | `Op`, `Handler`, `Perform`, `Handle` |
| Tipos afines               | Walker & Watkins (2001)  | `Affine[R, A]`                       |
| Representación de mónadas  | Filinski (1994)          | `Reify`, `Reflect`                   |
| Desfuncionalización        | Reynolds (1972)          | `Expr[A]`, `Frame`                   |

## Instalación

```bash
go get code.hybscloud.com/kont
```

Requiere Go 1.26+.

## Tipos principales

| Tipo                          | Propósito                                                             |
|-------------------------------|-----------------------------------------------------------------------|
| `Cont[R, A]`                  | Computación CPS: `func(func(A) R) R`                                  |
| `Eff[A]`                      | Computación con efectos: alias de tipo para `Cont[Resumed, A]`        |
| `Pure`                        | Eleva un valor a `Eff` con inferencia de tipos completa               |
| `Expr[A]`                     | Computación desfuncionalizada (bucle de evaluación sin asignaciones)  |
| `Shift`/`Reset`               | Operadores de control delimitado                                      |
| `Op[O Op[O, A], A]`           | Interfaz F-acotada de operación de efecto                             |
| `Handler[H Handler[H, R], R]` | Interfaz F-acotada de manejador de efectos                            |
| `Either[E, A]`                | Tipo suma para manejo de errores                                      |
| `Affine[R, A]`                | Continuación de un solo uso                                           |
| `Erased`                      | Alias de tipo para `any` que marca valores con tipo borrado en marcos |
| `Reify`/`Reflect`             | Puente: Cont ↔ Expr (Filinski 1994)                                   |

## Uso básico

Si es su primera vez con `kont`, empiece con `Return`/`Bind`/`Run` para aprender la composición; después adopte los
ejecutores de efectos estándar (`State`, `Reader`, `Writer`, `Error`); por último, use las APIs `Expr`/`Step` para rutas
críticas sensibles a asignaciones o runtimes dirigidos desde fuera.

### Return y Run

```go
m := kont.Return[int](42)
result := kont.Run(m) // 42
```

### Bind (composición monádica)

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

## Efectos estándar

### State (estado)

```go
comp := kont.GetState(func(s int) kont.Eff[int] {
    return kont.PutState(s+10, kont.Perform(kont.Get[int]{}))
})
result, state := kont.RunState[int, int](0, comp)
```

### Reader (lector)

```go
comp := kont.AskReader(func(cfg Config) kont.Eff[string] {
    return kont.Pure(cfg.BaseURL)
})
result := kont.RunReader(config, comp)
```

### Writer (escritor)

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

## Paso a paso

`Step` y `StepExpr` proporcionan evaluación efecto a efecto para runtimes externos.

Convención de finalización con nil: la frontera de stepping y los ejecutores de efectos tratan
un `Resumed` nil como «completado con el valor cero». Esto implica que las computaciones cuyo
tipo de resultado final sea un puntero o una interfaz no pueden usar nil como valor de resultado
significativo; envuelva esos resultados en un tipo suma/option (por ejemplo, `Either`) si
necesita distinguirlos.

```go
result, susp := kont.Step(computation)
for susp != nil {
    op := susp.Op()        // observar la operación pendiente
    v := execute(op)        // el runtime externo gestiona la operación
    result, susp = susp.Resume(v) // avanzar a la siguiente suspensión
}
// result es el valor final
```

Equivalente Expr:

```go
result, susp := kont.StepExpr(exprComputation)
```

Cada suspensión es de un solo uso (afín): `Resume` entra en pánico si se reutiliza.

## Efectos compuestos

Los ejecutores compuestos despachan varias familias de efectos desde un único manejador.

```go
// State + Reader
result, state := kont.RunStateReader[int, string, int](0, "env", comp)

// State + Error (el estado sigue disponible incluso ante un error)
result, state := kont.RunStateError[int, string, int](0, comp) // result: Either[string, int]

// State + Writer
result, state, logs := kont.RunStateWriter[int, string, int](0, comp)

// Reader + State + Error
result, state := kont.RunReaderStateError[string, int, string, int]("env", 0, comp)
```

Todos los ejecutores compuestos disponen de equivalentes Expr (`RunStateReaderExpr`, `RunStateErrorExpr`,
`RunStateWriterExpr`, `RunReaderStateErrorExpr`).

## Seguridad de recursos

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

## Evaluación desfuncionalizada

Los closures se transforman en estructuras de marcos etiquetados. Un evaluador trampolín iterativo los procesa sin
crecimiento de pila. El bucle de evaluación no asigna memoria; la construcción de marcos sí puede asignar.

### Return y Map

```go
c := kont.ExprReturn(42)
c = kont.ExprMap(c, func(x int) int { return x * 2 })
result := kont.RunPure(c) // 84
```

### Bind (encadenamiento monádico)

```go
c := kont.ExprReturn(10)
c = kont.ExprBind(c, func(x int) kont.Expr[string] {
    return kont.ExprReturn(fmt.Sprintf("value=%d", x))
})
result := kont.RunPure(c) // "value=10"
```

### Tubería multietapa

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

### Then (secuenciación con descarte)

```go
first := kont.ExprReturn("ignored")
second := kont.ExprReturn(42)
c := kont.ExprThen(first, second)
result := kont.RunPure(c) // 42
```

### Efectos Expr

Las computaciones Expr soportan los mismos efectos estándar mediante `HandleExpr` y ejecutores dedicados. Componga
`ExprBind`/`ExprThen`/`ExprMap` directamente con `ExprPerform`:

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

### Construcción directa de marcos

Para uso avanzado: construir y evaluar cadenas de marcos directamente:

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

Convierte entre las dos representaciones en tiempo de ejecución (Filinski 1994).

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

La conversión de ida y vuelta preserva la semántica: `Reify ∘ Reflect ≡ id` y `Reflect ∘ Reify ≡ id`.

## Patrones prácticos

Un patrón típico de extremo a extremo combina una computación `Expr` con la API de stepping y la seguridad de recursos:

```go
// 1. Construir una computación desfuncionalizada que dispara un efecto.
prog := kont.ExprBind(
    kont.ExprReturn(0),
    func(seed int) kont.Expr[int] {
        return kont.ExprPerform[int](Op{Seed: seed})
    },
)

// 2. Avanzar un paso. Si suspende, el llamador posee el resume afín.
v, susp := kont.StepExpr[int](prog)
if susp != nil {
    // 3. Conducir la suspensión desde fuera —por ejemplo, desde un bucle proactor—
    //    y reanudarla una sola vez con el valor despachado.
    v = susp.Resume(handle(susp.Operation()))
}
_ = v
```

Para computaciones que poseen recursos, envuelva el cuerpo en `Bracket` para que `release` se ejecute en cada salida
terminal (éxito, lanzamiento o cortocircuito):

```go
prog := kont.Bracket(
    func() (handle, error) { return acquire() },
    func(h handle) error    { return h.Close() },
    func(h handle) kont.Eff[kont.Either[error, result]] {
        return useResource(h)
    },
)
```

Cada sección anterior (`Efectos estándar`, `Paso a paso`, `Seguridad de recursos`, `Evaluación desfuncionalizada`)
compone; el orden mostrado aquí —*construir con `Expr*`, ejecutar paso a paso desde fuera, envolver en `Bracket`*— es el
patrón fundamental que usan `takt` y `sess` para integrarse con runtimes proactor.

## Referencias

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

## Licencia

Licencia MIT. Consulte [LICENSE](LICENSE) para más detalles.

©2026 [Hayabusa Cloud Co., Ltd.](https://code.hybscloud.com)
