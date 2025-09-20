package internal

import (
	"regexp"
	"strconv"
	"strings"
)

type TranscriptionStack struct {
	phrases      []string
	concatenated string
	lastSentence string
	textInjector *TextInjector
}

func NewTranscriptionStack(injector *TextInjector) *TranscriptionStack {
	return &TranscriptionStack{
		phrases:      make([]string, 0),
		concatenated: "",
		textInjector: injector,
	}
}

func (ts *TranscriptionStack) AddPhrase(phrase string, realTimeTyping bool) {
	// Process voice commands first
	processedPhrase := ts.processVoiceCommands(phrase, realTimeTyping)

	if processedPhrase == nil {
		// Command was executed, don't add to phrases
		return
	}

	// Handle special formatting commands first
	if *processedPhrase == "\n" || *processedPhrase == "\n\n" {
		ts.phrases = append(ts.phrases, *processedPhrase)
		if realTimeTyping {
			if *processedPhrase == "\n" {
				ts.textInjector.TypeNewline()
			} else {
				ts.textInjector.TypeParagraphBreak()
			}
		}
		return // Don't add spacing logic for line breaks
	}

	// Add appropriate spacing for regular text
	var textWithSpacing string
	// Only add space if we have existing content AND the last thing wasn't a line break
	if len(ts.phrases) > 0 && !ts.lastWasLineBreak() {
		textWithSpacing = " " + *processedPhrase
	} else {
		// No space needed at start or after line breaks
		textWithSpacing = *processedPhrase
	}

	// Regular text
	ts.phrases = append(ts.phrases, textWithSpacing)
	ts.concatenated += textWithSpacing
	ts.lastSentence = *processedPhrase

	if realTimeTyping {
		ts.textInjector.TypeText(textWithSpacing)
	}
}

func (ts *TranscriptionStack) processVoiceCommands(sentence string, realTimeTyping bool) *string {
	sentenceLower := strings.ToLower(strings.TrimSpace(sentence))

	// Check for undo command
	if ts.containsKeywords(sentenceLower, []string{"undo that"}) {
		ts.handleUndoCommand(realTimeTyping)
		return nil
	}

	// Check for undo words commands
	if ts.isUndoWordsCommand(sentenceLower) {
		wordCount := ts.extractWordCountFromUndo(sentenceLower)
		ts.handleUndoWordsCommand(wordCount, realTimeTyping)
		return nil
	}

	// Check for newline command
	if ts.containsKeywords(sentenceLower, []string{"newline", "new line"}) {
		result := "\n"
		return &result
	}

	// Check for paragraph command
	if ts.containsKeywords(sentenceLower, []string{"next para", "new para", "next paragraph", "new paragraph"}) {
		result := "\n\n"
		return &result
	}

	// Check for stop commands
	if ts.containsKeywords(sentenceLower, []string{"end voice", "end recording", "stop recording", "stop voice"}) {
		// This will be handled by the main service
		return nil
	}

	// No command detected, return as regular text
	return &sentence
}

func (ts *TranscriptionStack) containsKeywords(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func (ts *TranscriptionStack) handleUndoCommand(realTimeTyping bool) {
	if len(ts.phrases) == 0 {
		return
	}

	removed := ts.phrases[len(ts.phrases)-1]
	ts.phrases = ts.phrases[:len(ts.phrases)-1]

	if realTimeTyping {
		if removed == "\n" {
			ts.textInjector.TypeKeyCombo([]string{"BackSpace"})
		} else if removed == "\n\n" {
			ts.textInjector.TypeKeyCombo([]string{"BackSpace"})
			ts.textInjector.TypeKeyCombo([]string{"BackSpace"})
		} else {
			ts.textInjector.TypeBackspaces(len(removed))
		}
	}

	// Rebuild concatenated string
	ts.rebuildConcatenated()
}

func (ts *TranscriptionStack) isUndoWordsCommand(sentence string) bool {
	return strings.Contains(sentence, "undo word") ||
		(strings.Contains(sentence, "undo last") && strings.Contains(sentence, "word"))
}

func (ts *TranscriptionStack) extractWordCountFromUndo(sentence string) int {
	if strings.Contains(sentence, "undo word") && !strings.Contains(sentence, "undo last") {
		return 1
	}

	// Look for patterns like "undo last 3 words"
	re := regexp.MustCompile(`undo last (\d+) word`)
	matches := re.FindStringSubmatch(sentence)
	if len(matches) > 1 {
		if count, err := strconv.Atoi(matches[1]); err == nil {
			return count
		}
	}

	// Written numbers
	wordNumbers := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	}

	for word, count := range wordNumbers {
		if strings.Contains(sentence, "undo last "+word+" word") {
			return count
		}
	}

	return 1
}

