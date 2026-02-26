package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

type TextInjector struct {
	displayServer  string
	silentMode     bool
	availableTools map[string]bool
}

func NewTextInjector() *TextInjector {
	injector := &TextInjector{
		displayServer:  detectDisplayServer(),
		silentMode:     false,
		availableTools: make(map[string]bool),
	}

	// Detect available tools
	injector.detectAvailableTools()

	return injector
}

func detectDisplayServer() string {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	x11Display := os.Getenv("DISPLAY")

	if sessionType == "wayland" || waylandDisplay != "" {
		return "wayland"
	} else if sessionType == "x11" || x11Display != "" {
		return "x11"
	}
	return "unknown"
}

func (ti *TextInjector) detectAvailableTools() {
	tools := []string{"xdotool", "ydotool", "wtype", "wl-copy", "wl-paste", "xclip", "xsel"}

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			ti.availableTools[tool] = true
		}
	}

}

func (ti *TextInjector) getAvailableToolsList() []string {
	var tools []string
	for tool, available := range ti.availableTools {
		if available {
			tools = append(tools, tool)
		}
	}
	return tools
}

func (ti *TextInjector) TypeText(text string) error {
	if text == "" {
		return nil
	}

	// Try direct typing first
	if err := ti.tryDirectTyping(text); err == nil {
		return nil
	}

	// Try clipboard paste as fallback
	if err := ti.tryClipboardPaste(text); err == nil {
		return nil
	}

	// Enter silent mode
	ti.silentMode = true
	log.Printf("All typing methods failed, entering silent mode")
	return fmt.Errorf("all typing methods failed")
}

func (ti *TextInjector) tryDirectTyping(text string) error {
	var tools []string

	if ti.displayServer == "wayland" {
		tools = []string{"wtype", "ydotool", "xdotool"}
	} else {
		tools = []string{"xdotool", "wtype", "ydotool"}
	}

	var lastErr error
	for _, tool := range tools {
		if !ti.availableTools[tool] {
			continue
		}

		var cmd *exec.Cmd
		switch tool {
		case "xdotool":
			cmd = exec.Command("xdotool", "type", "--delay", "50", text)
		case "wtype":
			cmd = exec.Command("wtype", text)
		case "ydotool":
			// `--` avoids text beginning with `-` being parsed as a flag.
			cmd = exec.Command("ydotool", "type", "--", text)
		}

		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("direct typing failed: %w", lastErr)
	}
	return fmt.Errorf("direct typing failed")
}

func (ti *TextInjector) tryClipboardPaste(text string) error {
	// Save current clipboard content
	originalClip, _ := ti.readClipboard()

	// Set text to clipboard
	if err := ti.writeClipboard(text); err != nil {
		return fmt.Errorf("failed to write to clipboard: %w", err)
	}

	// Try to paste
	pasteErr := ti.TypeKeyCombo([]string{"ctrl", "v"})

	// Restore original clipboard content
	time.Sleep(100 * time.Millisecond) // Give paste time to complete
	if originalClip != "" {
		_ = ti.writeClipboard(originalClip)
	} else {
		_ = ti.clearClipboard()
	}

	if pasteErr == nil {
		return nil
	}

	return fmt.Errorf("clipboard paste failed")
}

func (ti *TextInjector) TypeKeyCombo(keys []string) error {
	var tools []string

	if ti.displayServer == "wayland" {
		tools = []string{"ydotool", "wtype", "xdotool"}
	} else {
		tools = []string{"xdotool", "ydotool", "wtype"}
	}

	var lastErr error
	for _, tool := range tools {
		if !ti.availableTools[tool] {
			continue
		}

		if err := ti.executeKeyCombo(tool, keys); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("key combo failed %v: %w", keys, lastErr)
	}
	return fmt.Errorf("key combo failed: %v", keys)
}

func (ti *TextInjector) executeKeyCombo(tool string, keys []string) error {
	var cmd *exec.Cmd

	switch tool {
	case "xdotool":
		mappedKeys := ti.mapKeysForXdotool(keys)
		cmd = exec.Command("xdotool", "key", strings.Join(mappedKeys, "+"))

	case "ydotool":
		keycodeArgs, err := ti.buildYdotoolKeyArgs(keys)
		if err != nil {
			return fmt.Errorf("ydotool key mapping failed: %w", err)
		}
		cmdArgs := append([]string{"key"}, keycodeArgs...)
		cmd = exec.Command("ydotool", cmdArgs...)

	case "wtype":
		// wtype has limited key combo support
		if len(keys) == 1 {
			key := ti.mapKeyForWtype(keys[0])
			if key != "" {
				cmd = exec.Command("wtype", "-P", key, "-p", key)
			}
		} else if len(keys) == 2 && keys[0] == "shift" && keys[1] == "Return" {
			// Special case for Shift+Enter
			cmd = exec.Command("wtype", "-P", "enter", "-p", "enter")
		}
	}

	if cmd == nil {
		return fmt.Errorf("unsupported key combo for %s", tool)
	}

	return cmd.Run()
}

