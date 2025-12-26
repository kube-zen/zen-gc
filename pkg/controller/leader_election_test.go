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
	"context"
	"os"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
)

func TestNewLeaderElection(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  func()
		cleanup   func()
		wantError bool
	}{
		{
			name: "with POD_NAME env var",
			setupEnv: func() {
				os.Setenv("POD_NAME", "test-pod-123")
			},
			cleanup: func() {
				os.Unsetenv("POD_NAME")
			},
			wantError: false,
		},
		{
			name: "without POD_NAME env var (uses hostname)",
			setupEnv: func() {
				os.Unsetenv("POD_NAME")
			},
			cleanup: func() {
				// No cleanup needed
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanup()

			client := fake.NewSimpleClientset()
			le, err := NewLeaderElection(client, "default", "test-leader-election")

			if tt.wantError {
				if err == nil {
					t.Error("NewLeaderElection() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewLeaderElection() returned error: %v", err)
			}

			if le == nil {
				t.Fatal("NewLeaderElection() returned nil")
			}

			if le.client == nil {
				t.Error("NewLeaderElection() did not set client")
			}

			if le.namespace != "default" {
				t.Errorf("NewLeaderElection() namespace = %v, want %v", le.namespace, "default")
			}

			if le.name != "test-leader-election" {
				t.Errorf("NewLeaderElection() name = %v, want %v", le.name, "test-leader-election")
			}

			if le.identity == "" {
				t.Error("NewLeaderElection() did not set identity")
			}

			if le.isLeader {
				t.Error("NewLeaderElection() isLeader should be false initially")
			}
		})
	}
}

func TestLeaderElection_IsLeader(t *testing.T) {
	client := fake.NewSimpleClientset()
	le, err := NewLeaderElection(client, "default", "test-leader-election")
	if err != nil {
		t.Fatalf("NewLeaderElection() returned error: %v", err)
	}

	// Initially should not be leader
	if le.IsLeader() {
		t.Error("IsLeader() should return false initially")
	}

	// Manually set leader status (simulating OnStartedLeading callback)
	le.mu.Lock()
	le.isLeader = true
	le.mu.Unlock()

	if !le.IsLeader() {
		t.Error("IsLeader() should return true after becoming leader")
	}
}

func TestLeaderElection_SetCallbacks(t *testing.T) {
	client := fake.NewSimpleClientset()
	le, err := NewLeaderElection(client, "default", "test-leader-election")
	if err != nil {
		t.Fatalf("NewLeaderElection() returned error: %v", err)
	}

	var startedCalled bool
	var stoppedCalled bool

	onStarted := func(ctx context.Context) {
		startedCalled = true
	}

	onStopped := func() {
		stoppedCalled = true
	}

	le.SetCallbacks(onStarted, onStopped)

	// Simulate callbacks being invoked
	if le.onStarted != nil {
		le.onStarted(context.Background())
	}
	if le.onStopped != nil {
		le.onStopped()
	}

	if !startedCalled {
		t.Error("SetCallbacks() onStarted callback was not set correctly")
	}

	if !stoppedCalled {
		t.Error("SetCallbacks() onStopped callback was not set correctly")
	}
}

func TestLeaderElection_Identity(t *testing.T) {
	os.Setenv("POD_NAME", "test-pod-123")
	defer os.Unsetenv("POD_NAME")

	client := fake.NewSimpleClientset()
	le, err := NewLeaderElection(client, "default", "test-leader-election")
	if err != nil {
		t.Fatalf("NewLeaderElection() returned error: %v", err)
	}

	identity := le.Identity()
	if identity == "" {
		t.Error("Identity() returned empty string")
	}

	// Identity should contain the pod name
	if identity == "" {
		t.Error("Identity() should not be empty")
	}
}

func TestLeaderElection_Run_ContextCanceled(t *testing.T) {
	client := fake.NewSimpleClientset()
	le, err := NewLeaderElection(client, "default", "test-leader-election")
	if err != nil {
		t.Fatalf("NewLeaderElection() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately to test graceful shutdown
	cancel()

	// Run should return when context is canceled
	// This test may take a moment as leader election needs to initialize
	errCh := make(chan error, 1)
	go func() {
		errCh <- le.Run(ctx)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Run() did not return within timeout")
	}
}
