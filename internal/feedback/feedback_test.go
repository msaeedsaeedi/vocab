package feedback

import (
	"strings"
	"testing"
)

func TestPromptY(t *testing.T) {
	input := strings.NewReader("y\n")
	result, err := readFrom(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected true for 'y'")
	}
}

func TestPromptYes(t *testing.T) {
	input := strings.NewReader("yes\n")
	result, err := readFrom(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected true for 'yes'")
	}
}

func TestPromptN(t *testing.T) {
	input := strings.NewReader("n\n")
	result, err := readFrom(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected false for 'n'")
	}
}

func TestPromptNo(t *testing.T) {
	input := strings.NewReader("no\n")
	result, err := readFrom(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("expected false for 'no'")
	}
}

func TestPromptYCapital(t *testing.T) {
	input := strings.NewReader("Y\n")
	result, err := readFrom(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatal("expected true for 'Y'")
	}
}

func TestPromptInvalid(t *testing.T) {
	input := strings.NewReader("maybe\n")
	_, err := readFrom(input)
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestPromptEmpty(t *testing.T) {
	input := strings.NewReader("\n")
	_, err := readFrom(input)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
