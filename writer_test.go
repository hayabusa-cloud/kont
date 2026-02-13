// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont_test

import (
	"slices"
	"testing"

	"code.hybscloud.com/kont"
)

func TestWriterTell(t *testing.T) {
	comp := kont.TellWriter("hello", kont.TellWriter("world", kont.Return[kont.Resumed](42)))

	result, logs := kont.RunWriter[string, int](comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
	if logs[0] != "hello" || logs[1] != "world" {
		t.Fatalf("got logs %v, want [hello world]", logs)
	}
}

func TestWriterExec(t *testing.T) {
	comp := kont.TellWriter("log1", kont.TellWriter("log2", kont.Return[kont.Resumed]("result")))

	logs := kont.ExecWriter[string, string](comp)
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
}

func TestWriterNoLogs(t *testing.T) {
	comp := kont.Return[kont.Resumed, int](42)

	result, logs := kont.RunWriter[string, int](comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 0 {
		t.Fatalf("got %d logs, want 0", len(logs))
	}
}

func TestWriterIntLogs(t *testing.T) {
	comp := kont.TellWriter(1, kont.TellWriter(2, kont.TellWriter(3, kont.Return[kont.Resumed](6))))

	result, logs := kont.RunWriter[int, int](comp)
	if result != 6 {
		t.Fatalf("got result %d, want 6", result)
	}
	if len(logs) != 3 {
		t.Fatalf("got %d logs, want 3", len(logs))
	}
	sum := 0
	for _, n := range logs {
		sum += n
	}
	if sum != 6 {
		t.Fatalf("sum of logs is %d, want 6", sum)
	}
}

func TestExprWriterTell(t *testing.T) {
	comp := kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "hello"}),
		kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "world"}),
			kont.ExprReturn(42)))

	result, logs := kont.RunWriterExpr[string, int](comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
	if logs[0] != "hello" || logs[1] != "world" {
		t.Fatalf("got logs %v, want [hello world]", logs)
	}
}

func TestExprWriterExec(t *testing.T) {
	comp := kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "log1"}),
		kont.ExprThen(kont.ExprPerform(kont.Tell[string]{Value: "log2"}),
			kont.ExprReturn("result")))

	_, logs := kont.RunWriterExpr[string, string](comp)
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
}

func TestExprWriterNoLogs(t *testing.T) {
	comp := kont.ExprReturn[int](42)

	result, logs := kont.RunWriterExpr[string, int](comp)
	if result != 42 {
		t.Fatalf("got result %d, want 42", result)
	}
	if len(logs) != 0 {
		t.Fatalf("got %d logs, want 0", len(logs))
	}
}

func TestExprWriterIntLogs(t *testing.T) {
	comp := kont.ExprThen(kont.ExprPerform(kont.Tell[int]{Value: 1}),
		kont.ExprThen(kont.ExprPerform(kont.Tell[int]{Value: 2}),
			kont.ExprThen(kont.ExprPerform(kont.Tell[int]{Value: 3}),
				kont.ExprReturn(6))))

	result, logs := kont.RunWriterExpr[int, int](comp)
	if result != 6 {
		t.Fatalf("got result %d, want 6", result)
	}
	if len(logs) != 3 {
		t.Fatalf("got %d logs, want 3", len(logs))
	}
	sum := 0
	for _, n := range logs {
		sum += n
	}
	if sum != 6 {
		t.Fatalf("sum of logs is %d, want 6", sum)
	}
}

func TestWriterChained(t *testing.T) {
	// Multiple tells in a row
	comp := kont.TellWriter("a", kont.TellWriter("b", kont.TellWriter("c", kont.Return[kont.Resumed](struct{}{}))))

	_, logs := kont.RunWriter[string, struct{}](comp)
	if len(logs) != 3 {
		t.Fatalf("got %d logs, want 3", len(logs))
	}
	expected := []string{"a", "b", "c"}
	for i, log := range slices.All(logs) {
		if log != expected[i] {
			t.Fatalf("log[%d] = %q, want %q", i, log, expected[i])
		}
	}
}

