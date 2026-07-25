package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestActiveRecordingPIDValidatesProcessIdentity(t *testing.T) {
	original := sessionFile
	sessionFile = filepath.Join(t.TempDir(), "session.lock")
	t.Cleanup(func() { sessionFile = original })

	startTime, err := processStartTime(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionFile, []byte(fmt.Sprintf("%d %s\n", os.Getpid(), startTime)), 0600); err != nil {
		t.Fatal(err)
	}
	pid, active, err := activeRecordingPID()
	if err != nil || !active || pid != os.Getpid() {
		t.Fatalf("activeRecordingPID() = %d, %v, %v", pid, active, err)
	}

	if err := os.WriteFile(sessionFile, []byte(fmt.Sprintf("%d stale\n", os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	if _, active, err := activeRecordingPID(); err != nil || active {
		t.Fatalf("stale session considered active: active=%v err=%v", active, err)
	}
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Fatalf("stale session was not removed: %v", err)
	}
}

func TestSignalActiveRecordingTargetsOnlySessionPID(t *testing.T) {
	original := sessionFile
	sessionFile = filepath.Join(t.TempDir(), "session.lock")
	t.Cleanup(func() { sessionFile = original })

	command := exec.Command("sleep", "30")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	startTime, err := processStartTime(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(fmt.Sprintf("%d %s\n", command.Process.Pid, startTime))
	if err := os.WriteFile(sessionFile, data, 0600); err != nil {
		t.Fatal(err)
	}

	active, err := signalActiveRecording(syscall.SIGTERM)
	if err != nil || !active {
		t.Fatalf("signalActiveRecording() = %v, %v", active, err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("SIGTERM did not terminate the recorded process")
	}
}
