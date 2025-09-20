package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
)

type AudioConfig struct {
	SampleRate int `json:"sample_rate"`
	Channels   int `json:"channels"`
	BufferSize int `json:"buffer_size"`
}

type AudioStream struct {
	stream      *portaudio.Stream
	config      *AudioConfig
	audioBuffer []int16
	dataChan    chan []byte
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewAudioStream(config *AudioConfig) (*AudioStream, error) {
	// Redirect ALSA error messages to log file for cleaner console output
	redirectALSAErrorsToFile()
	defer restoreStderr()

	err := portaudio.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize portaudio: %w", err)
	}

	// Log where ALSA errors are being saved
	log.Println("Audio system initialized (ALSA errors logged to alsa_audio.log)")

	ctx, cancel := context.WithCancel(context.Background())

	return &AudioStream{
		config:      config,
		audioBuffer: make([]int16, config.BufferSize),
		dataChan:    make(chan []byte, 100), // Buffer audio data
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

var originalStderr int = -1

func redirectALSAErrorsToFile() {
	// Create/clear the ALSA log file
	logFile, err := os.OpenFile("alsa_audio.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("Warning: Could not create ALSA log file: %v", err)
		return
	}

	// Save original stderr
	originalStderr, err = syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		log.Printf("Warning: Could not duplicate stderr: %v", err)
		logFile.Close()
		return
	}

	// Redirect stderr to log file
	err = syscall.Dup2(int(logFile.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		log.Printf("Warning: Could not redirect stderr: %v", err)
		syscall.Close(originalStderr)
		originalStderr = -1
	}

	logFile.Close() // Close our handle, stderr now owns it
}

func restoreStderr() {
	if originalStderr != -1 {
		// Restore original stderr
		syscall.Dup2(originalStderr, int(os.Stderr.Fd()))
		syscall.Close(originalStderr)
		originalStderr = -1
	}
}

func (a *AudioStream) Start() error {
	// Try to find Jabra device first, fall back to default
	var inputDevice *portaudio.DeviceInfo
	devices, err := portaudio.Devices()
	if err != nil {
		return fmt.Errorf("failed to get audio devices: %w", err)
	}

	// Look for Jabra device
	for _, device := range devices {
		if device.MaxInputChannels > 0 && strings.Contains(strings.ToLower(device.Name), "jabra") {
			inputDevice = device
			log.Printf("Found Jabra device: %s", device.Name)
			break
		}
	}

	// Fall back to default input device if no Jabra found
	if inputDevice == nil {
		inputDevice, err = portaudio.DefaultInputDevice()
		if err != nil {
			return fmt.Errorf("failed to get default input device: %w", err)
		}
		log.Printf("Using default audio device: %s", inputDevice.Name)
	}

	log.Printf("Using audio device: %s (Max channels: %d)", inputDevice.Name, inputDevice.MaxInputChannels)

	// Use minimum of requested channels and device max channels
	channels := a.config.Channels
	log.Printf("Requested channels: %d, Device max: %d", channels, inputDevice.MaxInputChannels)

	// Special handling for USB audio devices (like Jabra)
	if strings.Contains(strings.ToLower(inputDevice.Name), "usb") || strings.Contains(strings.ToLower(inputDevice.Name), "jabra") {
		channels = 1 // Force mono for USB devices
		log.Printf("USB/Jabra device detected, forcing channels to 1")
	} else if channels > inputDevice.MaxInputChannels {
		channels = inputDevice.MaxInputChannels
		log.Printf("Reducing channels from %d to %d (device limit)", a.config.Channels, channels)
	}
	log.Printf("Final channels to use: %d", channels)
	// Update config so downstream (Deepgram) uses the actual channel count
	a.config.Channels = channels

	// Configure input parameters manually for better control
	inputParams := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: channels,
			Latency:  inputDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(a.config.SampleRate),
		FramesPerBuffer: a.config.BufferSize,
	}

	log.Printf("Audio params: Device=%s, Channels=%d, SampleRate=%.0f, Buffer=%d",
		inputDevice.Name, channels, inputParams.SampleRate, inputParams.FramesPerBuffer)

	// Create and start stream
	stream, err := portaudio.OpenStream(inputParams, a.audioCallback)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}

	a.stream = stream

	if err := a.stream.Start(); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	log.Printf("Audio stream started (Sample rate: %d, Channels: %d, Buffer: %d)",
		a.config.SampleRate, a.config.Channels, a.config.BufferSize)

	return nil
}

func (a *AudioStream) Stop() error {
	a.cancel()

	if a.stream != nil {
		if err := a.stream.Stop(); err != nil {
			log.Printf("Error stopping stream: %v", err)
		}
		if err := a.stream.Close(); err != nil {
			log.Printf("Error closing stream: %v", err)
		}
	}

	close(a.dataChan)
	portaudio.Terminate()

	log.Println("Audio stream stopped")
	return nil
}

func (a *AudioStream) GetDataChannel() <-chan []byte {
	return a.dataChan
}

func (a *AudioStream) audioCallback(inputBuffer []int16) {
	select {
	case <-a.ctx.Done():
		return
	default:
		// Convert int16 to bytes (little endian)
		audioBytes := make([]byte, len(inputBuffer)*2)
		for i, sample := range inputBuffer {
			audioBytes[i*2] = byte(sample)
			audioBytes[i*2+1] = byte(sample >> 8)
		}

		// Send to channel (non-blocking)
		select {
		case a.dataChan <- audioBytes:
		case <-time.After(10 * time.Millisecond):
			// Drop frame if channel is full
		}
	}
}
