package main

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"

	msginterfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/api/listen/v1/websocket/interfaces"
	interfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/client/interfaces/v1"
	"github.com/deepgram/deepgram-go-sdk/v3/pkg/client/listen"

	"voice-typing/internal"
)

type TestCallback struct {
	messages atomic.Int64
}

func (tc *TestCallback) Open(*msginterfaces.OpenResponse) error {
	log.Println("✅ WebSocket opened")
	return nil
}

func (tc *TestCallback) Message(response *msginterfaces.MessageResponse) error {
	count := tc.messages.Add(1)
	if len(response.Channel.Alternatives) > 0 {
		transcript := response.Channel.Alternatives[0].Transcript
		if transcript != "" {
			log.Printf("📝 Transcript (%d): %s (final: %t)", count, transcript, response.IsFinal)
		}
	}
	return nil
}

func (tc *TestCallback) Metadata(*msginterfaces.MetadataResponse) error {
	log.Println("📊 Metadata received")
	return nil
}

func (tc *TestCallback) SpeechStarted(*msginterfaces.SpeechStartedResponse) error {
	log.Println("🎤 Speech started")
	return nil
}

func (tc *TestCallback) UtteranceEnd(*msginterfaces.UtteranceEndResponse) error {
	log.Println("🏁 Utterance ended")
	return nil
}

func (tc *TestCallback) Close(*msginterfaces.CloseResponse) error {
	log.Println("WebSocket closed")
	return nil
}

func (tc *TestCallback) Error(response *msginterfaces.ErrorResponse) error {
	log.Printf("WebSocket error: %s", response.ErrMsg)
	return nil
}

func (tc *TestCallback) UnhandledEvent(data []byte) error {
	log.Printf("Unhandled event: %s", data)
	return nil
}

func main() {
	apiKey := os.Getenv("DEEPGRAM_API_KEY")
	if apiKey == "" {
		config, err := internal.LoadConfig("config.json")
		if err != nil {
			log.Fatalf("Set DEEPGRAM_API_KEY or provide config.json: %v", err)
		}
		apiKey = config.DeepgramAPIKey
	}

	listen.Init(listen.InitLib{LogLevel: listen.LogLevelTrace})
	options := &interfaces.LiveTranscriptionOptions{
		Model:       "nova-3",
		Language:    "en-US",
		SmartFormat: true,
		Punctuate:   true,
		Encoding:    "linear16",
		SampleRate:  16000,
		Channels:    1,
	}
	callback := &TestCallback{}
	client, err := listen.NewWSUsingCallback(
		context.Background(),
		apiKey,
		&interfaces.ClientOptions{},
		options,
		callback,
	)
	if err != nil {
		log.Fatalf("Create WebSocket client: %v", err)
	}
	if !client.Connect() {
		log.Fatal("Connect to Deepgram WebSocket")
	}
	defer client.Stop()

	silence := make([]byte, 1024*2)
	for i := 1; i <= 10; i++ {
		if err := client.WriteBinary(silence); err != nil {
			log.Printf("Send audio chunk %d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Println("Waiting 5 seconds for final messages...")
	time.Sleep(5 * time.Second)
	log.Printf("Received %d messages", callback.messages.Load())
}
