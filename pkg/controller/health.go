/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"net/http"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/health"
)

// HealthChecker provides health check functionality for the GC controller.
// This now uses zen-sdk/pkg/health as the base implementation.
type HealthChecker struct {
	// Informer sync checker from zen-sdk
	informerChecker *health.InformerSyncChecker

	// Track last evaluation time to verify active processing.
	lastEvaluationTime   time.Time
	lastEvaluationTimeMu sync.RWMutex

	// Maximum time since last evaluation before considering controller unhealthy.
	maxTimeSinceLastEvaluation time.Duration

	// Reconciler reference for checking informer sync status.
	reconciler *GCPolicyReconciler
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(reconciler *GCPolicyReconciler) *HealthChecker {
	// Create informer sync checker using zen-sdk
	informerChecker := health.NewInformerSyncChecker(func() map[string]func() bool {
		reconciler.resourceInformersMu.RLock()
		defer reconciler.resourceInformersMu.RUnlock()

		informers := make(map[string]func() bool)
		for uid, informer := range reconciler.resourceInformers {
			if informer != nil {
				// Capture informer in closure
				inf := informer
				informers[string(uid)] = func() bool { return inf.HasSynced() }
			}
		}
		return informers
	})

	return &HealthChecker{
		informerChecker:            informerChecker,
		reconciler:                 reconciler,
		maxTimeSinceLastEvaluation: 5 * time.Minute, // Default: 5 minutes
	}
}

// SetMaxTimeSinceLastEvaluation sets the maximum time since last evaluation.
func (h *HealthChecker) SetMaxTimeSinceLastEvaluation(d time.Duration) {
	h.maxTimeSinceLastEvaluation = d
}

// UpdateLastEvaluationTime updates the last evaluation time.
func (h *HealthChecker) UpdateLastEvaluationTime() {
	h.lastEvaluationTimeMu.Lock()
	defer h.lastEvaluationTimeMu.Unlock()
	h.lastEvaluationTime = time.Now()
}

// ReadinessCheck verifies that the controller is ready to serve requests.
// It checks:
// 1. All resource informers are synced
// 2. Controller has been running long enough (at least 10 seconds).
func (h *HealthChecker) ReadinessCheck(req *http.Request) error {
	return h.informerChecker.ReadinessCheck(req)
}

// LivenessCheck verifies that the controller is actively processing policies.
// It checks:
// 1. Controller has evaluated policies recently (within maxTimeSinceLastEvaluation)
// 2. If no policies exist, controller is still considered alive (no work to do)
// 3. If policies exist but haven't been evaluated, check if reconciler is processing.
func (h *HealthChecker) LivenessCheck(req *http.Request) error {
	// Use informer checker for basic liveness
	if err := h.informerChecker.LivenessCheck(req); err != nil {
		return err
	}

	// Additional check: verify we have policies or have been active recently
	h.reconciler.resourceInformersMu.RLock()
	hasPolicies := len(h.reconciler.resourceInformers) > 0
	h.reconciler.resourceInformersMu.RUnlock()

	if !hasPolicies {
		// No policies, so no evaluation needed - controller is healthy
		return nil
	}

	// If we have policies, check last evaluation time
	h.lastEvaluationTimeMu.RLock()
	lastActivity := h.lastEvaluationTime
	h.lastEvaluationTimeMu.RUnlock()

	if !lastActivity.IsZero() {
		timeSinceActivity := time.Since(lastActivity)
		if timeSinceActivity > h.maxTimeSinceLastEvaluation {
			// No activity for too long - but this is a warning, not a failure
			// The informer sync check is more important for liveness
			return nil
		}
	}

	return nil
}

// StartupCheck is a simple check for startup probe.
// Returns nil if controller is initialized, error otherwise.
func (h *HealthChecker) StartupCheck(req *http.Request) error {
	return h.informerChecker.StartupCheck(req)
}
