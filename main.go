package main

import (
	"context"
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
	"time"

	"voice-typing/internal"
)

const (
	VERSION   = "0.1.0"
	LOCK_FILE = "/tmp/voice_recording.lock"
)

type SpeechService struct {
	config        *internal.Config
	audioStream   *internal.AudioStream
	textInjector  *internal.TextInjector
	transcription *internal.TranscriptionStack
	deepgram      *internal.DeepgramService
	recording     bool
	initializing  bool
}

func main() {
	var (
		configPath = flag.String("config", "", "Path to configuration file")
		version    = flag.Bool("version", false, "Show version information")
		hotkey     = flag.Bool("hotkey", false, "Single hotkey toggle mode")
		stopkey    = flag.Bool("stopkey", false, "Gracefully stop active recording")
		service    = flag.Bool("service", false, "Run as persistent service")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Voice Typing v%s\n", VERSION)
		os.Exit(0)
	}

	// Determine config path
	if *configPath == "" {
		// Try multiple locations in order:
		// 1. config.json in current directory (local)
		// 2. ~/.config/voice-typing/config.json (XDG standard)
		homeDir, _ := os.UserHomeDir()
		localConfigPath := "config.json"
		xdgConfigPath := filepath.Join(homeDir, ".config", "voice-typing", "config.json")

		if _, err := os.Stat(localConfigPath); err == nil {
			*configPath = localConfigPath
		} else if _, err := os.Stat(xdgConfigPath); err == nil {
			*configPath = xdgConfigPath
		} else {
			// Default to XDG path even if it doesn't exist (for better error messages)
			*configPath = xdgConfigPath
		}
	}

	// Load configuration
	log.Printf("Loading config from: %s", *configPath)
	cfg, err := internal.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded successfully")

	log.Printf("Mode detection - hotkey: %v, stopkey: %v, service: %v", *hotkey, *stopkey, *service)

	if *stopkey {
		// Don't kill existing instances when we're just sending a stop signal
		log.Println("Stop key pressed - requesting graceful stop")
		handleGracefulStop()
	} else if *hotkey {
		// Kill existing instances to prevent conflicts (only for hotkey/recording modes)
		if err := killExistingInstances(); err != nil {
			log.Printf("Warning: %v", err)
		}
		log.Println("Hotkey mode - performing single toggle")
		handleHotkeyToggle(cfg)
	} else if *service {
		// Kill existing instances to prevent conflicts
		if err := killExistingInstances(); err != nil {
			log.Printf("Warning: %v", err)
		}
		log.Println("Starting persistent service mode")
		runPersistentService(cfg)
	} else {
		// Kill existing instances to prevent conflicts
		if err := killExistingInstances(); err != nil {
			log.Printf("Warning: %v", err)
		}
		// Default: single recording session
		log.Println("Starting single recording session")
		runSingleSession(cfg)
	}

	log.Println("Main function completed")
}

func killExistingInstances() error {
	cmd := exec.Command("pgrep", "-f", "voice-typing")
	output, err := cmd.Output()
	if err != nil {
		// No existing processes found
		log.Println("No existing processes found")
		return nil
	}

	currentPid := os.Getpid()
	pids := strings.Split(strings.TrimSpace(string(output)), "\n")

	log.Printf("Current PID: %d, Found PIDs: %v", currentPid, pids)

	killedCount := 0
	for _, pidStr := range pids {
		if pidStr == "" {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			log.Printf("Invalid PID: %s", pidStr)
			continue
		}

		if pid == currentPid {
			log.Printf("Skipping current process PID: %d", pid)
			continue
		}

		if err := syscall.Kill(pid, syscall.SIGTERM); err == nil {
			killedCount++
			log.Printf("Killed existing process %d", pid)
		} else {
			log.Printf("Failed to kill process %d: %v", pid, err)
		}
	}

	if killedCount > 0 {
		time.Sleep(500 * time.Millisecond) // Give processes time to cleanup
	}

	log.Printf("Killed %d existing processes, continuing...", killedCount)
	return nil
}

func handleHotkeyToggle(cfg *internal.Config) {
	// Check for existing recording session
	if _, err := os.Stat(LOCK_FILE); err == nil {
		// Stop existing recording
		stopExistingRecording()
		return
	}

	// Start new recording session
	startHotkeyRecording(cfg)
}

func handleGracefulStop() {
	// Check if there's an active recording
	data, err := os.ReadFile(LOCK_FILE)
	if err != nil {
		log.Println("No active recording found")
		showNotification("Voice Typing", "No active recording to stop", "dialog-information")
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		log.Printf("Invalid PID in lock file: %v", err)
		os.Remove(LOCK_FILE)
		showNotification("Voice Typing Error", "Invalid lock file", "dialog-error")
		return
	}

	// Send SIGUSR1 for graceful stop
	log.Printf("Sending graceful stop signal (SIGUSR1) to PID %d", pid)
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		log.Printf("Error sending stop signal to process %d: %v", pid, err)
		showNotification("Voice Typing Error", "Failed to send stop signal", "dialog-error")
		return
	}

	// showNotification("Voice Typing", "Stopping recording gracefully...", "audio-input-microphone-muted")
}

