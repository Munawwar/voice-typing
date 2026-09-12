package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaultsAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"deepgram_api_key":"real-key","transcription":{"keyterms":["Bulma","Xiaomi"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Audio.SampleRate != 16000 || config.Audio.Channels != 1 || config.Audio.BufferSize != 1024 {
		t.Fatalf("unexpected audio defaults: %+v", config.Audio)
	}
	if config.Transcription.Model != "nova-3" || config.Transcription.Language != "en-US" {
		t.Fatalf("unexpected transcription defaults: %+v", config.Transcription)
	}
	if len(config.Transcription.Keyterms) != 2 || config.Transcription.Keyterms[0] != "Bulma" {
		t.Fatalf("unexpected keyterms: %v", config.Transcription.Keyterms)
	}
	if config.Transcription.MipOptIn {
		t.Fatal("MIP participation enabled by default")
	}

	for _, invalid := range []string{
		`{"deepgram_api_key":"your_deepgram_api_key_here"}`,
		`{"deepgram_api_key":"real-key","audio":{"channels":-1}}`,
	} {
		if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfig(path); err == nil {
			t.Fatalf("LoadConfig accepted %s", invalid)
		}
	}
}
