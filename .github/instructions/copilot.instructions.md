# kont — Copilot Review Instructions

Last line of defense. Report only severe bugs that slip through tests and human review.

## Do Not Report

- Style, naming, formatting, documentation
- Suggestions, micro-optimizations, refactoring ideas
- Allocation concerns (frame construction, callbacks, boxing, and pool misses can allocate)
- `panic("phantom")` in `OpResult()` methods (intentional phantom type marker)
- `unhandledEffect` panic calls in `Dispatch` methods (intentional fail-fast for unmatched ops)
- `panic("kont: ...")` in `evalFrames`/`RunPure` default cases (intentional unreachable guards)

## Domain Context

- A `Cont[Resumed, A]` can return a `*genericMarker` for a pending operation. `handleDispatch` resumes the marker or discards it when a handler supplies the final answer.
- `Handle` maps a bare nil answer to the zero result. The CPS error runners distinguish normal nil completion (`Right` of the zero value) from a handler that exits with nil (the zero `Either`, a `Left`).
- F-bounded `Op[O Op[O,A], A]` and `Handler[H Handler[H,R], R]` are correct recursive constraints, not infinite types
- Structural assertions like `op.(interface{ DispatchState(*S) (Resumed, bool) })` are the intended dispatch mechanism
