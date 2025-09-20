package main

import (
	"context"
	"log"
	"os"
	"time"

	msginterfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/api/listen/v1/websocket/interfaces"
	interfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/client/interfaces/v1"
	"github.com/deepgram/deepgram-go-sdk/v3/pkg/client/listen"
)

type TestCallback struct {
	connected bool
	messages  int
}

func (tc *TestCallback) Open(or *msginterfaces.OpenResponse) error {
	log.Println("✅ WebSocket OPENED successfully!")
	tc.connected = true
	return nil
}

func (tc *TestCallback) Message(mr *msginterfaces.MessageResponse) error {
	tc.messages++
	if len(mr.Channel.Alternatives) > 0 {
		transcript := mr.Channel.Alternatives[0].Transcript
		if transcript != "" {
			log.Printf("📝 Transcript (%d): %s (final: %t)", tc.messages, transcript, mr.IsFinal)
		}
	}
	return nil
}

func (tc *TestCallback) Metadata(md *msginterfaces.MetadataResponse) error {
	log.Printf("📊 Metadata received")
	return nil
}

func (tc *TestCallback) SpeechStarted(ssr *msginterfaces.SpeechStartedResponse) error {
	log.Printf("🎤 Speech started")
	return nil
}

func (tc *TestCallback) UtteranceEnd(ur *msginterfaces.UtteranceEndResponse) error {
	log.Printf("🏁 Utterance ended")
	return nil
}

func (tc *TestCallback) Close(cr *msginterfaces.CloseResponse) error {
	log.Printf("❌ WebSocket closed")
	return nil
}

func (tc *TestCallback) Error(er *msginterfaces.ErrorResponse) error {
	log.Printf("🚨 WebSocket error: %s", er.ErrMsg)
	return nil
}

func (tc *TestCallback) UnhandledEvent(byData []byte) error {
	log.Printf("❓ Unhandled event: %s", string(byData))
	return nil
}

func main() {
	// Read API key from config.json
	apiKey := os.Getenv("DEEPGRAM_API_KEY")
	if apiKey == "" {
		// Try to read from config file
		if data, err := os.ReadFile("config.json"); err == nil {
			// Simple extraction - just find the key value
			content := string(data)
			start := "\"deepgram_api_key\": \""
			if idx := len(start); idx > 0 {
				if startIdx := len(content); startIdx > 0 {
					for i := 0; i < len(content)-len(start); i++ {
						if content[i:i+len(start)] == start {
							endIdx := i + len(start)
							for j := endIdx; j < len(content); j++ {
								if content[j] == '"' {
									apiKey = content[endIdx:j]
									break
								}
							}
							break
						}
					}
				}
			}
		}
	}

	if apiKey == "" || apiKey == "your_deepgram_api_key_here" {
		log.Fatal("❌ Please set DEEPGRAM_API_KEY environment variable or update config.json")
	}

	log.Printf("🔑 Using API key: %s...%s", apiKey[:8], apiKey[len(apiKey)-4:])

	// Initialize SDK with trace logging
	listen.Init(listen.InitLib{LogLevel: listen.LogLevelTrace})

	ctx := context.Background()

	// Set up options
	options := &interfaces.LiveTranscriptionOptions{
		Model:       "nova-3",
		Language:    "en-US",
		SmartFormat: true,
		Punctuate:   true,
		Encoding:    "linear16",
		SampleRate:  16000,
		Channels:    1,
	}

	// Create callback
	callback := &TestCallback{}

	log.Println("🚀 Creating Deepgram WebSocket client...")

	// Create WebSocket client
	wsClient, err := listen.NewWSUsingCallback(ctx, apiKey, &interfaces.ClientOptions{}, options, callback)
	if err != nil {
		log.Fatalf("❌ Failed to create WebSocket client: %v", err)
	}

	log.Println("🔗 Explicitly connecting WebSocket...")
	wsClient.Connect()

	log.Println("⏳ Waiting for WebSocket connection...")

	// Wait for connection with timeout
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Fatal("❌ Timeout waiting for WebSocket connection")
		case <-ticker.C:
			if callback.connected {
				log.Println("✅ WebSocket connected! Sending test audio...")
				goto connected
			}
			log.Print("⏳ Still waiting...")
		}
	}

connected:
	// Send some test audio data (silence)
	testAudio := make([]byte, 1024*2) // 1024 samples of 16-bit audio (silence)

	for i := 0; i < 10; i++ {
		if err := wsClient.WriteBinary(testAudio); err != nil {
			log.Printf("❌ Error sending audio data: %v", err)
		} else {
			log.Printf("📤 Sent test audio chunk %d", i+1)
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("🎯 Test completed! Waiting 5 seconds for any final messages...")
	time.Sleep(5 * time.Second)

	log.Printf("📊 Final stats: Connected=%t, Messages=%d", callback.connected, callback.messages)

	// Cleanup
	wsClient.Stop()
	log.Println("🏁 Test finished")
}
