package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

const maximumWorkerRunDuration = 167 * time.Hour

func init() {
	duration, bounded, err := workerRunWindow(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "worker run-window configuration invalid:", err)
		os.Exit(2)
	}
	if !bounded {
		return
	}
	go func() {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		<-timer.C
		process, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = process.Signal(syscall.SIGTERM)
		}
	}()
}

func workerRunWindow(lookup func(string) (string, bool)) (time.Duration, bool, error) {
	jobName, cloudRunJob := lookup("CLOUD_RUN_JOB")
	jobName = strings.TrimSpace(jobName)
	rawDuration, configured := lookup("WORKER_RUN_DURATION")
	rawDuration = strings.TrimSpace(rawDuration)
	if !configured || rawDuration == "" {
		if cloudRunJob && jobName != "" {
			return 0, false, errors.New("WORKER_RUN_DURATION is required for Cloud Run jobs")
		}
		return 0, false, nil
	}
	duration, err := time.ParseDuration(rawDuration)
	if err != nil {
		return 0, false, fmt.Errorf("parse WORKER_RUN_DURATION: %w", err)
	}
	if duration < time.Minute {
		return 0, false, errors.New("WORKER_RUN_DURATION must be at least 1m")
	}
	if duration > maximumWorkerRunDuration {
		return 0, false, fmt.Errorf("WORKER_RUN_DURATION cannot exceed %s", maximumWorkerRunDuration)
	}
	return duration, true, nil
}
