package internal

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/gordonklaus/portaudio"
)

type AudioConfig struct {
	SampleRate int `json:"sample_rate"`
	Channels   int `json:"channels"`
	BufferSize int `json:"buffer_size"`
}

type AudioStream struct {
	stream   *portaudio.Stream
	dataChan chan []byte
	ctx      context.Context
	cancel   context.CancelFunc
}

func StartAudioStream(config *AudioConfig) (result *AudioStream, resultErr error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize PortAudio: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	audio := &AudioStream{
		dataChan: make(chan []byte, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
	defer func() {
		if resultErr != nil {
			_ = audio.Stop()
		}
	}()
	inputDevice, err := portaudio.DefaultInputDevice()
	if err != nil {
		return nil, fmt.Errorf("get default input device: %w", err)
	}
	if inputDevice.MaxInputChannels < 1 {
		return nil, fmt.Errorf("default audio device %q has no input channels", inputDevice.Name)
	}
	if config.Channels > inputDevice.MaxInputChannels {
		log.Printf(
			"Reducing channels from %d to device maximum %d",
			config.Channels,
			inputDevice.MaxInputChannels,
		)
		config.Channels = inputDevice.MaxInputChannels
	}

	parameters := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: config.Channels,
			Latency:  inputDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(config.SampleRate),
		FramesPerBuffer: config.BufferSize,
	}
	stream, err := portaudio.OpenStream(parameters, audio.audioCallback)
	if err != nil {
		return nil, fmt.Errorf("open audio stream: %w", err)
	}
	audio.stream = stream
	if err := stream.Start(); err != nil {
		return nil, fmt.Errorf("start audio stream: %w", err)
	}
	log.Printf(
		"Audio stream started (device: %s, sample rate: %d, channels: %d, buffer: %d)",
		inputDevice.Name,
		config.SampleRate,
		config.Channels,
		config.BufferSize,
	)
	return audio, nil
}

func (a *AudioStream) Stop() error {
	a.cancel()
	var stopErr, closeErr error
	if a.stream != nil {
		stopErr = a.stream.Stop()
		closeErr = a.stream.Close()
	}
	close(a.dataChan)
	terminateErr := portaudio.Terminate()
	if err := errors.Join(stopErr, closeErr, terminateErr); err != nil {
		return fmt.Errorf("stop audio stream: %w", err)
	}
	return nil
}

func (a *AudioStream) audioCallback(input []int16) {
	select {
	case <-a.ctx.Done():
		return
	default:
	}

	data := make([]byte, len(input)*2)
	for i, sample := range input {
		data[i*2] = byte(sample)
		data[i*2+1] = byte(sample >> 8)
	}
	select {
	case a.dataChan <- data:
	default:
	}
}
