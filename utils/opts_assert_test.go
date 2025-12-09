package utils

import (
	"errors"
	"testing"
)

func TestExtractOptsValue_Success(t *testing.T) {
	opts := map[string]any{"a": 123, "b": "x"}
	gotInt, err := ExtractOptsValue[int]("a", opts)
	if err != nil {
		t.Fatal(err)
	}
	if gotInt != 123 {
		t.Fatalf("got=%d want=123", gotInt)
	}

	gotStr, err := ExtractOptsValue[string]("b", opts)
	if err != nil {
		t.Fatal(err)
	}
	if gotStr != "x" {
		t.Fatalf("got=%s want=x", gotStr)
	}
}

func TestExtractOptsValue_Errors(t *testing.T) {
	_, err := ExtractOptsValue[int]("a")
	if !errors.Is(err, OptsNIlErr) {
		t.Fatalf("want OptsNIlErr got=%v", err)
	}

	_, err = ExtractOptsValue[int]("a", map[string]any{})
	if !errors.Is(err, OptsInvalidKeyErr) {
		t.Fatalf("want OptsInvalidKeyErr got=%v", err)
	}

	_, err = ExtractOptsValue[int]("a", map[string]any{"a": "x"})
	if !errors.Is(err, OptsAssertTypeErr) {
		t.Fatalf("want OptsAssertTypeErr got=%v", err)
	}
}

func TestExtractOptsValueWithDefault_DefaultPaths(t *testing.T) {
	if got := ExtractOptsValueWithDefault[int]("a", 7, nil); got != 7 {
		t.Fatalf("nil opts default got=%d", got)
	}
	if got := ExtractOptsValueWithDefault[int]("a", 8, map[string]any{}); got != 8 {
		t.Fatalf("missing key default got=%d", got)
	}
	if got := ExtractOptsValueWithDefault[int]("a", 9, map[string]any{"a": "x"}); got != 9 {
		t.Fatalf("assert fail default got=%d", got)
	}

	if got := ExtractOptsValueWithDefault[int]("a", 0, map[string]any{"a": 5}); got != 5 {
		t.Fatalf("success got=%d want=5", got)
	}
	if got := ExtractOptsValueWithDefault[bool]("b", false, map[string]any{"b": true}); got != true {
		t.Fatalf("bool success got=%v want=true", got)
	}
}
