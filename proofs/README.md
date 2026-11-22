# Stepping and ownership

For a fresh algebraic computation evaluated at the same point, `Step(m)` and `StepExpr(Reify(m))` expose the same operations and final answer to a driver making the same typed decisions. This holds for every finite interaction prefix, including repeated observations while waiting for a reply.

| Theorem in [Stepping.lean](Stepping.lean) | Correspondence to Go |
|---|---|
| `first_observation`, `capture_transfers_once`, `saved_answer_reusable` | Both entry points call the body with the same final continuation. `Reify` saves a completed answer or stores the marker in a shared cell. `StepExpr` takes a captured marker with one atomic exchange. A later attempt through another copy finds the cell empty. [Bridge](../bridge.go) |
| `prefix_simulation`, `reify_step_prefix` | Direct stepping classifies the host answer. Stepping a fresh capture yields a suspension with the same operation and marker. Each successful reply goes through the same `Suspension.resume` path. [Stepping](../step.go) |
| `wait_keeps_pending` | A driver that has no reply can retain the suspension and call `Op()` again. The same operation and continuation are available on retry. [Suspension.Op](../step.go) |
| `discard_skips_callback` | A winning discard releases the owned continuation frames and skips the pending callback. [Suspension disposal](../step.go) · [Frame disposal](../pool.go) |
| `spent_rejects`, `first_attempt_owns` | In the modification order of a shared handle’s atomic counter, the first `Add(1)` returns one. Under the documented call bound, every later resume or discard loses. A successor suspension starts with its own counter. [Claim](../affine.go) · [Go atomic operations](https://pkg.go.dev/sync/atomic) |

The model starts at a host return or operation, with bare nil already decoded to the answer type’s zero value. The host supplies callbacks that reach the next observed boundary and replies of the declared type. Callers share the returned suspension pointer, and total attempts on that handle fit in a `uintptr`. Winning an atomic claim establishes ownership. Finishing the callback and retiring external I/O are separate events.

Lean checks the transition model. The source mapping above and the [Go trace tests](../stepping_laws_test.go) connect it to the implementation. Both entry points run every script of up to five decisions over two typed replies, wait and discard. A separate Go test shows that a losing discard can return while the winning callback is running.

From this directory, use the pinned Lean 4.35.0 toolchain to build and replay the proofs. The file prints the transitive axioms of each theorem.

```sh
lake build
lake check
```
