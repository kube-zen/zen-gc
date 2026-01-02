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

// Package webhook provides HTTP server for validating and mutating admission webhooks.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/validation"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

var (
	// Codecs is the codec factory for deserializing admission requests.
	Codecs = serializer.NewCodecFactory(scheme.Scheme)

	// ErrUnexpectedObjectType indicates an unexpected object type was encountered.
	ErrUnexpectedObjectType = errors.New("expected GarbageCollectionPolicy")
)

func init() {
	// Add GarbageCollectionPolicy to scheme for deserialization
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Sprintf("failed to add scheme: %v", err))
	}
}

// Server handles admission webhook requests.
//
//nolint:revive // Renaming would be a breaking change
type WebhookServer struct {
	server *http.Server
}

// NewServer creates a new webhook server.
//
//nolint:revive // Keep for backward compatibility
func NewWebhookServer(addr, certFile, keyFile string) (*WebhookServer, error) {
	mux := http.NewServeMux()
	ws := &WebhookServer{}

	// Register validation endpoint
	mux.HandleFunc("/validate-gc-policy", ws.handleValidate)

	// Register mutation endpoint
	mux.HandleFunc("/mutate-gc-policy", ws.handleMutate)

	// Health check endpoint for webhook
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	ws.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return ws, nil
}

// Start starts the webhook server without TLS (for testing).
func (ws *WebhookServer) Start(ctx context.Context) error {
	logger := sdklog.NewLogger("zen-gc-webhook")
	logger.Info("Starting webhook server without TLS (testing mode)...")

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down webhook server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ws.server.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "Error shutting down webhook server")
		}
	}()

	if err := ws.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("webhook server error: %w", err)
	}

	return nil
}

// StartTLS starts the webhook server with TLS.
func (ws *WebhookServer) StartTLS(ctx context.Context, certFile, keyFile string) error {
	logger := sdklog.NewLogger("zen-gc-webhook")
	logger.Info("Starting webhook server with TLS", sdklog.String("address", ws.server.Addr))

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down webhook server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ws.server.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "Error shutting down webhook server")
		}
	}()

	if err := ws.server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("webhook server error: %w", err)
	}

	return nil
}

// handleValidate handles admission review requests for GarbageCollectionPolicy validation.
func (ws *WebhookServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read admission review request
	logger := sdklog.NewLogger("zen-gc-webhook")
	var review admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		logger.Error(err, "Failed to decode admission review")
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// Set response UID
	response := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID: review.Request.UID,
		},
	}

	// Validate the policy
	if err := ws.validatePolicy(review.Request); err != nil {
		logger.Debug("Policy validation failed", sdklog.String("error", err.Error()))
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    http.StatusUnprocessableEntity,
			Message: err.Error(),
		}
	} else {
		response.Response.Allowed = true
		logger.Debug("Policy validation succeeded")
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error(err, "Failed to encode admission review response")
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// validatePolicy validates a GarbageCollectionPolicy from an admission request.
func (ws *WebhookServer) validatePolicy(req *admissionv1.AdmissionRequest) error {
	// Only validate CREATE and UPDATE operations
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return nil
	}

	// Deserialize the object
	var policy v1alpha1.GarbageCollectionPolicy
	decoder := Codecs.UniversalDeserializer()

	// For CREATE and UPDATE operations, use Object
	rawObj := req.Object

	obj, _, err := decoder.Decode(rawObj.Raw, nil, &policy)
	if err != nil {
		return fmt.Errorf("failed to decode GarbageCollectionPolicy: %w", err)
	}

	policyObj, ok := obj.(*v1alpha1.GarbageCollectionPolicy)
	if !ok {
		return fmt.Errorf("%w, got %T", ErrUnexpectedObjectType, obj)
	}

	// Validate the policy using the validation package
	if err := validation.ValidatePolicy(policyObj); err != nil {
		return fmt.Errorf("policy validation failed: %w", err)
	}

	return nil
}