func (ts *TranscriptionStack) handleUndoWordsCommand(wordCount int, realTimeTyping bool) {
	if len(ts.phrases) == 0 {
		return
	}

	// Remove words from the transcription stack
	wordsRemoved := 0
	charactersToBackspace := 0 // Track characters to backspace

	for wordsRemoved < wordCount && len(ts.phrases) > 0 {
		lastPhrase := ts.phrases[len(ts.phrases)-1]

		// Skip line breaks when counting words
		if lastPhrase == "\n" || lastPhrase == "\n\n" {
			ts.phrases = ts.phrases[:len(ts.phrases)-1]
			continue
		}

		// Count words in the last phrase
		words := strings.Fields(strings.TrimSpace(lastPhrase))
		wordsToRemove := wordCount - wordsRemoved

		if len(words) <= wordsToRemove {
			// Remove entire phrase
			wordsRemoved += len(words)
			charactersToBackspace += len(lastPhrase) // Count all characters in removed phrase
			ts.phrases = ts.phrases[:len(ts.phrases)-1]
		} else {
			// Remove only some words from the last phrase
			remainingWords := words[:len(words)-wordsToRemove]

			if len(remainingWords) > 0 {
				// Update the phrase with remaining words
				newPhrase := strings.Join(remainingWords, " ")
				// Preserve leading space if original had it
				if strings.HasPrefix(lastPhrase, " ") && !strings.HasPrefix(newPhrase, " ") {
					newPhrase = " " + newPhrase
				}

				// Calculate characters to backspace (original length - new length)
				charactersToBackspace += len(lastPhrase) - len(newPhrase)

				ts.phrases[len(ts.phrases)-1] = newPhrase
			} else {
				// Remove entire phrase if no words left
				charactersToBackspace += len(lastPhrase)
				ts.phrases = ts.phrases[:len(ts.phrases)-1]
			}
			wordsRemoved = wordCount
		}
	}

	// Handle real-time typing
	if realTimeTyping {
		// Use precise character backspacing instead of word selection
		ts.textInjector.TypeBackspaces(charactersToBackspace)
	}

	// Rebuild concatenated string
	ts.rebuildConcatenated()
}

func (ts *TranscriptionStack) lastWasLineBreak() bool {
	if len(ts.phrases) == 0 {
		return false
	}
	lastPhrase := ts.phrases[len(ts.phrases)-1]
	return lastPhrase == "\n" || lastPhrase == "\n\n"
}

func (ts *TranscriptionStack) rebuildConcatenated() {
	textParts := make([]string, 0)
	for _, part := range ts.phrases {
		if part != "\n" && part != "\n\n" {
			textParts = append(textParts, part)
		}
	}
	ts.concatenated = strings.Join(textParts, "")
}

func (ts *TranscriptionStack) GetCurrentText() string {
	return ts.concatenated
}

func (ts *TranscriptionStack) Clear() {
	ts.phrases = make([]string, 0)
	ts.concatenated = ""
	ts.lastSentence = ""
}
