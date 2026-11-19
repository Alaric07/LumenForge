package m65rgbultraW

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGPUChildTimeoutPreservesAMDFallback(t *testing.T) {
	for _, scenario := range []string{"success", "error", "child-timeout", "parent-during", "parent-after", "already-cancelled", "nvidia-disabled", "zero", "invalid"} {
		t.Run(scenario, func(t *testing.T) {
			parent, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if scenario == "already-cancelled" {
				cancel()
			}
			nvidiaCalls, amdCalls := 0, 0
			var childContext context.Context
			nvidia := func(child context.Context, index int) ([]byte, error) {
				nvidiaCalls++
				childContext = child
				if index != 2 {
					t.Error("wrong NVIDIA index", index)
				}
				deadline, ok := child.Deadline()
				parentDeadline, _ := parent.Deadline()
				if !ok || !deadline.Before(parentDeadline) {
					t.Error("child consumed entire parent deadline")
				}
				switch scenario {
				case "success":
					return []byte(" 42\n"), nil
				case "zero":
					return []byte("0\n"), nil
				case "invalid":
					return []byte("bad\n"), nil
				case "error":
					return nil, errors.New("immediate failure")
				case "child-timeout":
					<-child.Done()
					if parent.Err() != nil {
						t.Error("parent expired with child")
					}
					return nil, child.Err()
				case "parent-during":
					cancel()
					<-child.Done()
					return nil, child.Err()
				case "parent-after":
					cancel()
					return nil, errors.New("reply failed after parent cancellation")
				default:
					t.Error("unexpected NVIDIA call")
					return nil, errors.New("unexpected")
				}
			}
			amd := func() float32 {
				amdCalls++
				if parent.Err() != nil {
					t.Error("AMD started after parent cancellation")
				}
				return 71
			}
			index := 2
			if scenario == "nvidia-disabled" {
				index = -1
			}
			value := sampleLightingGPU(parent, index, nvidia, amd)
			wantValue, wantNVIDIA, wantAMD := float32(71), 1, 1
			switch scenario {
			case "success":
				wantValue, wantAMD = 42, 0
			case "parent-during", "parent-after":
				wantValue, wantAMD = 0, 0
			case "already-cancelled":
				wantValue, wantNVIDIA, wantAMD = 0, 0, 0
			case "nvidia-disabled":
				wantNVIDIA = 0
			}
			if value != wantValue || nvidiaCalls != wantNVIDIA || amdCalls != wantAMD {
				t.Fatal("wrong fallback", value, nvidiaCalls, amdCalls)
			}
			if childContext != nil && childContext.Err() == nil {
				t.Fatal("child context not released")
			}
		})
	}
}

func TestNVIDIAQueryProcessStopsWithContext(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	// Signal our PID after launch, then exec the blocker without another child.
	requireLighting(t, os.WriteFile(filepath.Join(dir, "nvidia-smi"), []byte(`#!/bin/sh
printf '%s\n' "$$" > "$M65_TEST_STARTED"
exec sleep 30
`), 0700))
	t.Setenv("M65_TEST_STARTED", started)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := queryLightingNVIDIA(ctx, 0); finished <- err }()
	pid := 0
	waitCondition(t, func() bool {
		select {
		case err := <-finished:
			t.Fatalf("query exited before STARTED: %v", err)
		default:
		}
		data, err := os.ReadFile(started)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil && pid > 0
	})
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatal("signalled query was not running", err)
	}
	cancel()
	if err := receiveWithin(t, finished); err == nil {
		t.Fatal("cancelled running query reported success")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatal("query process remains after cancellation/Wait", pid, err)
	}
}
