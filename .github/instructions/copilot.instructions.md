# kont — Copilot Instructions

Last line of defense. Report only severe bugs that slip through tests and human review.

## Do Not Report

- Style, naming, formatting, documentation
- Suggestions, micro-optimizations, refactoring ideas
- Allocation concerns (by design: `Cont` suspensions allocate; `Expr` evaluation loop does not)
- `panic("phantom")` in `OpResult()` methods (intentional phantom type marker)
- `unhandledEffect` panic calls in `Dispatch` methods (intentional fail-fast for unmatched ops)
- `panic("kont: ...")` in `evalFrames`/`RunPure` default cases (intentional unreachable guards)

## Domain Context

- `Cont[R, A]` is `func(func(A) R) R` — returning an `effectSuspension` as `Resumed` is normal control flow, not an error
- `effectMarker`, `bindMarker`, `thenMarker`, `mapMarker` all implement `effectSuspension` — this is the single dispatch interface, not a type hierarchy smell
- `result == nil` in `handleDispatch` means zero-value completion — this is intentional
- F-bounded `Op[O Op[O,A], A]` and `Handler[H Handler[H,R], R]` are correct recursive constraints, not infinite types
- Structural assertions like `op.(interface{ DispatchState(*S) (Resumed, bool) })` are the intended dispatch mechanism
