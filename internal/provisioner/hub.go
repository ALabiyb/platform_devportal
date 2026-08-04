
// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package provisioner contains the SSE broadcast hub and the 15-step
// provisioning orchestrator that wires GitLab, Jenkins, Harbor, DefectDojo,
// ArgoCD, and PostgreSQL together for a single developer project.
package provisioner

import (
	"sync"

	"github.com/google/uuid"
)

// StepEvent is one event broadcast to every SSE subscriber watching a project's
// provisioning progress. It matches the JSON shape the browser expects.
type StepEvent struct {
	StepIndex int    `json:"step_index"`
	Label     string `json:"label"`
	Status    string `json:"status"`           // "pending" | "running" | "done" | "failed"
	Detail    string `json:"detail,omitempty"` // error message or success note
	Done      bool   `json:"done,omitempty"`   // true on the final event (success or failure)
}

// Hub manages SSE subscriber channels keyed by project ID.
// Multiple browser tabs can watch the same project simultaneously.
// Broadcast drops events for slow subscribers rather than blocking the orchestrator.
type Hub struct {
	mu   sync.Mutex
	subs map[uuid.UUID][]chan StepEvent
}

// NewHub constructs an empty Hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[uuid.UUID][]chan StepEvent)}
}

// Subscribe registers a new subscriber for the given project and returns
// a receive-only channel and an unsubscribe function.
// The caller must call unsubscribe() when done (e.g. via defer) to release
// the channel and remove it from the hub.
func (h *Hub) Subscribe(projectID uuid.UUID) (<-chan StepEvent, func()) {
	ch := make(chan StepEvent, 32) // buffered: orchestrator never blocks on slow clients
	h.mu.Lock()
	h.subs[projectID] = append(h.subs[projectID], ch)
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[projectID]
		for i, c := range list {
			if c == ch {
				h.subs[projectID] = append(list[:i], list[i+1:]...)
				break
			}
		}
		close(ch)
		if len(h.subs[projectID]) == 0 {
			delete(h.subs, projectID)
		}
	}
	return ch, unsubscribe
}

// Broadcast sends event to every subscriber watching projectID.
// Uses a non-blocking send so a slow browser tab can't stall the orchestrator.
func (h *Hub) Broadcast(projectID uuid.UUID, event StepEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs[projectID] {
		select {
		case ch <- event:
		default: // subscriber is slow; drop this event rather than block
		}
	}
}
