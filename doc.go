// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package kont provides continuation-passing style primitives and algebraic effects
// in Go.
//
// The core type [Cont] represents a computation that accepts a continuation
// and produces a final result. This encoding enables delimited control operators
// such as [Shift] and [Reset] for capturing and manipulating continuations.
//
// # Design Philosophy
//
// kont provides:
//   - Minimal but complete interfaces for continuations, control, and effects
//   - F-bounded polymorphism for compile-time dispatch and devirtualization
//   - Defunctionalized evaluation with allocation-free evaluation loops (construction may allocate)
//
// # F-Bounded Architecture
//
// The package uses Go 1.26 F-bounded polymorphism (type T[P T[P]]) as a core
// architectural principle. This enables:
//
//   - Compile-time knowledge of concrete types at monomorphization time
//   - Potential devirtualization of dispatch calls by the compiler
//   - Allocation-free trampoline loops for effect handling through typed dispatch
//
// Key F-bounded interfaces:
//
//   - [Op]: type Op[O Op[O, A], A any] — operations know their concrete type
//   - [Handler]: type Handler[H Handler[H, R], R any] — handlers know their concrete type
//
// # Core Operations
//
// Minimal monad operations:
//
//   - [Return]: Lift a pure value into a continuation
//   - [Bind]: Sequence two continuations
//
// Derived operations:
//
//   - [Map]: Apply a function to the result — equivalent to Bind(m, func(a) Return(f(a)))
//   - [Then]: Sequence, discarding first result — equivalent to Bind(m, func(_) n)
//
// Execution:
//
//   - [Suspend]: Create a continuation from a CPS function
//   - [Run]: Execute a continuation to obtain the result
//   - [RunWith]: Execute with a custom final handler
//
// # Delimited Control
//
//   - [Shift]: Capture the current continuation up to [Reset]
//   - [Reset]: Establish a delimiter for [Shift]
//
// # Stepping Boundary
//
// [Step] and [StepExpr] provide one-effect-at-a-time evaluation for external
// runtimes that drive computation asynchronously (e.g., event loops).
// Unlike [Handle]/[HandleExpr], which run a synchronous trampoline to completion,
// the stepping API yields control at each effect suspension.
//
// Nil completion convention: effect runners and stepping treat a nil [Resumed]
// value as “completed with the zero value”. This implies computations whose
// final result type is a pointer or interface cannot use nil as a meaningful
// result value; wrap such results in a sum type (e.g., [Either]) if you need to
// distinguish “completed with nil” from “completed with zero”.
//
//   - [Step]: Drive a [Cont] computation until it completes or suspends
//   - [StepExpr]: Drive an [Expr] computation until it completes or suspends
//   - [Suspension]: Pending operation with one-shot resumption handle
//   - [Suspension.Op]: Returns the effect operation that caused the suspension
//   - [Suspension.Resume]: Advance to the next suspension or completion (panics on reuse)
//   - [Suspension.TryResume]: Non-panicking variant of Resume
//   - [Suspension.Discard]: Drop without invoking
//
// Returns (value, nil) on completion, or (zero, [*Suspension]) when pending.
// Affine semantics: each [Suspension] may be resumed at most once.
//
// # Algebraic Effects
//
// Effects are defined as types implementing the F-bounded [Op] constraint,
// and handlers interpret these effects via the F-bounded [Handler] interface.
// Handler dispatch returns (resumeValue, true) to continue the computation,
// or (finalResult, false) to short-circuit.
//
//   - [Op]: F-bounded effect operation interface
//   - [Operation]: Runtime type for effect operations
//   - [Resumed]: Runtime type for resumption values
//   - [Handler]: F-bounded effect interpreter interface
//   - [Perform]: Trigger an effect operation
//   - [Handle]: Run a computation with an F-bounded effect handler
//   - [HandleFunc]: Create a handler from a dispatch function
//
// # Standard Effects
//
// All standard handler constructors return concrete types to enable
// F-bounded inference. Operations implement dispatch methods (e.g. DispatchState)
// called through structural assertions in handlers.
//
// State effect for mutable state threading:
//
//   - [Get], [Put], [Modify]: Effect operations
//   - [GetState], [PutState], [ModifyState]: Fused convenience constructors (Cont)
//   - [StateHandler]: Creates a State handler (returns *stateHandler and state getter)
//   - [RunState], [EvalState], [ExecState]: Run with State effect (Cont)
//   - [RunStateExpr]: Run with State effect (Expr)
//
// Reader effect for read-only environment:
//
//   - [Ask]: Effect operation
//   - [AskReader], [MapReader]: Fused convenience constructors (Cont)
//   - [ReaderHandler]: Creates a Reader handler (returns *readerHandler)
//   - [RunReader]: Run with Reader effect (Cont)
//   - [RunReaderExpr]: Run with Reader effect (Expr)
//
// Writer effect for accumulating output:
//
//   - [WriterContext]: Shared context for writer dispatch
//   - [Tell], [Listen], [Censor]: Effect operations
//   - [TellWriter]: Fused convenience constructor (Cont, uses thenMarker)
//   - [ListenWriter], [CensorWriter]: Convenience wrappers (Cont, delegate to Perform)
//   - [WriterHandler]: Creates a Writer handler (returns *writerHandler and output getter)
//   - [RunWriter], [ExecWriter]: Run with Writer effect (Cont)
//   - [RunWriterExpr]: Run with Writer effect (Expr)
//   - [Pair]: Tuple type for Listen results
//
// Error effect for exception-like control flow:
//
//   - [Throw], [Catch]: Effect operations
//   - [ErrorContext]: Shared context for error dispatch
//   - [Throw].DispatchError: sets error in context, returns (struct{}{}, true)
//   - [Catch].DispatchError: runs body with RunError internally
//   - [ThrowError], [CatchError]: Convenience constructors (Cont)
//   - [ExprThrowError]: Throw constructor (Expr — direct EffectFrame, not composable from ExprPerform)
//   - [RunError]: Run with Error effect (Cont), returns [Either]
//   - [RunErrorExpr]: Run with Error effect (Expr), returns [Either]
//
// # Composed Effects
//
// Multi-effect handlers dispatch multiple effect families from a single handler.
// Combined runners eliminate handler-layer overhead for multi-effect hot paths.
//
// State + Reader:
//
//   - [RunStateReader]: Run with State + Reader (Cont)
//   - [RunStateReaderExpr]: Run with State + Reader (Expr)
//
// State + Error (state always available, even on error):
//
//   - [RunStateError]: Run with State + Error (Cont), returns ([Either], S)
//   - [EvalStateError]: Returns only the Either result
//   - [ExecStateError]: Returns only the final state
//   - [RunStateErrorExpr]: Run with State + Error (Expr)
//
// State + Writer:
//
//   - [RunStateWriter]: Run with State + Writer (Cont), returns (A, S, []W)
//   - [RunStateWriterExpr]: Run with State + Writer (Expr)
//
// Reader + State + Error:
//
//   - [RunReaderStateError]: Run with Reader + State + Error (Cont), returns ([Either], S)
//   - [RunReaderStateErrorExpr]: Run with Reader + State + Error (Expr)
//
// # Either Type
//
// [Either] represents success (Right) or failure (Left):
//
//   - [Left], [Right]: Constructors
//   - [Either.IsLeft], [Either.IsRight]: Predicates
//   - [Either.GetLeft], [Either.GetRight]: Accessors
//   - [MatchEither]: Pattern matching
//   - [MapEither]: Functor map over Right
//   - [FlatMapEither]: Monadic bind
//   - [MapLeftEither]: Transform Left value
//
// # Resource Safety
//
// Exception-safe resource management:
//
//   - [Bracket]: Acquire-release-use with guaranteed cleanup
//   - [OnError]: Run cleanup only on error
//
// # Affine Continuations
//
// [Affine] wraps a continuation with one-shot enforcement:
//
//   - [Once]: Create an affine continuation
//   - [Affine.Resume]: Invoke (panics on reuse)
//   - [Affine.TryResume]: Non-panicking variant
//   - [Affine.Discard]: Drop without invoking
//
// # Bridge: Reify / Reflect
//
// The two representations can be converted at runtime following
// Filinski (1994): reify converts semantic values to syntactic
// representations, and reflect is the inverse.
//
//   - [Reify]: Cont[Resumed, A] → Expr[A] (closures become frames)
//   - [Reflect]: Expr[A] → Cont[Resumed, A] (frames become closures)
//
// Conversion is lazy for effectful computations: each effect step is
// translated on demand during evaluation. Round-trip preserves semantics.
//
// # Defunctionalized Evaluation
//
// Defunctionalization (Reynolds 1972) enables allocation-free evaluation loops
// for continuation frames. Instead of closures, continuations are represented as tagged
// frame structures. The [Expr] type carries explicit frame data, unlike the
// closure-based [Cont] which tracks the answer type R at compile time.
//
// Type-erased values:
//
//   - [Erased]: Type alias for any, marking type-erased intermediate values
//     in the frame chain. Concrete types are recovered via type assertions
//     at frame boundaries. Frame type parameters use [Erased] (e.g.
//     BindFrame[Erased, Erased]) to document the type-erasure boundary.
//
// [Frame] is the marker interface for all frame types:
//
//   - [ReturnFrame]: Computation complete
//   - [BindFrame]: Monadic sequencing
//   - [MapFrame]: Functor transformation
//   - [ThenFrame]: Sequencing with discard
//   - [EffectFrame]: Suspended effect operation (carries [Operation] for dispatch)
//
// Constructors and combinators:
//
//   - [ExprReturn]: Create completed computation
//   - [ExprBind]: Sequence computations
//   - [ExprMap]: Transform result
//   - [ExprThen]: Sequence with discard
//   - [ExprPerform]: Perform an effect operation (creates [EffectFrame])
//   - [ExprSuspend]: Create suspended computation
//   - [ChainFrames]: Compose frame chains
//   - [RunPure]: Iteratively evaluate pure computation (panics on effects)
//   - [HandleExpr]: Evaluate with F-bounded effect handler
//
// # Example
//
//	type Ask[A any] struct{}
//	func (Ask[A]) OpResult() A { panic("phantom") }
//
//	comp := kont.Bind(
//		kont.Perform(Ask[int]{}),
//		func(x int) kont.Cont[kont.Resumed, int] {
//			return kont.Return[kont.Resumed](x * 2)
//		},
//	)
//
//	result := kont.Handle(comp, kont.HandleFunc[int](func(op kont.Operation) (kont.Resumed, bool) {
//		switch op.(type) {
//		case Ask[int]:
//			return 21, true // resume with 21
//		default:
//			panic("unhandled effect")
//		}
//	}))
//	// result == 42
package kont
