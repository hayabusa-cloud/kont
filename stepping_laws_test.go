// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"testing"

	"code.hybscloud.com/kont"
)

func TestSteppingReplyPrefixes(t *testing.T) {
	for _, entry := range []struct {
		name string
		step func(kont.Eff[int]) (int, *kont.Suspension[int])
	}{
		{"Step", kont.Step[int]},
		{"Reify", func(m kont.Eff[int]) (int, *kont.Suspension[int]) { return kont.StepExpr(kont.Reify(m)) }},
	} {
		t.Run(entry.name, func(t *testing.T) {
			// Each digit selects one of two typed replies, discard, or wait.
			for length, count := 0, 1; length <= 5; length, count = length+1, count*4 {
				for script := range count {
					checkReplyPrefix(t, entry.step, length, script)
				}
			}
		})
	}
}

func checkReplyPrefix(t *testing.T, step func(kont.Eff[int]) (int, *kont.Suspension[int]), length, script int) {
	t.Helper()
	calls := 0
	var loop func(int) kont.Eff[int]
	loop = func(sum int) kont.Eff[int] {
		return kont.GetState(func(more bool) kont.Eff[int] {
			calls++
			if !more {
				return kont.Pure(sum)
			}
			return kont.AskReader(func(delta int) kont.Eff[int] {
				calls++
				return loop(sum + delta)
			})
		})
	}

	starts := 0
	value, s := step(func(k func(int) kont.Resumed) kont.Resumed {
		starts++
		return loop(0)(k)
	})
	want, resumes := 0, 0
	for i, digits := 0, script; ; i, digits = i+1, digits/4 {
		if value != 0 || s == nil || starts != 1 || calls != resumes {
			t.Fatalf("length %d script %d step %d: value %d, pending %v, starts %d, calls %d, replies %d", length, script, i, value, s != nil, starts, calls, resumes)
		}
		isGet := resumes%2 == 0
		var op kont.Operation = kont.Get[bool]{}
		var reply kont.Resumed = false
		if !isGet {
			op, reply = kont.Ask[int]{}, 0
		}
		if got := s.Op(); got != op {
			t.Fatalf("length %d script %d step %d observed %T, want %T", length, script, i, got, op)
		}
		if i == length {
			s.Discard() // Clean up after observing the final pending operation.
			return
		}
		choice := digits % 4
		switch choice {
		case 2:
			s.Discard()
			if _, _, won := s.TryResume(reply); won || calls != resumes {
				t.Fatalf("discard at step %d invoked a callback", i)
			}
			return
		case 3:
			continue // The driver keeps the same handle until a reply is available.
		}
		if isGet {
			reply = choice == 1
		} else {
			delta := 2 + choice
			want += delta
			reply = delta
		}
		old := s
		value, s = old.Resume(reply)
		resumes++
		if _, _, won := old.TryResume(reply); won || starts != 1 || calls != resumes {
			t.Fatalf("reply at step %d: starts %d, calls %d, want 1 and %d", i, starts, calls, resumes)
		}
		if isGet && choice == 0 {
			if s != nil || value != want {
				t.Fatalf("stop at step %d returned %d, pending %v, want %d", i, value, s != nil, want)
			}
			return
		}
	}
}

func TestDiscardDuringResume(t *testing.T) {
	for _, path := range []struct {
		name  string
		start func(func(int) int) (int, *kont.Suspension[int])
	}{
		{"Cont", func(f func(int) int) (int, *kont.Suspension[int]) {
			return kont.Step(kont.Map(kont.Perform(kont.Get[int]{}), f))
		}},
		{"Reify", func(f func(int) int) (int, *kont.Suspension[int]) {
			return kont.StepExpr(kont.Reify(kont.Map(kont.Perform(kont.Get[int]{}), f)))
		}},
		{"Expr", func(f func(int) int) (int, *kont.Suspension[int]) {
			return kont.StepExpr(kont.ExprMap(kont.ExprPerform(kont.Get[int]{}), f))
		}},
	} {
		t.Run(path.name, func(t *testing.T) {
			entered, proceed := make(chan struct{}), make(chan struct{})
			_, s := path.start(func(v int) int {
				close(entered)
				<-proceed
				return v + 1
			})
			done := make(chan struct{})
			go func() {
				defer close(done)
				if v, next := s.Resume(41); v != 42 || next != nil {
					t.Errorf("winning resume returned (%d, %p)", v, next)
				}
			}()
			<-entered
			s.Discard()
			if _, _, won := s.TryResume(0); won {
				t.Error("another reply won while the callback was running")
			}
			close(proceed)
			<-done
		})
	}
}
