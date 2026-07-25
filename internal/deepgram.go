package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	msginterfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/api/listen/v1/websocket/interfaces"
	interfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/client/interfaces/v1"
	"github.com/deepgram/deepgram-go-sdk/v3/pkg/client/listen"
)

var ErrVoiceStop = errors.New("recording stopped by voice command")

type deepgramService struct {
	config   *Config
	stack    *TranscriptionStack
	stop     chan struct{}
	failures chan error
	stopOnce sync.Once
}

type deepgramCallback struct {
	service *deepgramService
}

func (dc *deepgramCallback) Open(*msginterfaces.OpenResponse) error {
	log.Println("Deepgram WebSocket connection opened")
	return nil
}

func (dc *deepgramCallback) Message(response *msginterfaces.MessageResponse) error {
	if len(response.Channel.Alternatives) == 0 || !response.IsFinal {
		return nil
	}
	transcript := strings.TrimSpace(response.Channel.Alternatives[0].Transcript)
	if transcript == "" {
		return nil
	}

	log.Printf("Deepgram transcript: %s", transcript)
	stop, err := dc.service.stack.addPhrase(transcript)
	if err != nil {
		select {
		case dc.service.failures <- err:
		default:
		}
		return nil
	}
	if stop {
		dc.service.stopOnce.Do(func() { close(dc.service.stop) })
	}
	return nil
}

func (dc *deepgramCallback) Metadata(*msginterfaces.MetadataResponse) error {
	return nil
}

func (dc *deepgramCallback) SpeechStarted(*msginterfaces.SpeechStartedResponse) error {
	log.Println("Speech started")
	return nil
}

func (dc *deepgramCallback) UtteranceEnd(*msginterfaces.UtteranceEndResponse) error {
	log.Println("Utterance ended")
	return nil
}

func (dc *deepgramCallback) Close(*msginterfaces.CloseResponse) error {
	log.Println("Deepgram WebSocket connection closed")
	return nil
}

func (dc *deepgramCallback) Error(response *msginterfaces.ErrorResponse) error {
	err := fmt.Errorf("deepgram WebSocket error: %s", response.ErrMsg)
	log.Print(err)
	select {
	case dc.service.failures <- err:
	default:
	}
	return nil
}

func (dc *deepgramCallback) UnhandledEvent(data []byte) error {
	log.Printf("Unhandled event: %s", data)
	return nil
}

func StreamTranscription(
	ctx context.Context,
	config *Config,
	stack *TranscriptionStack,
	audioStream *AudioStream,
	ready func(),
) error {
	ds := &deepgramService{
		config:   config,
		stack:    stack,
		stop:     make(chan struct{}),
		failures: make(chan error, 1),
	}
	listen.InitWithDefault()
	options := &interfaces.LiveTranscriptionOptions{
		Model:           ds.config.Transcription.Model,
		Language:        ds.config.Transcription.Language,
		SmartFormat:     ds.config.Transcription.SmartFormat,
		Punctuate:       ds.config.Transcription.Punctuate,
		ProfanityFilter: ds.config.Transcription.ProfanityFilter,
		FillerWords:     ds.config.Transcription.FillerWords,
		Encoding:        "linear16",
		SampleRate:      ds.config.Audio.SampleRate,
		Channels:        ds.config.Audio.Channels,
	}
	if ds.config.Transcription.MipOptOut {
		ctx = interfaces.WithCustomParameters(ctx, map[string][]string{"mip_opt_out": {"true"}})
	}

	client, err := listen.NewWSUsingCallback(
		ctx,
		ds.config.DeepgramAPIKey,
		&interfaces.ClientOptions{},
		options,
		&deepgramCallback{service: ds},
	)
	if err != nil {
		return fmt.Errorf("create Deepgram WebSocket client: %w", err)
	}
	if !client.Connect() {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("connect to Deepgram WebSocket")
	}
	defer client.Stop()
	if ready != nil {
		ready()
	}

	audio := audioStream.dataChan
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ds.stop:
			return ErrVoiceStop
		case err := <-ds.failures:
			return err
		case data, ok := <-audio:
			if !ok {
				return nil
			}
			if err := client.WriteBinary(data); err != nil {
				return fmt.Errorf("send audio to Deepgram: %w", err)
			}
		}
	}
}
