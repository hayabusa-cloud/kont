import Std

namespace Kont

variable {Op A S T : Type} {Reply : Op → Type}

/-- A suspended host computation carries the reply type of its operation. -/
structure Marker (Op : Type) (Reply : Op → Type) (S : Type) where
  op : Op
  resume : Reply op → S

abbrev Raw (Op : Type) (Reply : Op → Type) (A S : Type) := Sum A (Marker Op Reply S)

inductive Frontier (Op : Type) (Reply : Op → Type) (A S : Type) where
  | returned (value : A)
  | pending (op : Op) (next : Reply op → S)
  | rejected

def classify : Raw Op Reply A S → Frontier Op Reply A S
  | .inl a => .returned a
  | .inr m => .pending m.op m.resume

inductive Expr (Op : Type) (Reply : Op → Type) (A S : Type) where
  | saved (value : A)
  | capture (slot : Option (Marker Op Reply S))

def reify : Raw Op Reply A S → Expr Op Reply A S
  | .inl a => .saved a
  | .inr m => .capture (some m)

/-- All aliases read the same capture slot. The second component is its new state. -/
def take : Expr Op Reply A S → Frontier Op Reply A S × Expr Op Reply A S
  | .saved a => (.returned a, .saved a)
  | .capture none => (.rejected, .capture none)
  | .capture (some m) => (.pending m.op m.resume, .capture none)

theorem first_observation (raw : Raw Op Reply A S) :
    (take (reify raw)).1 = classify raw := by cases raw <;> rfl

theorem capture_transfers_once (m : Marker Op Reply S) :
    let first := take (Expr.capture (A := A) (some m))
    first.1 = .pending m.op m.resume ∧ take first.2 = (.rejected, first.2) := ⟨rfl, rfl⟩

theorem saved_answer_reusable (a : A) :
    take (Expr.saved (Op := Op) (Reply := Reply) (S := S) a) = (.returned a, .saved a) := rfl

inductive Decision (R : Type) where
  | resume (reply : R)
  | discard
  | wait

abbrev Driver (Op : Type) (Reply : Op → Type) := (op : Op) → Decision (Reply op)

inductive Event (Op : Type) (Reply : Op → Type) (A : Type) where
  | observed (op : Op)
  | waited
  | invoked (op : Op) (reply : Reply op)
  | returned (value : A)
  | discarded
  | rejected

/-- Waiting keeps the issued suspension. Only a reply evaluates the next host state. -/
def runPrefix (observe : S → Frontier Op Reply A S) :
    List (Driver Op Reply) → Frontier Op Reply A S → List (Event Op Reply A)
  | _, .returned a => [.returned a]
  | _, .rejected => [.rejected]
  | [], .pending op _ => [.observed op]
  | act :: rest, .pending op next => match act op with
    | .wait => .observed op :: .waited :: runPrefix observe rest (.pending op next)
    | .discard => [.observed op, .discarded]
    | .resume v => .observed op :: .invoked op v :: runPrefix observe rest (observe (next v))

theorem wait_keeps_pending (observe : S → Frontier Op Reply A S)
    (op : Op) (next : Reply op → S) (act : Driver Op Reply) (rest : List (Driver Op Reply))
    (waiting : act op = .wait) :
    runPrefix observe (act :: rest) (.pending op next) =
      .observed op :: .waited :: runPrefix observe rest (.pending op next) := by
  simp [runPrefix, waiting]

theorem discard_skips_callback (observe : S → Frontier Op Reply A S)
    (op : Op) (next : Reply op → S) (act : Driver Op Reply) (rest : List (Driver Op Reply))
    (stop : act op = .discard) :
    runPrefix observe (act :: rest) (.pending op next) = [.observed op, .discarded] := by
  simp [runPrefix, stop]

def mapResidual (embed : S → T) : Frontier Op Reply A S → Frontier Op Reply A T
  | .returned a => .returned a
  | .rejected => .rejected
  | .pending op next => .pending op (fun v => embed (next v))

/-- A single-step simulation preserves every finite interaction prefix. -/
theorem prefix_simulation (left : S → Frontier Op Reply A S) (right : T → Frontier Op Reply A T)
    (embed : S → T) (step : ∀ s, right (embed s) = mapResidual embed (left s))
    (acts : List (Driver Op Reply)) (f : Frontier Op Reply A S) :
    runPrefix right acts (mapResidual embed f) = runPrefix left acts f := by
  induction acts generalizing f with
  | nil => cases f <;> rfl
  | cons act rest ih =>
    cases f with
    | returned a => rfl
    | rejected => rfl
    | pending op next =>
      cases decision : act op with
      | resume v =>
        simpa [runPrefix, mapResidual, decision, step] using ih (left (next v))
      | discard => simp [mapResidual, runPrefix, decision]
      | wait =>
        simpa [runPrefix, mapResidual, decision] using ih (.pending op next)

inductive Cursor (Op : Type) (Reply : Op → Type) (A S : Type) where
  | entry (expr : Expr Op Reply A S)
  | running (s : S)

/-- Once an entry transfers its marker, successors go through the native classifier. -/
def observeCursor (host : S → Raw Op Reply A S) :
    Cursor Op Reply A S → Frontier Op Reply A (Cursor Op Reply A S)
  | .entry e => mapResidual Cursor.running (take e).1
  | .running s => mapResidual Cursor.running (classify (host s))

theorem reify_step_prefix (host : S → Raw Op Reply A S) (s : S) (acts : List (Driver Op Reply)) :
    runPrefix (observeCursor host) acts (observeCursor host (.entry (reify (host s)))) =
    runPrefix (fun x => classify (host x)) acts (classify (host s)) := by
  rw [observeCursor, first_observation]
  exact prefix_simulation _ _ Cursor.running (fun _ => rfl) acts (classify (host s))

inductive Attempt where | resume | discard
inductive Outcome where | invoke | release | reject

def win : Attempt → Outcome
  | .resume => .invoke
  | .discard => .release

/-- Attempts on one shared handle in the modification order of its atomic counter. -/
def outcomes (modulus count : Nat) : List Attempt → List Outcome
  | [] => []
  | use :: rest =>
    let n := (count + 1) % modulus
    (if n = 1 then win use else .reject) :: outcomes modulus n rest

theorem spent_rejects (modulus : Nat) (uses : List Attempt) (count : Nat)
    (spent : 0 < count) (bound : count + uses.length < modulus) :
    outcomes modulus count uses = uses.map (fun _ => Outcome.reject) := by
  induction uses generalizing count with
  | nil => rfl
  | cons use rest ih =>
    simp only [List.length_cons] at bound
    have small : count + 1 < modulus := by omega
    have late : count + 1 ≠ 1 := by omega
    simp only [outcomes, Nat.mod_eq_of_lt small, ite_eq_right late, List.map_cons]
    rw [ih (count + 1) (by omega) (by omega)]

theorem first_attempt_owns (modulus : Nat) (use : Attempt) (rest : List Attempt)
    (bound : (use :: rest).length < modulus) :
    outcomes modulus 0 (use :: rest) = win use :: rest.map (fun _ => Outcome.reject) := by
  simp only [List.length_cons] at bound
  have small : 1 < modulus := by omega
  simp [outcomes, Nat.mod_eq_of_lt small, spent_rejects modulus rest 1 (by omega) (by omega)]

#print axioms first_observation
#print axioms capture_transfers_once
#print axioms saved_answer_reusable
#print axioms wait_keeps_pending
#print axioms discard_skips_callback
#print axioms prefix_simulation
#print axioms reify_step_prefix
#print axioms spent_rejects
#print axioms first_attempt_owns

end Kont
