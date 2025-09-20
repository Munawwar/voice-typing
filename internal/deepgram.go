package internal

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	msginterfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/api/listen/v1/websocket/interfaces"
	interfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/client/interfaces/v1"
	"github.com/deepgram/deepgram-go-sdk/v3/pkg/client/listen"
)

type DeepgramService struct {
	wsClient       *listen.WSCallback
	config         *Config
	stack          *TranscriptionStack
	stopRequested  bool
	realTimeTyping bool
	connected      bool
}

// DeepgramCallback implements LiveMessageCallback interface
type DeepgramCallback struct {
	service *DeepgramService
}

func (dc *DeepgramCallback) Open(or *msginterfaces.OpenResponse) error {
	log.Println("Deepgram WebSocket connection opened")
	dc.service.connected = true
	return nil
}

func (dc *DeepgramCallback) Message(mr *msginterfaces.MessageResponse) error {
	if len(mr.Channel.Alternatives) > 0 {
		transcript := strings.TrimSpace(mr.Channel.Alternatives[0].Transcript)
		if transcript != "" && mr.IsFinal {
			log.Printf("Deepgram transcript: %s", transcript)

			// Check for stop commands
			if dc.service.isStopCommand(transcript) {
				dc.service.stopRequested = true
				log.Println("Stop command detected")
				return nil
			}

			// Add to transcription stack
			dc.service.stack.AddPhrase(transcript, dc.service.realTimeTyping)
		}
	}
	return nil
}

func (dc *DeepgramCallback) Metadata(md *msginterfaces.MetadataResponse) error {
	return nil
}

func (dc *DeepgramCallback) SpeechStarted(ssr *msginterfaces.SpeechStartedResponse) error {
	log.Println("Speech started")
	return nil
}

func (dc *DeepgramCallback) UtteranceEnd(ur *msginterfaces.UtteranceEndResponse) error {
	log.Println("Utterance ended")
	return nil
}

func (dc *DeepgramCallback) Close(cr *msginterfaces.CloseResponse) error {
	log.Println("Deepgram WebSocket connection closed")
	return nil
}

func (dc *DeepgramCallback) Error(er *msginterfaces.ErrorResponse) error {
	log.Printf("Deepgram WebSocket error: %v", er.ErrMsg)
	return nil
}

func (dc *DeepgramCallback) UnhandledEvent(byData []byte) error {
	log.Printf("Unhandled event: %s", string(byData))
	return nil
}

func NewDeepgramService(cfg *Config, stack *TranscriptionStack) (*DeepgramService, error) {
	return &DeepgramService{
		config:        cfg,
		stack:         stack,
		stopRequested: false,
		connected:     false,
	}, nil
}

func (ds *DeepgramService) StartStreaming(ctx context.Context, audioStream *AudioStream, realTimeTyping bool) error {
	log.Println("Starting Deepgram WebSocket streaming...")

	ds.realTimeTyping = realTimeTyping

	// Initialize SDK with default settings per docs
	listen.InitWithDefault()

	// Set up WebSocket streaming options
	options := &interfaces.LiveTranscriptionOptions{
		Model:       ds.config.Transcription.Model,
		Language:    ds.config.Transcription.Language,
		SmartFormat: ds.config.Transcription.SmartFormat,
		Punctuate:   ds.config.Transcription.Punctuate,
		FillerWords: ds.config.Transcription.FillerWords,
		Encoding:    "linear16",
		SampleRate:  ds.config.Audio.SampleRate,
		Channels:    ds.config.Audio.Channels,
	}

	// Create callback
	callback := &DeepgramCallback{service: ds}

	// Apply privacy settings to context if configured
	if ds.config.Transcription.MipOptOut {
		params := make(map[string][]string, 0)
		params["mip_opt_out"] = []string{"true"}
		ctx = interfaces.WithCustomParameters(ctx, params)
		log.Println("Privacy: Opted out of Deepgram Model Improvement Program")
	}

	// Create WebSocket client using callback approach
	wsClient, err := listen.NewWSUsingCallback(ctx, ds.config.DeepgramAPIKey, &interfaces.ClientOptions{}, options, callback)
	if err != nil {
		return fmt.Errorf("failed to create WebSocket client: %w", err)
	}
	ds.wsClient = wsClient

	// Explicitly connect the WebSocket
	log.Println("Connecting to Deepgram WebSocket...")
	ds.wsClient.Connect()

	// Wait for connection to be established
	log.Println("Waiting for Deepgram WebSocket connection...")
	for i := 0; i < 50; i++ { // Wait up to 5 seconds
		if ds.connected {
			log.Println("Deepgram WebSocket connection established")
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 49 {
			return fmt.Errorf("timeout waiting for Deepgram WebSocket open")
		}
	}

	// Stream audio data
	dataChan := audioStream.GetDataChannel()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping due to context cancellation")
			ds.wsClient.Stop()
			return nil

		case audioData, ok := <-dataChan:
			if !ok {
				log.Println("Audio stream ended")
				ds.wsClient.Stop()
				return nil
			}

			if ds.stopRequested {
				log.Println("Stopping due to voice command")
				ds.wsClient.Stop()
				return nil
			}

			// Send audio data to Deepgram
			if err := ds.wsClient.WriteBinary(audioData); err != nil {
				log.Printf("Error sending audio data: %v", err)
				continue
			}
		}
	}
}

func (ds *DeepgramService) isStopCommand(transcript string) bool {
	transcriptLower := strings.ToLower(strings.TrimSpace(transcript))
	stopKeywords := []string{
		"end voice", "end recording", "stop recording", "stop voice",
	}

	for _, keyword := range stopKeywords {
		if strings.Contains(transcriptLower, keyword) {
			return true
		}
	}
	return false
}

func (ds *DeepgramService) IsStopRequested() bool {
	return ds.stopRequested
}

func (ds *DeepgramService) Close() error {
	if ds.wsClient != nil {
		ds.wsClient.Stop()
	}
	return nil
}
