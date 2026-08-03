package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type TranscriptionConfig struct {
	Model           string `json:"model"`
	Language        string `json:"language"`
	SmartFormat     bool   `json:"smart_format"`
	Punctuate       bool   `json:"punctuate"`
	ProfanityFilter bool   `json:"profanity_filter"`
	FillerWords     bool   `json:"filler_words"`
	MipOptIn        bool   `json:"mip_opt_in"`
}

type Config struct {
	DeepgramAPIKey string              `json:"deepgram_api_key"`
	Audio          AudioConfig         `json:"audio"`
	Transcription  TranscriptionConfig `json:"transcription"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	config.DeepgramAPIKey = strings.TrimSpace(config.DeepgramAPIKey)
	if config.DeepgramAPIKey == "" ||
		config.DeepgramAPIKey == "your_deepgram_api_key_here" ||
		config.DeepgramAPIKey == "your_actual_api_key_here" {
		return nil, fmt.Errorf("deepgram_api_key must contain a real API key")
	}
	if config.Audio.SampleRate == 0 {
		config.Audio.SampleRate = 16000
	}
	if config.Audio.Channels == 0 {
		config.Audio.Channels = 1
	}
	if config.Audio.BufferSize == 0 {
		config.Audio.BufferSize = 1024
	}
	if config.Audio.SampleRate < 1 || config.Audio.Channels < 1 || config.Audio.BufferSize < 1 {
		return nil, fmt.Errorf("audio sample_rate, channels, and buffer_size must all be positive")
	}
	if config.Transcription.Model == "" {
		config.Transcription.Model = "nova-3"
	}
	if config.Transcription.Language == "" {
		config.Transcription.Language = "en-US"
	}
	return &config, nil
}
