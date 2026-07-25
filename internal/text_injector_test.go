package internal

import (
	"slices"
	"testing"
)

func TestNewTextInjectorDetectsDisplayServer(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", ":0")
	if injector := NewTextInjector(); injector.displayServer != "wayland" {
		t.Fatalf("display server = %q, want wayland", injector.displayServer)
	}
}

func TestWtypeKeyCombosPreserveModifiers(t *testing.T) {
	tests := []struct {
		keys []string
		args []string
	}{
		{[]string{"shift", "Return"}, []string{"wtype", "-M", "shift", "-k", "Return", "-m", "shift"}},
		{[]string{"ctrl", "v"}, []string{"wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl"}},
		{[]string{"BackSpace"}, []string{"wtype", "-k", "BackSpace"}},
	}
	for _, test := range tests {
		command, err := keyComboCommand("wtype", test.keys)
		if err != nil {
			t.Fatalf("keyComboCommand(%v): %v", test.keys, err)
		}
		if !slices.Equal(command.Args, test.args) {
			t.Fatalf("keyComboCommand(%v) = %v, want %v", test.keys, command.Args, test.args)
		}
	}
}

func TestYdotoolReleasesKeysInReverseOrder(t *testing.T) {
	command, err := keyComboCommand("ydotool", []string{"ctrl", "v"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ydotool", "key", "29:1", "47:1", "47:0", "29:0"}
	if !slices.Equal(command.Args, want) {
		t.Fatalf("command args = %v, want %v", command.Args, want)
	}
}
