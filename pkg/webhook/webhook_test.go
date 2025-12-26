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

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestWebhookServer_handleValidate(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	tests := []struct {
		name            string
		request         admissionv1.AdmissionReview
		expectedAllowed bool
		expectedCode    int
	}{
		{
			name: "valid policy",
			request: admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID: "test-uid",
					Kind: metav1.GroupVersionKind{
						Group:   "gc.kube-zen.io",
						Version: "v1alpha1",
						Kind:    "GarbageCollectionPolicy",
					},
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
							Spec: v1alpha1.GarbageCollectionPolicySpec{
								TargetResource: v1alpha1.TargetResourceSpec{
									APIVersion: "v1",
									Kind:       "ConfigMap",
								},
								TTL: v1alpha1.TTLSpec{
									SecondsAfterCreation: int64Ptr(3600),
								},
							},
						}),
					},
				},
			},
			expectedAllowed: true,
			expectedCode:    http.StatusOK,
		},
		{
			name: "invalid policy - missing TTL",
			request: admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID: "test-uid-2",
					Kind: metav1.GroupVersionKind{
						Group:   "gc.kube-zen.io",
						Version: "v1alpha1",
						Kind:    "GarbageCollectionPolicy",
					},
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
							Spec: v1alpha1.GarbageCollectionPolicySpec{
								TargetResource: v1alpha1.TargetResourceSpec{
									APIVersion: "v1",
									Kind:       "ConfigMap",
								},
								TTL: v1alpha1.TTLSpec{},
							},
						}),
					},
				},
			},
			expectedAllowed: false,
			expectedCode:    http.StatusOK,
		},
		{
			name: "invalid policy - missing apiVersion",
			request: admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID: "test-uid-3",
					Kind: metav1.GroupVersionKind{
						Group:   "gc.kube-zen.io",
						Version: "v1alpha1",
						Kind:    "GarbageCollectionPolicy",
					},
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
							Spec: v1alpha1.GarbageCollectionPolicySpec{
								TargetResource: v1alpha1.TargetResourceSpec{
									Kind: "ConfigMap",
								},
								TTL: v1alpha1.TTLSpec{
									SecondsAfterCreation: int64Ptr(3600),
								},
							},
						}),
					},
				},
			},
			expectedAllowed: false,
			expectedCode:    http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/validate-gc-policy", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleValidate(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			var response admissionv1.AdmissionReview
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if response.Response == nil {
				t.Fatal("Response is nil")
			}

			if response.Response.Allowed != tt.expectedAllowed {
				errorMsg := ""
				if response.Response.Result != nil {
					errorMsg = response.Response.Result.Message
				}
				t.Errorf("Expected allowed=%v, got %v. Error: %s", tt.expectedAllowed, response.Response.Allowed, errorMsg)
			}

			if response.Response.UID != tt.request.Request.UID {
				t.Errorf("Expected UID %s, got %s", tt.request.Request.UID, response.Response.UID)
			}
		})
	}
}

func TestWebhookServer_handleValidate_InvalidMethod(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/validate-gc-policy", http.NoBody)
	w := httptest.NewRecorder()

	server.handleValidate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestWebhookServer_handleValidate_InvalidJSON(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/validate-gc-policy", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleValidate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWebhookServer_handleMutate_InvalidJSON(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate-gc-policy", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleMutate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWebhookServer_handleMutate_InvalidMethod(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/mutate-gc-policy", http.NoBody)
	w := httptest.NewRecorder()

	server.handleMutate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestWebhookServer_Start(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	// Wait for context to cancel
	<-ctx.Done()

	// Check if server stopped gracefully
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Server did not stop within timeout")
	}
}

func marshalPolicy(t *testing.T, policy *v1alpha1.GarbageCollectionPolicy) []byte {
	t.Helper()
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Failed to marshal policy: %v", err)
	}
	return data
}

func int64Ptr(i int64) *int64 {
	return &i
}

func TestWebhookServer_handleMutate(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	tests := []struct {
		name            string
		method          string
		request         admissionv1.AdmissionReview
		expectedAllowed bool
		expectedCode    int
	}{
		{
			name:   "valid CREATE mutation",
			method: http.MethodPost,
			request: admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID:       "test-uid",
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-policy",
								Namespace: "default",
							},
							Spec: v1alpha1.GarbageCollectionPolicySpec{
								TargetResource: v1alpha1.TargetResourceSpec{
									APIVersion: "v1",
									Kind:       "ConfigMap",
								},
							},
						}),
					},
				},
			},
			expectedAllowed: true,
			expectedCode:    http.StatusOK,
		},
		{
			name:   "UPDATE operation (no mutation)",
			method: http.MethodPost,
			request: admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID:       "test-uid",
					Operation: admissionv1.Update,
					Object: runtime.RawExtension{
						Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-policy",
								Namespace: "default",
							},
							Spec: v1alpha1.GarbageCollectionPolicySpec{
								TargetResource: v1alpha1.TargetResourceSpec{
									APIVersion: "v1",
									Kind:       "ConfigMap",
								},
							},
						}),
					},
				},
			},
			expectedAllowed: true,
			expectedCode:    http.StatusOK,
		},
		{
			name:            "invalid method",
			method:          http.MethodGet,
			request:         admissionv1.AdmissionReview{},
			expectedAllowed: false,
			expectedCode:    http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(tt.method, "/mutate-gc-policy", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleMutate(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			if tt.expectedCode == http.StatusOK {
				var response admissionv1.AdmissionReview
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if response.Response.Allowed != tt.expectedAllowed {
					t.Errorf("Expected allowed=%v, got %v", tt.expectedAllowed, response.Response.Allowed)
				}
			}
		})
	}
}

