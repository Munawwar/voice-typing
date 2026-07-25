package internal

import (
	"errors"
	"slices"
	"testing"
)

type recordingTyper struct {
	text       []rune
	backspaces int
	err        error
}

func (r *recordingTyper) typeText(text string) error {
	if r.err != nil {
		return r.err
	}
	r.text = append(r.text, []rune(text)...)
	return nil
}

func (r *recordingTyper) typeLineBreak() error {
	return r.typeText("\n")
}

func (r *recordingTyper) typeBackspaces(count int) error {
	if r.err != nil {
		return r.err
	}
	if count > len(r.text) {
		return errors.New("backspace exceeds typed text")
	}
	r.backspaces += count
	r.text = r.text[:len(r.text)-count]
	return nil
}

func TestVoiceCommandsRequireTheWholePhrase(t *testing.T) {
	for _, phrase := range []string{"please undo this migration", "undo last 0 words", "undo last 999999999999999999999 words"} {
		if command := parseVoiceCommand(phrase); command.kind != commandNone {
			t.Fatalf("%q parsed as command %+v", phrase, command)
		}
	}

	typer := &recordingTyper{}
	stack := NewTranscriptionStack(typer)
	stop, err := stack.addPhrase("We need to undo this migration.")
	if err != nil || stop {
		t.Fatalf("natural sentence treated as command: stop=%v err=%v", stop, err)
	}
	if got := string(typer.text); got != "We need to undo this migration." {
		t.Fatalf("typed %q", got)
	}

	stop, err = stack.addPhrase("stop-voice!")
	if err != nil || !stop {
		t.Fatalf("normalized stop command not recognized: stop=%v err=%v", stop, err)
	}
	if len(stack.phrases) != 1 {
		t.Fatalf("stop command changed phrases: %v", stack.phrases)
	}
}

func TestUndoUsesCharactersAndIncludesLineBreaks(t *testing.T) {
	typer := &recordingTyper{}
	stack := NewTranscriptionStack(typer)
	for _, phrase := range []string{"café", "newline", "world", "undo last 2 words"} {
		if stop, err := stack.addPhrase(phrase); err != nil || stop {
			t.Fatalf("addPhrase(%q): stop=%v err=%v", phrase, stop, err)
		}
	}
	if got := string(typer.text); got != "" {
		t.Fatalf("undo left %q", got)
	}
	if typer.backspaces != 10 {
		t.Fatalf("used %d backspaces, want 10 Unicode characters including newline", typer.backspaces)
	}
	if len(stack.phrases) != 0 {
		t.Fatalf("undo left phrases %v", stack.phrases)
	}
}

func TestInjectionFailureDoesNotCommitState(t *testing.T) {
	typer := &recordingTyper{err: errors.New("injection failed")}
	stack := NewTranscriptionStack(typer)
	if _, err := stack.addPhrase("hello"); err == nil {
		t.Fatal("expected injection error")
	}
	if len(stack.phrases) != 0 {
		t.Fatalf("failed text was committed: %v", stack.phrases)
	}

	typer.err = nil
	if _, err := stack.addPhrase("hello"); err != nil {
		t.Fatal(err)
	}
	typer.err = errors.New("injection failed")
	if _, err := stack.addPhrase("undo that"); err == nil {
		t.Fatal("expected undo error")
	}
	if !slices.Equal(stack.phrases, []string{"hello"}) {
		t.Fatalf("failed undo changed phrases: %v", stack.phrases)
	}
}
