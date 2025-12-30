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
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-gc/pkg/controller"
	gcwebhook "github.com/kube-zen/zen-gc/pkg/webhook"
	"github.com/kube-zen/zen-sdk/pkg/leader"
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

func init() {
	// Log version information at startup
	klog.V(2).Infof("GC Controller version: %s, commit: %s, build date: %s", version, commit, buildDate)
}

var (
	metricsAddr              = flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to")
	webhookAddr              = flag.String("webhook-addr", ":9443", "The address the webhook endpoint binds to")
	webhookCertFile          = flag.String("webhook-cert-file", "/etc/webhook/certs/tls.crt", "Path to TLS certificate file")
	webhookKeyFile           = flag.String("webhook-key-file", "/etc/webhook/certs/tls.key", "Path to TLS private key file")
	haMode                   = flag.String("ha-mode", "internal", "HA Mode: none (no leader election), internal (zen-sdk/pkg/leader), or external (zen-lead controller)")
	enableLeaderElection     = flag.Bool("enable-leader-election", true, "Enable leader election for HA (deprecated: use --ha-mode instead)")
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

	// Get config using controller-runtime (handles kubeconfig flag automatically)
	restCfg := ctrl.GetConfigOrDie()

	// Create dynamic client (still needed for resource informers)
	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		klog.Fatalf("Error building dynamic client: %v", err)
	}

	// Create Kubernetes client for events
	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		klog.Fatalf("Error building Kubernetes client: %v", err)
	}

	// Create scheme and add GarbageCollectionPolicy types
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		klog.Fatalf("Error adding scheme: %v", err)
	}

	// Determine HA mode (support legacy --enable-leader-election flag)
	haModeValue := *haMode
	if *enableLeaderElection == false && haModeValue == "internal" {
		// Legacy flag takes precedence
		haModeValue = "none"
		klog.Warningf("--enable-leader-election=false is deprecated. Use --ha-mode=none instead.")
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

	// Configure leader election based on HA mode
	var externalWatcher *leader.Watcher
	var shouldReconcile func() bool

	switch haModeValue {
	case "none":
		shouldReconcile = func() bool { return true } // Always reconcile
		klog.Warningf("Running in SINGLE mode (no HA). Accepting split-brain risk. Not recommended for production.")
	case "internal":
		shouldReconcile = func() bool { return true } // Manager handles leader election
		klog.Infof("Running in INTERNAL mode (built-in leader election via zen-sdk/pkg/leader)")
	case "external":
		shouldReconcile = func() bool {
			if externalWatcher == nil {
				return false // Not initialized yet
			}
			return externalWatcher.GetIsLeader()
		}
		klog.Infof("Running in EXTERNAL mode (zen-lead controller). Waiting for leader election...")
	default:
		klog.Fatalf("Invalid --ha-mode: %s. Must be 'none', 'internal', or 'external'", haModeValue)
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

	// Create controller-runtime Manager with leader election configuration
	mgrOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: *metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: "", // We'll handle webhook separately for now
		}),
		HealthProbeBindAddress: ":8081", // Health probes on separate port (controller-runtime requirement)
	}

	// Configure leader election based on mode
	if haModeValue == "internal" {
		leaderOpts := leader.Options{
			LeaseName: "gc-controller-leader-election",
			Enable:    true,
			Namespace: namespace,
		}
		mgrOpts = leader.ManagerOptions(mgrOpts, leaderOpts)
	} else {
		// none or external: no internal leader election
		mgrOpts.LeaderElection = false
	}

	mgr, err := ctrl.NewManager(restCfg, mgrOpts)
	if err != nil {
		klog.Fatalf("Error creating controller manager: %v", err)
	}

	// Setup external watcher for external mode (must be done after manager is created)
	if haModeValue == "external" {
		watcher, err := leader.NewWatcher(mgr.GetClient(), func(isLeader bool) {
			if isLeader {
				klog.Infof("Elected as leader via zen-lead. Starting reconciliation...")
			} else {
				klog.Infof("Lost leadership via zen-lead. Pausing reconciliation...")
			}
		})
		if err != nil {
			klog.Fatalf("Error creating external leader watcher: %v", err)
		}
		externalWatcher = watcher

		// Start watching in background
		go func() {
			if err := watcher.Watch(ctx); err != nil && err != context.Canceled {
				klog.Errorf("Error watching leader status: %v", err)
			}
		}()
	}

	// Create GC policy reconciler with leader check function for external mode
	var reconciler *controller.GCPolicyReconciler
	if haModeValue == "external" {
		reconciler = controller.NewGCPolicyReconcilerWithLeaderCheck(
			mgr.GetClient(),
			mgr.GetScheme(),
			dynamicClient,
			statusUpdater,
			eventRecorder,
			controllerConfig,
			shouldReconcile,
		)
	} else {
		reconciler = controller.NewGCPolicyReconciler(
			mgr.GetClient(),
			mgr.GetScheme(),
			dynamicClient,
			statusUpdater,
			eventRecorder,
			controllerConfig,
		)
	}

	// Setup reconciler with manager
	if err := reconciler.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Error setting up reconciler: %v", err)
	}

	// Add health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Fatalf("Error adding health check: %v", err)
	}

	// Add readiness check (only leader is ready)
	if err := mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
		// In controller-runtime, only the leader is ready
		// This is handled automatically by the manager
		return nil
	}); err != nil {
		klog.Fatalf("Error adding readiness check: %v", err)
	}

	// Start webhook server if enabled (separate from controller-runtime webhook server)
	var webhookServer *gcwebhook.WebhookServer
	if *enableWebhook {
		var err error
		webhookServer, err = gcwebhook.NewWebhookServer(*webhookAddr, *webhookCertFile, *webhookKeyFile)
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

	// Start the manager (this blocks until context is canceled)
	klog.Info("Starting GC controller manager...")
	if err := mgr.Start(ctx); err != nil {
		klog.Fatalf("Error starting manager: %v", err)
	}

	klog.Info("GC controller shutdown complete")
}
