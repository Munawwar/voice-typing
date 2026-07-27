package internal

import (
	"fmt"
	"log"

	webrtcvad "github.com/maxhawkins/go-webrtcvad"
)

const (
	vadFrameDurationMS = 30
	vadIdleAfterMS     = 2000
	vadPreRollMS       = 400
	vadMode            = 1
)

type voiceDetector interface {
	Process(int, []byte) (bool, error)
}

type audioGate struct {
	detector     voiceDetector
	sampleRate   int
	channels     int
	frameBytes   int
	idleFrames   int
	preRollBytes int
	silentFrames int
	active       bool
	pending      []byte
	preRoll      []byte
}

func newAudioGate(sampleRate, channels int) (*audioGate, error) {
	detector, err := webrtcvad.New()
	if err != nil {
		return nil, fmt.Errorf("initialize voice activity detector: %w", err)
	}
	if err := detector.SetMode(vadMode); err != nil {
		return nil, fmt.Errorf("configure voice activity detector: %w", err)
	}

	samplesPerChannel := sampleRate * vadFrameDurationMS / 1000
	if !detector.ValidRateAndFrameLength(sampleRate, samplesPerChannel) {
		return nil, fmt.Errorf(
			"voice activity detection does not support %d Hz audio; use 8000, 16000, 32000, or 48000 Hz, or pass --vad=false",
			sampleRate,
		)
	}
	return &audioGate{
		detector:     detector,
		sampleRate:   sampleRate,
		channels:     channels,
		frameBytes:   samplesPerChannel * channels * 2,
		idleFrames:   (vadIdleAfterMS + vadFrameDurationMS - 1) / vadFrameDurationMS,
		preRollBytes: ((vadPreRollMS + vadFrameDurationMS - 1) / vadFrameDurationMS) * samplesPerChannel * channels * 2,
		active:       true,
	}, nil
}

func (g *audioGate) Process(data []byte) ([]byte, error) {
	g.pending = append(g.pending, data...)
	output := make([]byte, 0, len(data))
	for len(g.pending) >= g.frameBytes {
		frame := g.pending[:g.frameBytes]
		g.pending = g.pending[g.frameBytes:]

		mono := frame
		if g.channels > 1 {
			mono = make([]byte, g.frameBytes/g.channels)
			for source, target := 0, 0; source < len(frame); source, target = source+g.channels*2, target+2 {
				copy(mono[target:target+2], frame[source:source+2])
			}
		}
		speaking, err := g.detector.Process(g.sampleRate, mono)
		if err != nil {
			return nil, fmt.Errorf("detect voice activity: %w", err)
		}

		if g.active {
			output = append(output, frame...)
			if speaking {
				g.silentFrames = 0
			} else {
				g.silentFrames++
				if g.silentFrames >= g.idleFrames {
					g.active = false
					g.preRoll = g.preRoll[:0]
					log.Println("VAD idle: pausing audio sent to Deepgram")
				}
			}
			continue
		}

		g.preRoll = append(g.preRoll, frame...)
		if len(g.preRoll) > g.preRollBytes {
			g.preRoll = g.preRoll[len(g.preRoll)-g.preRollBytes:]
		}
		if speaking {
			output = append(output, g.preRoll...)
			g.preRoll = g.preRoll[:0]
			g.silentFrames = 0
			g.active = true
			log.Println("VAD active: resuming audio sent to Deepgram")
		}
	}
	return output, nil
}