func TestWebhookServer_mutatePolicy(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	tests := []struct {
		name            string
		request         *admissionv1.AdmissionRequest
		expectedPatches int
		expectError     bool
	}{
		{
			name: "CREATE with defaults needed",
			request: &admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Object: runtime.RawExtension{
					Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-policy",
							Namespace: "default",
						},
						Spec: v1alpha1.GarbageCollectionPolicySpec{
							TargetResource: v1alpha1.TargetResourceSpec{
								APIVersion: "v1",
								Kind:       "ConfigMap",
							},
						},
					}),
				},
			},
			expectedPatches: 2, // behavior defaults + namespace default
			expectError:     false,
		},
		{
			name: "UPDATE operation (no mutation)",
			request: &admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Object: runtime.RawExtension{
					Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-policy",
							Namespace: "default",
						},
						Spec: v1alpha1.GarbageCollectionPolicySpec{
							TargetResource: v1alpha1.TargetResourceSpec{
								APIVersion: "v1",
								Kind:       "ConfigMap",
							},
						},
					}),
				},
			},
			expectedPatches: 0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches, err := server.mutatePolicy(tt.request)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(patches) != tt.expectedPatches {
				t.Errorf("Expected %d patches, got %d", tt.expectedPatches, len(patches))
			}
		})
	}
}

func TestWebhookServer_mutatePolicy_WithExistingBehavior(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("NewWebhookServer() returned error: %v", err)
	}

	request := &admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					Behavior: v1alpha1.BehaviorSpec{
						MaxDeletionsPerSecond: 20,  // Already set
						BatchSize:             100, // Already set
						// PropagationPolicy not set
					},
				},
			}),
		},
	}

	patches, err := server.mutatePolicy(request)
	if err != nil {
		t.Errorf("mutatePolicy() returned error: %v", err)
	}

	// Should add PropagationPolicy default
	if len(patches) != 2 { // PropagationPolicy + namespace
		t.Errorf("Expected 2 patches, got %d", len(patches))
	}
}

func TestWebhookServer_mutatePolicy_WithNamespaceSet(t *testing.T) {
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("NewWebhookServer() returned error: %v", err)
	}

	request := &admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: marshalPolicy(t, &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "kube-system", // Already set
					},
				},
			}),
		},
	}

	patches, err := server.mutatePolicy(request)
	if err != nil {
		t.Errorf("mutatePolicy() returned error: %v", err)
	}

	// Should add behavior defaults but not namespace (already set)
	if len(patches) != 1 { // Only behavior defaults
		t.Errorf("Expected 1 patch (behavior), got %d", len(patches))
	}
}

func TestWebhookServer_init(t *testing.T) {
	// Test that init() function runs without error
	// This is tested implicitly by creating a webhook server
	server, err := NewWebhookServer(":0", "", "")
	if err != nil {
		t.Fatalf("NewWebhookServer() returned error: %v", err)
	}
	if server == nil {
		t.Fatal("NewWebhookServer() returned nil")
	}
	// Codecs is initialized by init() - if we can create a server, init() worked
	// Codecs is a value type (serializer.CodecFactory), not a pointer, so we can't compare it to nil
	// We just verify it's accessible by using it
	if Codecs.UniversalDeserializer() == nil {
		t.Fatal("Codecs.UniversalDeserializer() returned nil")
	}
}

func TestWebhookServer_StartTLS(t *testing.T) {
	// Create a temporary directory for cert files
	tmpDir := t.TempDir()
	certFile := tmpDir + "/cert.pem"
	keyFile := tmpDir + "/key.pem"

	// Create dummy cert files (empty files for testing)
	// These are invalid certs, so StartTLS will fail with TLS error
	// This test verifies that StartTLS attempts to start and handles errors
	if err := os.WriteFile(certFile, []byte("dummy cert"), 0o600); err != nil {
		t.Fatalf("Failed to create cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("dummy key"), 0o600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	server, err := NewWebhookServer(":0", certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.StartTLS(ctx, certFile, keyFile)
	}()

	// Wait for error (expected due to invalid cert)
	select {
	case err := <-errCh:
		// Expected: TLS error due to invalid cert data
		// This verifies that StartTLS attempts to start the server
		if err == nil {
			t.Error("StartTLS() should return error with invalid cert file")
		}
		// Check that error is about TLS/certificate (not server closed)
		if errors.Is(err, http.ErrServerClosed) {
			t.Error("StartTLS() should not return ErrServerClosed with invalid cert")
		}
	case <-time.After(2 * time.Second):
		// If no error after 2 seconds, cancel context to trigger shutdown
		cancel()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				// Accept TLS errors as expected
				if err.Error() == "" {
					t.Error("StartTLS() returned empty error")
				}
			}
		case <-time.After(1 * time.Second):
			t.Error("StartTLS() did not return within timeout after cancel")
		}
	}
}

func TestWebhookServer_StartTLS_InvalidCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := tmpDir + "/nonexistent.pem"
	keyFile := tmpDir + "/key.pem"

	// Create only key file (cert file missing)
	if err := os.WriteFile(keyFile, []byte("dummy key"), 0o600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	server, err := NewWebhookServer(":0", certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to create webhook server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// StartTLS should fail with invalid cert file
	err = server.StartTLS(ctx, certFile, keyFile)
	if err == nil {
		t.Error("StartTLS() should return error with invalid cert file")
	}
}
