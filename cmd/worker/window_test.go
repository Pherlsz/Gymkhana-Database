package main

import (
	"testing"
	"time"
)

func TestWorkerRunWindowAllowsContinuousLocalWorker(t *testing.T) {
	duration, bounded, err := workerRunWindow(environment(nil))
	if err != nil || bounded || duration != 0 {
		t.Fatalf("workerRunWindow() = %s, %t, %v", duration, bounded, err)
	}
}

func TestWorkerRunWindowRequiresDurationForCloudRunJob(t *testing.T) {
	_, _, err := workerRunWindow(environment(map[string]string{"CLOUD_RUN_JOB": "gymkhana-worker"}))
	if err == nil {
		t.Fatal("workerRunWindow() error = nil")
	}
}

func TestWorkerRunWindowAcceptsBoundedDuration(t *testing.T) {
	duration, bounded, err := workerRunWindow(environment(map[string]string{
		"CLOUD_RUN_JOB":      "gymkhana-worker",
		"WORKER_RUN_DURATION": "14m",
	}))
	if err != nil || !bounded || duration != 14*time.Minute {
		t.Fatalf("workerRunWindow() = %s, %t, %v", duration, bounded, err)
	}
}

func TestWorkerRunWindowRejectsUnsafeDurations(t *testing.T) {
	for _, value := range []string{"invalid", "30s", "168h"} {
		t.Run(value, func(t *testing.T) {
			_, _, err := workerRunWindow(environment(map[string]string{"WORKER_RUN_DURATION": value}))
			if err == nil {
				t.Fatal("workerRunWindow() error = nil")
			}
		})
	}
}

func environment(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
}
