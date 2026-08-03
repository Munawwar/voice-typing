package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"voice-typing/internal"
)

const version = "0.1.1"

var sessionFile = func() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			base = filepath.Join(cache, "voice-typing")
		} else {
			base = filepath.Join(os.TempDir(), fmt.Sprintf("voice-typing-%d", os.Getuid()))
		}
	}
	return filepath.Join(base, "session.lock")
}()

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	hotkey := flag.Bool("hotkey", false, "Start recording unless already active")
	stopkey := flag.Bool("stopkey", false, "Gracefully stop active recording")
	vadEnabled := flag.Bool("vad", true, "Pause Deepgram audio during silence")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Voice Typing v%s\n", version)
		return
	}

	if *stopkey {
		active, err := signalActiveRecording(syscall.SIGUSR1)
		if err != nil {
			log.Printf("Failed to stop recording: %v", err)
			showNotification("Voice Typing Error", "Failed to stop recording", "dialog-error")
		} else if !active {
			log.Println("No active recording found")
			showNotification("Voice Typing", "No active recording to stop", "dialog-information")
		}
		return
	}

	if *hotkey {
		_, active, err := activeRecordingPID()
		if err != nil {
			log.Printf("Failed to inspect recording session: %v", err)
			showNotification("Voice Typing Error", "Failed to check recording status", "dialog-error")
			return
		}
		if active {
			log.Println("Voice typing is already running")
			showNotification("Voice Typing", "Voice typing is already running.", "audio-input-microphone")
			return
		}
	}

	if *configPath == "" {
		configHome, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("Failed to locate configuration directory: %v", err)
		}
		*configPath = filepath.Join(configHome, "voice-typing", "config.json")
		if _, err := os.Stat("config.json"); err == nil {
			*configPath = "config.json"
		}
	}

	log.Printf("Loading config from: %s", *configPath)
	cfg, err := internal.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := runSingleSession(cfg, *vadEnabled); err != nil {
		log.Printf("Recording failed: %v", err)
		message := []rune(err.Error())
		if len(message) > 50 {
			message = message[:50]
		}
		showNotification("Speech Recognition", "Failed: "+string(message), "dialog-error")
	}
}

func processStartTime(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	closingParen := strings.LastIndexByte(string(data), ')')
	if closingParen < 0 {
		return "", fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(data[closingParen+1:]))
	if len(fields) <= 19 {
		return "", fmt.Errorf("process stat has %d fields", len(fields))
	}
	return fields[19], nil
}

func activeRecordingPID() (int, bool, error) {
	data, err := os.ReadFile(sessionFile)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	fields := strings.Fields(string(data))
	pid := 0
	if len(fields) > 0 {
		pid, _ = strconv.Atoi(fields[0])
	}
	valid := false
	if pid > 0 {
		startTime, statErr := processStartTime(pid)
		valid = statErr == nil && len(fields) == 2 && fields[1] == startTime
		if statErr == nil && len(fields) == 1 {
			executable, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
			valid = err == nil && strings.Contains(filepath.Base(executable), "voice-typing")
		}
	}
	if valid {
		return pid, true, nil
	}
	if err := os.Remove(sessionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, false, err
	}
	return 0, false, nil
}

func signalActiveRecording(sig syscall.Signal) (bool, error) {
	pid, active, err := activeRecordingPID()
	if err != nil || !active {
		return active, err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	if err := process.Signal(sig); err != nil {
		return false, err
	}
	return true, nil
}

func runSingleSession(cfg *internal.Config, vadEnabled bool) error {
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0700); err != nil {
		return fmt.Errorf("failed to create recording session directory: %w", err)
	}
	if _, active, err := activeRecordingPID(); err != nil {
		return fmt.Errorf("failed to inspect recording session: %w", err)
	} else if active {
		return fmt.Errorf("another recording is already active")
	}

	lock, err := os.OpenFile(sessionFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("another recording is already starting")
		}
		return fmt.Errorf("failed to create recording session: %w", err)
	}
	startTime, err := processStartTime(os.Getpid())
	if err == nil {
		_, err = fmt.Fprintf(lock, "%d %s\n", os.Getpid(), startTime)
	}
	closeErr := lock.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(sessionFile)
		return fmt.Errorf("failed to write recording session: %w", errors.Join(err, closeErr))
	}
	defer os.Remove(sessionFile)

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
	defer stopSignals()

	injector := internal.NewTextInjector()
	return record(ctx, cfg, internal.NewTranscriptionStack(injector), vadEnabled)
}

func record(ctx context.Context, config *internal.Config, transcription *internal.TranscriptionStack, vadEnabled bool) error {
	audioStream, err := internal.StartAudioStream(&config.Audio)
	if err != nil {
		return fmt.Errorf("failed to start audio: %w", err)
	}
	defer func() {
		if err := audioStream.Stop(); err != nil {
			log.Printf("Failed to stop audio cleanly: %v", err)
		}
	}()

	err = internal.StreamTranscription(ctx, config, transcription, audioStream, vadEnabled, func() {
		showNotification(
			"Voice Typing Ready!",
			"Focus on a text field and start talking. Say 'stop voice' to stop.",
			"audio-input-microphone",
		)
	})
	switch {
	case errors.Is(err, internal.ErrVoiceStop):
		showNotification("Voice Typing Stopped", "Recording ended by voice command.", "audio-input-microphone-muted")
		return nil
	case err != nil:
		return fmt.Errorf("streaming error: %w", err)
	default:
		showNotification("Voice Typing Stopped", "Recording stopped.", "audio-input-microphone-muted")
		return nil
	}
}

func showNotification(title, message, icon string) {
	if err := exec.Command("notify-send", title, message, "--icon="+icon, "--expire-time=3000").Run(); err != nil {
		log.Printf("Notification failed: %v", err)
	}
}