// TestListenWriterWithConcreteType tests that Listen works with concrete type parameters.
// This validates the dispatch pattern fix: Listen[W, A] for any A now implements
// writerOp[W], fixing the type switch limitation where case Listen[W, any] wouldn't
// match Listen[W, int].
func TestListenWriterWithConcreteType(t *testing.T) {
	// Inner computation returns int (concrete type)
	inner := kont.TellWriter("inner-log", kont.Return[kont.Resumed](42))

	// Listen observes the inner computation's output
	comp := kont.TellWriter("outer-before",
		kont.Bind(
			kont.ListenWriter[string, int](inner),
			func(pair kont.Pair[int, []string]) kont.Cont[kont.Resumed, kont.Pair[int, []string]] {
				return kont.TellWriter("outer-after", kont.Return[kont.Resumed](pair))
			},
		),
	)

	result, logs := kont.RunWriter[string, kont.Pair[int, []string]](comp)

	// Check result value
	if result.Fst != 42 {
		t.Fatalf("got result %d, want 42", result.Fst)
	}

	// Check listened output (only inner-log)
	if len(result.Snd) != 1 || result.Snd[0] != "inner-log" {
		t.Fatalf("listened output = %v, want [inner-log]", result.Snd)
	}

	// Check total logs (outer-before, inner-log, outer-after)
	if len(logs) != 3 {
		t.Fatalf("got %d logs, want 3: %v", len(logs), logs)
	}
	expected := []string{"outer-before", "inner-log", "outer-after"}
	for i, log := range slices.All(logs) {
		if log != expected[i] {
			t.Fatalf("log[%d] = %q, want %q", i, log, expected[i])
		}
	}
}

// TestCensorWriterWithConcreteType tests that Censor works with concrete type parameters.
// This validates the dispatch pattern fix for Censor[W, A].
func TestCensorWriterWithConcreteType(t *testing.T) {
	// Inner computation returns string (concrete type)
	inner := kont.TellWriter("secret", kont.TellWriter("password", kont.Return[kont.Resumed]("result")))

	// Censor redacts certain words
	redact := func(logs []string) []string {
		result := make([]string, len(logs))
		for i, log := range slices.All(logs) {
			if log == "secret" || log == "password" {
				result[i] = "[REDACTED]"
			} else {
				result[i] = log
			}
		}
		return result
	}

	comp := kont.TellWriter("before",
		kont.Bind(
			kont.CensorWriter[string, string](redact, inner),
			func(result string) kont.Cont[kont.Resumed, string] {
				return kont.TellWriter("after", kont.Return[kont.Resumed](result))
			},
		),
	)

	result, logs := kont.RunWriter[string, string](comp)

	// Check result value
	if result != "result" {
		t.Fatalf("got result %q, want %q", result, "result")
	}

	// Check logs are censored
	if len(logs) != 4 {
		t.Fatalf("got %d logs, want 4: %v", len(logs), logs)
	}
	expected := []string{"before", "[REDACTED]", "[REDACTED]", "after"}
	for i, log := range slices.All(logs) {
		if log != expected[i] {
			t.Fatalf("log[%d] = %q, want %q", i, log, expected[i])
		}
	}
}

// TestListenNestedWithConcreteTypes tests nested Listen with different concrete types.
func TestListenNestedWithConcreteTypes(t *testing.T) {
	// Innermost returns bool
	innermost := kont.TellWriter(1, kont.Return[kont.Resumed](true))

	// Middle returns Pair[bool, []int]
	middle := kont.ListenWriter[int, bool](innermost)

	// Outer returns Pair[Pair[bool, []int], []int]
	outer := kont.TellWriter(2,
		kont.Bind(
			middle,
			func(p kont.Pair[bool, []int]) kont.Cont[kont.Resumed, kont.Pair[bool, []int]] {
				return kont.TellWriter(3, kont.Return[kont.Resumed](p))
			},
		),
	)

	result, logs := kont.RunWriter[int, kont.Pair[bool, []int]](outer)

	// Check inner result
	if result.Fst != true {
		t.Fatalf("inner result = %v, want true", result.Fst)
	}

	// Check listened logs (only 1 from innermost)
	if len(result.Snd) != 1 || result.Snd[0] != 1 {
		t.Fatalf("listened = %v, want [1]", result.Snd)
	}

	// Check total logs [2, 1, 3]
	if len(logs) != 3 {
		t.Fatalf("logs = %v, want [2, 1, 3]", logs)
	}
}