func stopExistingRecording() {
	data, err := os.ReadFile(LOCK_FILE)
	if err != nil {
		log.Printf("Error reading lock file: %v", err)
		os.Remove(LOCK_FILE)
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		log.Printf("Invalid PID in lock file: %v", err)
		os.Remove(LOCK_FILE)
		return
	}

	log.Println("Stopping active voice recording...")

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		log.Printf("Error stopping process %d: %v", pid, err)
	} else {
		showNotification("Voice Typing Stopped", "Recording interrupted by hotkey.", "audio-input-microphone-muted")
	}

	time.Sleep(500 * time.Millisecond)
	os.Remove(LOCK_FILE)
}

func startHotkeyRecording(cfg *internal.Config) {
	// Create lock file
	if err := os.WriteFile(LOCK_FILE, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		log.Printf("Warning: Could not create lock file: %v", err)
	}
	defer os.Remove(LOCK_FILE)

	showNotification("Voice Typing Service", "Initializing... Please wait.", "audio-input-microphone")

	// Run single recording session
	runSingleSession(cfg)
}

func runSingleSession(cfg *internal.Config) {
	log.Println("Creating speech service...")
	service := NewSpeechService(cfg)
	log.Println("Speech service created")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	// SIGTERM: Force stop (from toggle hotkey)
	// SIGUSR1: Graceful stop (from stop hotkey)
	// SIGINT: Ctrl+C (graceful stop)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		sig := <-sigChan
		switch sig {
		case syscall.SIGUSR1:
			log.Println("\nReceived graceful stop signal (SIGUSR1)")
		case syscall.SIGTERM:
			log.Println("\nReceived force stop signal (SIGTERM)")
		case os.Interrupt:
			log.Println("\nReceived interrupt signal (Ctrl+C)")
		}
		cancel()
	}()

	log.Println("Starting recording...")
	if err := service.StartRecording(ctx, true); err != nil {
		log.Printf("Recording failed: %v", err)
		showNotification("Speech Recognition", fmt.Sprintf("Failed: %s", err.Error()[:50]), "dialog-error")
	}
	log.Println("Recording session completed")
}

func runPersistentService(cfg *internal.Config) {
	log.Println("Persistent service mode not yet implemented")
	log.Println("Use desktop environment hotkey settings to bind Super+] to:")
	log.Printf("   %s --hotkey", os.Args[0])
}

func NewSpeechService(cfg *internal.Config) *SpeechService {
	textInjector := internal.NewTextInjector()
	transcriptionStack := internal.NewTranscriptionStack(textInjector)

	return &SpeechService{
		config:        cfg,
		textInjector:  textInjector,
		transcription: transcriptionStack,
		recording:     false,
		initializing:  false,
	}
}

func (s *SpeechService) StartRecording(ctx context.Context, realTimeTyping bool) error {
	if s.recording || s.initializing {
		return fmt.Errorf("already recording or initializing")
	}

	s.initializing = true
	s.recording = true
	defer func() {
		s.recording = false
		s.initializing = false
	}()

	log.Println("Starting speech recognition...")
	log.Printf("Config loaded: API key length=%d, Sample rate=%d", len(s.config.DeepgramAPIKey), s.config.Audio.SampleRate)

	// Initialize audio stream
	audioStream, err := internal.NewAudioStream(&s.config.Audio)
	if err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}
	s.audioStream = audioStream
	defer s.audioStream.Stop()

	// Initialize Deepgram service
	deepgramService, err := internal.NewDeepgramService(s.config, s.transcription)
	if err != nil {
		return fmt.Errorf("failed to initialize Deepgram: %w", err)
	}
	s.deepgram = deepgramService
	defer s.deepgram.Close()

	// Clear transcription state
	s.transcription.Clear()

	// Start audio stream
	if err := s.audioStream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	s.initializing = false

	// Show ready notification
	if realTimeTyping {
		showNotification("Voice Typing Ready!",
			"Focus on a text field and start talking. Say 'stop voice' to stop.",
			"audio-input-microphone")
	}

	// Start Deepgram streaming
	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()

	streamErr := make(chan error, 1)
	go func() {
		streamErr <- s.deepgram.StartStreaming(streamCtx, s.audioStream, realTimeTyping)
	}()

	// Monitor for stop conditions
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping due to context cancellation (signal received)")
			showNotification("Voice Typing Stopped", "Recording stopped.", "audio-input-microphone-muted")
			return nil
		case err := <-streamErr:
			if err != nil {
				return fmt.Errorf("streaming error: %w", err)
			}
			return nil
		case <-ticker.C:
			if s.deepgram.IsStopRequested() {
				log.Println("Stopping due to voice command")
				showNotification("Voice Typing Stopped", "Recording ended by voice command.", "audio-input-microphone-muted")
				return nil
			}
		}
	}
}

func showNotification(title, message, icon string) {
	cmd := exec.Command("notify-send", title, message, "--icon="+icon, "--expire-time=3000")
	if err := cmd.Run(); err != nil {
		// Notifications are optional, don't fail if they don't work
		log.Printf("Notification failed: %v", err)
	}
}
