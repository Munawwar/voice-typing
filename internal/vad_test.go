package internal

import (
	"bytes"
	"errors"
	"testing"
)

type fakeVoiceDetector struct {
	decisions []bool
	frames    [][]byte
	err       error
}

func (f *fakeVoiceDetector) Process(_ int, frame []byte) (bool, error) {
	f.frames = append(f.frames, bytes.Clone(frame))
	if f.err != nil {
		return false, f.err
	}
	decision := f.decisions[0]
	f.decisions = f.decisions[1:]
	return decision, nil
}

func newTestAudioGate(detector voiceDetector, channels int) *audioGate {
	return &audioGate{
		detector:     detector,
		sampleRate:   16000,
		channels:     channels,
		frameBytes:   channels * 4,
		idleFrames:   3,
		preRollBytes: channels * 8,
		active:       true,
	}
}

func TestAudioGateIdlesAndReplaysPreRoll(t *testing.T) {
	detector := &fakeVoiceDetector{decisions: []bool{false, false, false, false, false, true}}
	gate := newTestAudioGate(detector, 1)
	frames := [][]byte{[]byte("aaaa"), []byte("bbbb"), []byte("cccc"), []byte("dddd"), []byte("eeee"), []byte("ffff")}

	for i, frame := range frames {
		got, err := gate.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case i < 3 && !bytes.Equal(got, frame):
			t.Fatalf("active frame %d = %q, want %q", i, got, frame)
		case i == 3 || i == 4:
			if len(got) != 0 {
				t.Fatalf("idle frame %d unexpectedly forwarded %q", i, got)
			}
		case i == 5 && !bytes.Equal(got, []byte("eeeeffff")):
			t.Fatalf("resume output = %q, want pre-roll %q", got, "eeeeffff")
		}
	}
	if !gate.active {
		t.Fatal("gate remained idle after speech")
	}
}

func TestAudioGateSpeechResetsIdleTimer(t *testing.T) {
	detector := &fakeVoiceDetector{decisions: []bool{false, false, true, false, false, false}}
	gate := newTestAudioGate(detector, 1)

	got, err := gate.Process([]byte("aaaabbbbccccddddeeeeffff"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("aaaabbbbccccddddeeeeffff")) {
		t.Fatalf("output = %q, want all active audio", got)
	}
	if gate.active {
		t.Fatal("gate did not idle after three consecutive silent frames")
	}
}

func TestAudioGateRechunksAndUsesFirstChannelForDetection(t *testing.T) {
	detector := &fakeVoiceDetector{decisions: []bool{true}}
	gate := newTestAudioGate(detector, 2)
	stereo := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	first, err := gate.Process(stereo[:3])
	if err != nil {
		t.Fatal(err)
	}
	second, err := gate.Process(stereo[3:])
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 0 || !bytes.Equal(second, stereo) {
		t.Fatalf("rechunked output = %v then %v, want empty then %v", first, second, stereo)
	}
	if !bytes.Equal(detector.frames[0], []byte{1, 2, 5, 6}) {
		t.Fatalf("detector frame = %v, want first channel samples", detector.frames[0])
	}
}

func TestAudioGatePropagatesDetectorFailure(t *testing.T) {
	detectorErr := errors.New("broken detector")
	gate := newTestAudioGate(&fakeVoiceDetector{err: detectorErr}, 1)

	if _, err := gate.Process([]byte("aaaa")); !errors.Is(err, detectorErr) {
		t.Fatalf("Process() error = %v, want %v", err, detectorErr)
	}
}

func TestNewAudioGateValidatesSampleRate(t *testing.T) {
	if _, err := newAudioGate(16000, 1); err != nil {
		t.Fatalf("newAudioGate(16000) failed: %v", err)
	}
	if _, err := newAudioGate(44100, 1); err == nil {
		t.Fatal("newAudioGate(44100) unexpectedly succeeded")
	}
}
