package internal

import (
	"encoding/json"
	"fmt"
	"os"
)

type TranscriptionConfig struct {
	Model           string `json:"model"`
	Language        string `json:"language"`
	SmartFormat     bool   `json:"smart_format"`
	Punctuate       bool   `json:"punctuate"`
	ProfanityFilter bool   `json:"profanity_filter"`
	FillerWords     bool   `json:"filler_words"`
	MipOptOut       bool   `json:"mip_opt_out"`
}

type Config struct {
	DeepgramAPIKey string              `json:"deepgram_api_key"`
	Hotkey         string              `json:"hotkey"`
	Audio          AudioConfig         `json:"audio"`
	Transcription  TranscriptionConfig `json:"transcription"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	if config.DeepgramAPIKey == "" {
		return nil, fmt.Errorf("deepgram_api_key is required in config")
	}

	// Set defaults if not specified
	if config.Audio.SampleRate == 0 {
		config.Audio.SampleRate = 16000
	}
	if config.Audio.Channels == 0 {
		config.Audio.Channels = 1
	}
	if config.Audio.BufferSize == 0 {
		config.Audio.BufferSize = 1024
	}
	if config.Transcription.Model == "" {
		config.Transcription.Model = "nova-3"
	}
	if config.Transcription.Language == "" {
		config.Transcription.Language = "en-US"
	}

	return &config, nil
}
