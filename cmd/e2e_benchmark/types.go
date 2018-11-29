package main

import (
	"sync"
	"time"
)

type options struct {
	kubeconfig           string
	workerCount          int
	totalStacks          int
	format               string
	collectLogsNamespace string
	maxDuration          time.Duration
}

type phaseState struct {
	Name     string    `json:"name,omitempty"`
	DoneTime time.Time `json:"done_time,omitempty"`
}

type workerState struct {
	sync.Mutex
	ID             string       `json:"ID,omitempty"`
	CurrentPhase   string       `json:"current_phase,omitempty"`
	CurrentMessage string       `json:"current_message,omitempty"`
	PreviousPhases []phaseState `json:"previous_phases,omitempty"`
}

type timedPhase struct {
	name     string
	duration time.Duration
}

type benchmarkReport struct {
	WorkerStates []*workerState `json:"worker_states,omitempty"`
	Error        string         `json:"error,omitempty"`
	Succeeded    bool           `json:"succeeded,omitempty"`
	Start        time.Time      `json:"start,omitempty"`
}
