package internal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type commandKind uint8

const (
	commandNone commandKind = iota
	commandUndoPhrase
	commandUndoWords
	commandNewline
	commandParagraph
	commandStop
)

type voiceCommand struct {
	kind  commandKind
	words int
}

type textTyper interface {
	typeText(string) error
	typeLineBreak() error
	typeBackspaces(int) error
}

var (
	commandSanitizeRegex = regexp.MustCompile(`[^a-z0-9\s]+`)
	undoWordsRegex       = regexp.MustCompile(`^undo(?: last)? (?:(\d+|one|two|three|four|five|six|seven|eight|nine|ten) )?words?$`)
	exactCommands        = map[string]commandKind{
		"undo": commandUndoPhrase, "undo that": commandUndoPhrase, "undo this": commandUndoPhrase,
		"delete that": commandUndoPhrase, "delete this": commandUndoPhrase,
		"newline": commandNewline, "new line": commandNewline, "next line": commandNewline, "line break": commandNewline,
		"next para": commandParagraph, "new para": commandParagraph, "next paragraph": commandParagraph,
		"new paragraph": commandParagraph, "paragraph break": commandParagraph,
		"end voice": commandStop, "end recording": commandStop, "stop recording": commandStop, "stop voice": commandStop,
	}
	writtenNumbers = map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	}
)

type TranscriptionStack struct {
	phrases      []string
	textInjector textTyper
}

func NewTranscriptionStack(injector textTyper) *TranscriptionStack {
	return &TranscriptionStack{textInjector: injector}
}

func parseVoiceCommand(sentence string) voiceCommand {
	normalized := strings.ToLower(strings.TrimSpace(sentence))
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.Join(strings.Fields(commandSanitizeRegex.ReplaceAllString(normalized, " ")), " ")
	if kind, found := exactCommands[normalized]; found {
		return voiceCommand{kind: kind}
	}

	matches := undoWordsRegex.FindStringSubmatch(normalized)
	if len(matches) == 0 {
		return voiceCommand{}
	}
	count := 1
	if matches[1] != "" {
		if parsed, err := strconv.Atoi(matches[1]); err == nil && parsed > 0 {
			count = parsed
		} else if written, found := writtenNumbers[matches[1]]; found {
			count = written
		} else {
			return voiceCommand{}
		}
	}
	return voiceCommand{kind: commandUndoWords, words: count}
}

func (ts *TranscriptionStack) addPhrase(phrase string) (bool, error) {
	command := parseVoiceCommand(phrase)
	switch command.kind {
	case commandStop:
		return true, nil
	case commandUndoPhrase:
		return false, ts.undo(0)
	case commandUndoWords:
		return false, ts.undo(command.words)
	case commandNewline, commandParagraph:
		count := 1
		if command.kind == commandParagraph {
			count = 2
		}
		for typed := 0; typed < count; typed++ {
			if err := ts.textInjector.typeLineBreak(); err != nil {
				if typed > 0 {
					ts.phrases = append(ts.phrases, strings.Repeat("\n", typed))
				}
				return false, fmt.Errorf("type line break: %w", err)
			}
		}
		ts.phrases = append(ts.phrases, strings.Repeat("\n", count))
		return false, nil
	}

	text := strings.TrimSpace(phrase)
	if text == "" {
		return false, nil
	}
	if len(ts.phrases) > 0 && ts.phrases[len(ts.phrases)-1] != "\n" && ts.phrases[len(ts.phrases)-1] != "\n\n" {
		text = " " + text
	}
	if err := ts.textInjector.typeText(text); err != nil {
		return false, fmt.Errorf("type text: %w", err)
	}
	ts.phrases = append(ts.phrases, text)
	return false, nil
}

func (ts *TranscriptionStack) undo(wordCount int) error {
	if len(ts.phrases) == 0 {
		return nil
	}
	if wordCount == 0 {
		removed := ts.phrases[len(ts.phrases)-1]
		if err := ts.textInjector.typeBackspaces(utf8.RuneCountInString(removed)); err != nil {
			return fmt.Errorf("undo phrase: %w", err)
		}
		ts.phrases = ts.phrases[:len(ts.phrases)-1]
		return nil
	}

	remaining := append([]string(nil), ts.phrases...)
	wordsRemoved, backspaces := 0, 0
	for wordsRemoved < wordCount && len(remaining) > 0 {
		last := remaining[len(remaining)-1]
		if last == "\n" || last == "\n\n" {
			backspaces += utf8.RuneCountInString(last)
			remaining = remaining[:len(remaining)-1]
			continue
		}

		words := strings.Fields(last)
		remove := min(wordCount-wordsRemoved, len(words))
		if remove == len(words) {
			wordsRemoved += remove
			backspaces += utf8.RuneCountInString(last)
			remaining = remaining[:len(remaining)-1]
			continue
		}

		updated := strings.Join(words[:len(words)-remove], " ")
		if strings.HasPrefix(last, " ") {
			updated = " " + updated
		}
		wordsRemoved += remove
		backspaces += utf8.RuneCountInString(last) - utf8.RuneCountInString(updated)
		remaining[len(remaining)-1] = updated
	}

	if backspaces == 0 {
		return nil
	}
	if err := ts.textInjector.typeBackspaces(backspaces); err != nil {
		return fmt.Errorf("undo %d words: %w", wordCount, err)
	}
	ts.phrases = remaining
	return nil
}
