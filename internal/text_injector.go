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
	tools := []string{"xdotool", "ydotool", "wtype", "wl-copy", "xclip", "xsel"}

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			ti.availableTools[tool] = true
		}
	}

	log.Printf("Available tools: %v (Display server: %s)", ti.getAvailableToolsList(), ti.displayServer)
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
			cmd = exec.Command("ydotool", "type", text)
		}

		if err := cmd.Run(); err == nil {
			log.Printf("✅ Typed via %s: %s", tool, text)
			return nil
		}
	}

	return fmt.Errorf("direct typing failed")
}

func (ti *TextInjector) tryClipboardPaste(text string) error {
	// Save current clipboard content
	originalClip, _ := clipboard.ReadAll()

	// Set text to clipboard
	if err := clipboard.WriteAll(text); err != nil {
		return fmt.Errorf("failed to write to clipboard: %w", err)
	}

	// Try to paste
	pasteErr := ti.TypeKeyCombo([]string{"ctrl", "v"})

	// Restore original clipboard content
	time.Sleep(100 * time.Millisecond) // Give paste time to complete
	if originalClip != "" {
		clipboard.WriteAll(originalClip)
	}

	if pasteErr == nil {
		log.Printf("✅ Pasted via clipboard: %s", text)
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

	for _, tool := range tools {
		if !ti.availableTools[tool] {
			continue
		}

		if err := ti.executeKeyCombo(tool, keys); err == nil {
			return nil
		}
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
		mappedKeys := ti.mapKeysForYdotool(keys)
		cmd = exec.Command("ydotool", "key", strings.Join(mappedKeys, "+"))

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

func (ti *TextInjector) mapKeysForYdotool(keys []string) []string {
	keyMap := map[string]string{
		"BackSpace": "Backspace",
		"Return":    "enter",
		"shift":     "shift",
		"ctrl":      "ctrl",
		"Left":      "left",
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

func (ti *TextInjector) TypeNewline() error {
	return ti.TypeKeyCombo([]string{"shift", "Return"})
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
