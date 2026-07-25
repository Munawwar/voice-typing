package internal

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

var (
	directToolOrder = map[string][]string{
		"wayland": {"wtype", "ydotool", "xdotool"},
		"x11":     {"xdotool", "wtype", "ydotool"},
		"unknown": {"xdotool", "wtype", "ydotool"},
	}
	comboToolOrder = map[string][]string{
		"wayland": {"ydotool", "wtype", "xdotool"},
		"x11":     {"xdotool", "ydotool", "wtype"},
		"unknown": {"xdotool", "ydotool", "wtype"},
	}
	ydotoolKeycodes = map[string]int{
		"BackSpace": 14,
		"Return":    28,
		"ctrl":      29,
		"v":         47,
		"shift":     42,
	}
	wtypeKeys = map[string]string{
		"BackSpace": "BackSpace",
		"Return":    "Return",
		"v":         "v",
	}
)

type TextInjector struct {
	displayServer  string
	availableTools map[string]bool
}

func NewTextInjector() *TextInjector {
	displayServer := "unknown"
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != "" {
		displayServer = "wayland"
	} else if os.Getenv("XDG_SESSION_TYPE") == "x11" || os.Getenv("DISPLAY") != "" {
		displayServer = "x11"
	}

	injector := &TextInjector{
		displayServer:  displayServer,
		availableTools: make(map[string]bool),
	}
	for _, tool := range []string{"xdotool", "ydotool", "wtype", "wl-copy", "wl-paste"} {
		if _, err := exec.LookPath(tool); err == nil {
			injector.availableTools[tool] = true
		}
	}
	return injector
}

func (ti *TextInjector) typeText(text string) error {
	if text == "" {
		return nil
	}
	var failures []error
	for _, tool := range directToolOrder[ti.displayServer] {
		if !ti.availableTools[tool] {
			continue
		}
		var command *exec.Cmd
		switch tool {
		case "xdotool":
			command = exec.Command(tool, "type", "--delay", "50", "--", text)
		case "wtype":
			command = exec.Command(tool, "--", text)
		case "ydotool":
			command = exec.Command(tool, "type", "--", text)
		}
		if err := command.Run(); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Errorf("%s: %w", tool, err))
		}
	}
	if len(failures) == 0 {
		failures = append(failures, fmt.Errorf("no direct typing tool is installed"))
	}

	var original string
	var err error
	if ti.displayServer == "wayland" && ti.availableTools["wl-paste"] {
		var output []byte
		output, err = exec.Command("wl-paste", "--no-newline").Output()
		original = string(output)
	} else {
		original, err = clipboard.ReadAll()
	}
	if err != nil {
		return errors.Join(append(failures, fmt.Errorf("read clipboard: %w", err))...)
	}
	if err := ti.writeClipboard(text); err != nil {
		return errors.Join(append(failures, fmt.Errorf("write clipboard: %w", err))...)
	}

	pasteErr := ti.typeKeyCombo([]string{"ctrl", "v"})
	time.Sleep(100 * time.Millisecond)
	if err := ti.writeClipboard(original); err != nil {
		log.Printf("Failed to restore clipboard: %v", err)
	}
	if pasteErr != nil {
		return errors.Join(append(failures, fmt.Errorf("paste clipboard: %w", pasteErr))...)
	}
	return nil
}

func (ti *TextInjector) typeKeyCombo(keys []string) error {
	var failures []error
	for _, tool := range comboToolOrder[ti.displayServer] {
		if !ti.availableTools[tool] {
			continue
		}
		command, err := keyComboCommand(tool, keys)
		if err == nil {
			err = command.Run()
		}
		if err == nil {
			return nil
		}
		failures = append(failures, fmt.Errorf("%s: %w", tool, err))
	}
	if len(failures) == 0 {
		return fmt.Errorf("no key injection tool is installed")
	}
	return errors.Join(failures...)
}

func keyComboCommand(tool string, keys []string) (*exec.Cmd, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("empty key combo")
	}
	switch tool {
	case "xdotool":
		return exec.Command(tool, "key", strings.Join(keys, "+")), nil
	case "ydotool":
		codes := make([]int, len(keys))
		for i, key := range keys {
			code, found := ydotoolKeycodes[key]
			if !found {
				return nil, fmt.Errorf("unsupported key %q", key)
			}
			codes[i] = code
		}
		args := []string{"key"}
		for _, code := range codes {
			args = append(args, fmt.Sprintf("%d:1", code))
		}
		for i := len(codes) - 1; i >= 0; i-- {
			args = append(args, fmt.Sprintf("%d:0", codes[i]))
		}
		return exec.Command(tool, args...), nil
	case "wtype":
		if len(keys) == 1 {
			key, found := wtypeKeys[keys[0]]
			if !found {
				return nil, fmt.Errorf("unsupported key %q", keys[0])
			}
			return exec.Command(tool, "-k", key), nil
		}
		if len(keys) == 2 && (keys[0] == "shift" || keys[0] == "ctrl") {
			key, found := wtypeKeys[keys[1]]
			if !found {
				return nil, fmt.Errorf("unsupported key %q", keys[1])
			}
			return exec.Command(tool, "-M", keys[0], "-k", key, "-m", keys[0]), nil
		}
		return nil, fmt.Errorf("unsupported key combo %v", keys)
	default:
		return nil, fmt.Errorf("unsupported typing tool %q", tool)
	}
}

func (ti *TextInjector) writeClipboard(text string) error {
	if ti.displayServer == "wayland" && ti.availableTools["wl-copy"] {
		command := exec.Command("wl-copy")
		command.Stdin = strings.NewReader(text)
		return command.Run()
	}
	return clipboard.WriteAll(text)
}

func (ti *TextInjector) typeLineBreak() error {
	if err := ti.typeKeyCombo([]string{"shift", "Return"}); err == nil {
		return nil
	}
	return ti.typeKeyCombo([]string{"Return"})
}

func (ti *TextInjector) typeBackspaces(count int) error {
	for range count {
		if err := ti.typeKeyCombo([]string{"BackSpace"}); err != nil {
			return err
		}
	}
	return nil
}
