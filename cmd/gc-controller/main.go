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

// Package main implements the GC controller command-line application.
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-gc/pkg/controller"
	"github.com/kube-zen/zen-gc/pkg/webhook"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown.
	DefaultShutdownTimeout = 30 * time.Second

	// DefaultBatchSize is the default batch size for deletions.
	DefaultBatchSize = 50

	// DefaultMaxConcurrentEvaluations is the default maximum number of concurrent policy evaluations.
	DefaultMaxConcurrentEvaluations = 5
)

var (
	// Version information (set via build flags).
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

var (
	kubeconfig               = flag.String("kubeconfig", "", "Path to kubeconfig file. If not set, uses in-cluster config")
	masterURL                = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig")
	metricsAddr              = flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to")
	webhookAddr              = flag.String("webhook-addr", ":9443", "The address the webhook endpoint binds to")
	webhookCertFile          = flag.String("webhook-cert-file", "/etc/webhook/certs/tls.crt", "Path to TLS certificate file")
	webhookKeyFile           = flag.String("webhook-key-file", "/etc/webhook/certs/tls.key", "Path to TLS private key file")
	enableLeaderElection     = flag.Bool("enable-leader-election", true, "Enable leader election for HA")
	leaderElectionNS         = flag.String("leader-election-namespace", "", "Namespace for leader election lease (defaults to POD_NAMESPACE)")
	enableWebhook            = flag.Bool("enable-webhook", true, "Enable validating webhook server")
	insecureWebhook          = flag.Bool("insecure-webhook", false, "Allow webhook to start without TLS (testing only, not recommended for production)")
	gcInterval               = flag.Duration("gc-interval", 1*time.Minute, "Interval between GC evaluation runs")
	maxDeletionsPerSecond    = flag.Int("max-deletions-per-second", 10, "Default maximum deletions per second (can be overridden per policy)")
	batchSize                = flag.Int("batch-size", DefaultBatchSize, "Default batch size for deletions (can be overridden per policy)")
	maxConcurrentEvaluations = flag.Int("max-concurrent-evaluations", DefaultMaxConcurrentEvaluations, "Maximum number of policies to evaluate concurrently")
)

//nolint:gocyclo // main function complexity is acceptable for initialization logic
func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Set up signals so we handle shutdown gracefully
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Build config
	cfg, err := buildConfig(*masterURL, *kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building dynamic client: %v", err)
	}

	// Create Kubernetes client for leader election and events
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building Kubernetes client: %v", err)
	}

	// Add GarbageCollectionPolicy to scheme
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		klog.Fatalf("Error adding scheme: %v", err)
	}

	// Get namespace for leader election
	namespace := *leaderElectionNS
	if namespace == "" {
		namespace = os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			// Try to read from service account namespace file
			if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
				namespace = string(ns)
			} else {
				namespace = "gc-system"
			}
		}
	}

	// Load controller configuration
	controllerConfig := config.NewControllerConfig()
	controllerConfig.LoadFromEnv() // Load from environment variables
	controllerConfig.WithGCInterval(*gcInterval)
	controllerConfig.WithMaxDeletionsPerSecond(*maxDeletionsPerSecond)
	controllerConfig.WithBatchSize(*batchSize)
	controllerConfig.WithMaxConcurrentEvaluations(*maxConcurrentEvaluations)

	klog.Infof("Controller configuration: GCInterval=%v, MaxDeletionsPerSecond=%d, BatchSize=%d, MaxConcurrentEvaluations=%d",
		controllerConfig.GCInterval, controllerConfig.MaxDeletionsPerSecond, controllerConfig.BatchSize, controllerConfig.MaxConcurrentEvaluations)

	// Create status updater with configuration
	statusUpdater := controller.NewStatusUpdaterWithConfig(dynamicClient, controllerConfig)

	// Create event recorder
	eventRecorder := controller.NewEventRecorder(kubeClient)

	// Create GC controller with configuration
	gcController, err := controller.NewGCControllerWithConfig(dynamicClient, statusUpdater, eventRecorder, controllerConfig)
	if err != nil {
		klog.Fatalf("Error creating GC controller: %v", err)
	}

	// Setup leader election if enabled (needed for metrics server readiness check)
	var leaderElection *controller.LeaderElection
	if *enableLeaderElection {
		leaderElection, err = controller.NewLeaderElection(kubeClient, namespace, "gc-controller-leader-election")
		if err != nil {
			klog.Fatalf("Error creating leader election: %v", err)
		}
	}

	// Start metrics server (pass leaderElection for readiness check)
	go startMetricsServer(*metricsAddr, leaderElection)

	// Start webhook server if enabled
	var webhookServer *webhook.WebhookServer
	if *enableWebhook {
		var err error
		webhookServer, err = webhook.NewWebhookServer(*webhookAddr, *webhookCertFile, *webhookKeyFile)
		if err != nil {
			klog.Fatalf("Error creating webhook server: %v", err)
		}

		// Check if TLS files exist
		certExists := false
		keyExists := false
		if _, err := os.Stat(*webhookCertFile); err == nil {
			certExists = true
		}
		if _, err := os.Stat(*webhookKeyFile); err == nil {
			keyExists = true
		}

		if certExists && keyExists {
			// TLS files exist, start with TLS
			go func() {
				if err := webhookServer.StartTLS(ctx, *webhookCertFile, *webhookKeyFile); err != nil {
					klog.Fatalf("Error starting webhook server: %v", err)
				}
			}()
			klog.Infof("Webhook server starting with TLS on %s", *webhookAddr)
		} else {
			// TLS files missing - check if insecure mode is allowed
			if !*insecureWebhook {
				klog.Fatalf("Webhook TLS certificates not found (cert: %s, key: %s). TLS is required for production. Use --insecure-webhook flag only for testing.", *webhookCertFile, *webhookKeyFile)
			}
			klog.Warningf("Webhook starting without TLS (insecure mode) - NOT RECOMMENDED FOR PRODUCTION")
			go func() {
				if err := webhookServer.Start(ctx); err != nil {
					klog.Fatalf("Error starting webhook server: %v", err)
				}
			}()
		}
	}

	// Setup leader election callbacks if enabled
	if *enableLeaderElection {
		if leaderElection == nil {
			klog.Fatalf("Leader election not initialized")
		}

		// Set callbacks
		leaderElection.SetCallbacks(
			func(ctx context.Context) {
				// Started leading - start controller
				if err := gcController.Start(); err != nil {
					klog.Fatalf("Error starting GC controller: %v", err)
				}
				klog.Info("GC controller started (leader)")
			},
			func() {
				// Stopped leading - stop controller
				gcController.Stop()
				klog.Info("GC controller stopped (lost leadership)")
			},
		)

		// Run leader election (blocks until context is canceled).
		klog.Info("Starting leader election...")
		go func() {
			if err := leaderElection.Run(ctx); err != nil {
				klog.Fatalf("Leader election error: %v", err)
			}
		}()

		klog.Info("Waiting for leadership...")
	} else {
		// No leader election - start controller directly
		if err := gcController.Start(); err != nil {
			klog.Fatalf("Error starting GC controller: %v", err)
		}
		klog.Info("GC controller is running (no leader election). Press Ctrl+C to stop.")
	}

	// Wait for shutdown signal
	<-ctx.Done()
	klog.Info("Shutdown signal received, initiating graceful shutdown...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer shutdownCancel()

	// Stop controller gracefully (with timeout)
	done := make(chan struct{})
	go func() {
		gcController.Stop()
		close(done)
	}()

	select {
	case <-done:
		klog.Info("Controller stopped successfully")
	case <-shutdownCtx.Done():
		klog.Warning("Controller shutdown timed out, forcing exit")
	}

	// Webhook server shutdown is handled automatically via context cancellation
	// No explicit Stop() call needed

	klog.Info("GC controller shutdown complete")
}

// buildConfig builds a Kubernetes config from the given master URL and kubeconfig path.
func buildConfig(masterURL, kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		// Try in-cluster config first
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}
		// Fall back to default kubeconfig location
		kubeconfigPath = clientcmd.RecommendedHomeFile
	}

	return clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
}

// startMetricsServer starts the Prometheus metrics server.
func startMetricsServer(addr string, leaderElection *controller.LeaderElection) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// If leader election is enabled, only the leader should be ready
		if leaderElection != nil && !leaderElection.IsLeader() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Not leader"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"` + version + `","commit":"` + commit + `","buildDate":"` + buildDate + `"}`))
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	klog.Infof("Starting metrics server on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		klog.Fatalf("Error starting metrics server: %v", err)
	}
}