// handleMutate handles admission review requests for GarbageCollectionPolicy mutation (defaults).
func (ws *WebhookServer) handleMutate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read admission review request
	logger := sdklog.NewLogger("zen-gc-webhook")
	var review admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		logger.Error(err, "Failed to decode admission review")
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// Set response UID
	response := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID: review.Request.UID,
		},
	}

	// Mutate the policy (set defaults)
	patches, err := ws.mutatePolicy(review.Request)
	if err != nil {
		logger.Error(err, "Policy mutation failed")
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to mutate policy: %v", err),
		}
	} else {
		response.Response.Allowed = true
		if len(patches) > 0 {
			patchBytes, err := json.Marshal(patches)
			if err != nil {
				logger.Error(err, "Failed to marshal patches")
				response.Response.Allowed = false
				response.Response.Result = &metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: fmt.Sprintf("Failed to marshal patches: %v", err),
				}
			} else {
				response.Response.Patch = patchBytes
				response.Response.PatchType = func() *admissionv1.PatchType {
					pt := admissionv1.PatchTypeJSONPatch
					return &pt
				}()
				logger.Debug("Policy mutation succeeded", sdklog.Int("patches", len(patches)))
			}
		} else {
			logger.Debug("Policy mutation succeeded (no patches needed)")
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error(err, "Failed to encode admission review response")
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// mutatePolicy mutates a GarbageCollectionPolicy to set default values.
func (ws *WebhookServer) mutatePolicy(req *admissionv1.AdmissionRequest) ([]map[string]interface{}, error) {
	// Only mutate CREATE operations
	if req.Operation != admissionv1.Create {
		return nil, nil
	}

	// Deserialize the object
	var policy v1alpha1.GarbageCollectionPolicy
	decoder := Codecs.UniversalDeserializer()

	obj, _, err := decoder.Decode(req.Object.Raw, nil, &policy)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GarbageCollectionPolicy: %w", err)
	}

	policyObj, ok := obj.(*v1alpha1.GarbageCollectionPolicy)
	if !ok {
		return nil, fmt.Errorf("%w, got %T", ErrUnexpectedObjectType, obj)
	}

	// Collect patches for default values
	var patches []map[string]interface{}

	// Ensure behavior spec exists
	behaviorPath := "/spec/behavior"
	hasBehavior := policyObj.Spec.Behavior.MaxDeletionsPerSecond != 0 ||
		policyObj.Spec.Behavior.BatchSize != 0 ||
		policyObj.Spec.Behavior.DryRun ||
		policyObj.Spec.Behavior.Finalizer != "" ||
		policyObj.Spec.Behavior.PropagationPolicy != "" ||
		policyObj.Spec.Behavior.GracePeriodSeconds != nil

	// Set default behavior values if not specified
	if !hasBehavior {
		// Create behavior object with defaults
		patches = append(patches, map[string]interface{}{
			"op":   "add",
			"path": behaviorPath,
			"value": map[string]interface{}{
				"maxDeletionsPerSecond": 10,
				"batchSize":             50,
				"propagationPolicy":     "Background",
			},
		})
	} else {
		// Set individual defaults if behavior exists but fields are missing
		if policyObj.Spec.Behavior.MaxDeletionsPerSecond == 0 {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  behaviorPath + "/maxDeletionsPerSecond",
				"value": 10,
			})
		}
		if policyObj.Spec.Behavior.BatchSize == 0 {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  behaviorPath + "/batchSize",
				"value": 50,
			})
		}
		if policyObj.Spec.Behavior.PropagationPolicy == "" {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  behaviorPath + "/propagationPolicy",
				"value": "Background",
			})
		}
	}

	// Set default namespace to "*" if not specified (for cluster-wide policies)
	if policyObj.Spec.TargetResource.Namespace == "" {
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  "/spec/targetResource/namespace",
			"value": "*",
		})
	}

	return patches, nil
}