func (ti *TextInjector) mapKeysForXdotool(keys []string) []string {
	keyMap := map[string]string{
		"BackSpace": "BackSpace",
		"Return":    "Return",
		"shift":     "shift",
		"ctrl":      "ctrl",
		"Left":      "Left",
		"v":         "v",
	}

	mapped := make([]string, len(keys))
	for i, key := range keys {
		if mappedKey, ok := keyMap[key]; ok {
			mapped[i] = mappedKey
		} else {
			mapped[i] = key
		}
	}
	return mapped
}

func (ti *TextInjector) buildYdotoolKeyArgs(keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("empty key combo")
	}

	keycodes := make([]int, len(keys))
	for i, key := range keys {
		code, err := ti.mapKeyToYdotoolKeycode(key)
		if err != nil {
			return nil, err
		}
		keycodes[i] = code
	}

	// ydotool key expects Linux input keycodes with press/release states.
	args := make([]string, 0, len(keys)*2)
	for _, code := range keycodes {
		args = append(args, fmt.Sprintf("%d:1", code))
	}
	for i := len(keycodes) - 1; i >= 0; i-- {
		args = append(args, fmt.Sprintf("%d:0", keycodes[i]))
	}
	return args, nil
}

func (ti *TextInjector) mapKeyToYdotoolKeycode(key string) (int, error) {
	keycodeMap := map[string]int{
		"BackSpace": 14,
		"Return":    28,
		"ctrl":      29,
		"v":         47,
		"shift":     42,
		"Left":      105,
	}

	if code, ok := keycodeMap[key]; ok {
		return code, nil
	}

	return 0, fmt.Errorf("unsupported ydotool key: %s", key)
}

func (ti *TextInjector) mapKeyForWtype(key string) string {
	keyMap := map[string]string{
		"BackSpace": "backspace",
		"Return":    "enter",
		"shift":     "shift",
	}

	if mappedKey, ok := keyMap[key]; ok {
		return mappedKey
	}
	return ""
}

func (ti *TextInjector) readClipboard() (string, error) {
	if ti.displayServer == "wayland" && ti.availableTools["wl-paste"] {
		cmd := exec.Command("wl-paste", "--no-newline")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return clipboard.ReadAll()
}

func (ti *TextInjector) writeClipboard(text string) error {
	if ti.displayServer == "wayland" && ti.availableTools["wl-copy"] {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return clipboard.WriteAll(text)
}

func (ti *TextInjector) clearClipboard() error {
	if ti.displayServer == "wayland" && ti.availableTools["wl-copy"] {
		return exec.Command("wl-copy", "--clear").Run()
	}
	return clipboard.WriteAll("")
}

func (ti *TextInjector) TypeNewline() error {
	// Shift+Enter is preferred for chat-style text fields.
	// Fall back to Enter for editors/forms where that is the newline action.
	if err := ti.TypeKeyCombo([]string{"shift", "Return"}); err == nil {
		return nil
	}
	return ti.TypeKeyCombo([]string{"Return"})
}

func (ti *TextInjector) TypeParagraphBreak() error {
	if err := ti.TypeKeyCombo([]string{"shift", "Return"}); err != nil {
		return err
	}
	return ti.TypeKeyCombo([]string{"shift", "Return"})
}

func (ti *TextInjector) TypeBackspaces(count int) error {
	for i := 0; i < count; i++ {
		if err := ti.TypeKeyCombo([]string{"BackSpace"}); err != nil {
			return err
		}
	}
	return nil
}

func (ti *TextInjector) SuggestInstallation() {
	log.Printf("\n🔧 Install typing tools for %s:", ti.displayServer)

	if ti.displayServer == "wayland" {
		log.Println("Primary (Wayland):")
		log.Println("  sudo apt install wtype ydotool")
		log.Println("  sudo systemctl enable --now ydotoold")
		log.Println("  sudo usermod -a -G input $USER")
		log.Println("\nFallback (X11 compatibility):")
		log.Println("  sudo apt install xdotool")
	} else {
		log.Println("Primary (X11):")
		log.Println("  sudo apt install xdotool")
		log.Println("\nFallback (Wayland compatibility):")
		log.Println("  sudo apt install wtype ydotool")
	}
}
