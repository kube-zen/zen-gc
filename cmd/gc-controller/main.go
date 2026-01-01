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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-gc/pkg/controller"
	gcwebhook "github.com/kube-zen/zen-gc/pkg/webhook"
	"github.com/kube-zen/zen-sdk/pkg/leader"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-sdk/pkg/observability"
	"github.com/kube-zen/zen-sdk/pkg/zenlead"
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
	logger    *sdklog.Logger
	setupLog  *sdklog.Logger
)

var (
	metricsAddr              = flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to")
	webhookAddr              = flag.String("webhook-addr", ":9443", "The address the webhook endpoint binds to")
	webhookCertFile          = flag.String("webhook-cert-file", "/etc/webhook/certs/tls.crt", "Path to TLS certificate file")
	webhookKeyFile           = flag.String("webhook-key-file", "/etc/webhook/certs/tls.key", "Path to TLS private key file")
	leaderElectionMode       = flag.String("leader-election-mode", "builtin", "Leader election mode: builtin (default), zenlead, or disabled")
	leaderElectionID         = flag.String("leader-election-id", "", "The ID for leader election (default: gc-controller-leader-election). Required for builtin mode.")
	leaderElectionLeaseName  = flag.String("leader-election-lease-name", "", "The LeaderGroup CRD name (required for zenlead mode)")
	enableWebhook            = flag.Bool("enable-webhook", true, "Enable validating webhook server")
	insecureWebhook          = flag.Bool("insecure-webhook", false, "Allow webhook to start without TLS (testing only, not recommended for production)")
	gcInterval               = flag.Duration("gc-interval", 1*time.Minute, "Interval between GC evaluation runs")
	maxDeletionsPerSecond    = flag.Int("max-deletions-per-second", 10, "Default maximum deletions per second (can be overridden per policy)")
	batchSize                = flag.Int("batch-size", DefaultBatchSize, "Default batch size for deletions (can be overridden per policy)")
	maxConcurrentEvaluations = flag.Int("max-concurrent-evaluations", DefaultMaxConcurrentEvaluations, "Maximum number of policies to evaluate concurrently")
)

//nolint:gocyclo // main function complexity is acceptable for initialization logic
func main() {
	flag.Parse()

	// Initialize zen-sdk logger (configures controller-runtime logger automatically)
	logger = sdklog.NewLogger("zen-gc")
	setupLog = logger.WithComponent("setup")
	setupLog.Debug("GC Controller starting", sdklog.String("version", version), sdklog.String("commit", commit), sdklog.String("buildDate", buildDate))

	// Set up signals so we handle shutdown gracefully
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize OpenTelemetry tracing using SDK
	if shutdown, err := observability.InitWithDefaults(ctx, "zen-gc"); err != nil {
		setupLog.Warn("OpenTelemetry tracer initialization failed, continuing without tracing",
			sdklog.String("error", err.Error()),
			sdklog.ErrorCode("OTEL_INIT_FAILED"),
		)
	} else {
		setupLog.Info("OpenTelemetry tracing initialized")
		defer func() {
			if err := shutdown(ctx); err != nil {
				setupLog.Warn("Failed to shutdown tracing", sdklog.String("error", err.Error()))
			}
		}()
	}

	// Get config using controller-runtime (handles kubeconfig flag automatically)
	restCfg := ctrl.GetConfigOrDie()

	// Apply REST config defaults (via zen-sdk helper)
	zenlead.ControllerRuntimeDefaults(restCfg)

	// Create dynamic client (still needed for resource informers)
	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		setupLog.Error(err, "Error building dynamic client", sdklog.ErrorCode("CLIENT_ERROR"))
		os.Exit(1)
	}

	// Create Kubernetes client for events
	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		setupLog.Error(err, "Error building Kubernetes client", sdklog.ErrorCode("CLIENT_ERROR"))
		os.Exit(1)
	}

	// Create scheme and add GarbageCollectionPolicy types
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Error adding scheme", sdklog.ErrorCode("SCHEME_ERROR"))
		os.Exit(1)
	}

	// Get namespace (required for leader election)
	namespace, err := leader.RequirePodNamespace()
	if err != nil {
		setupLog.Error(err, "Failed to determine pod namespace", sdklog.ErrorCode("NAMESPACE_ERROR"))
		os.Exit(1)
	}

	// Load controller configuration
	controllerConfig := config.NewControllerConfig()
	controllerConfig.LoadFromEnv() // Load from environment variables
	controllerConfig.WithGCInterval(*gcInterval)
	controllerConfig.WithMaxDeletionsPerSecond(*maxDeletionsPerSecond)
	controllerConfig.WithBatchSize(*batchSize)
	controllerConfig.WithMaxConcurrentEvaluations(*maxConcurrentEvaluations)

	setupLog.Info("Controller configuration",
		sdklog.String("gcInterval", controllerConfig.GCInterval.String()),
		sdklog.Int("maxDeletionsPerSecond", controllerConfig.MaxDeletionsPerSecond),
		sdklog.Int("batchSize", controllerConfig.BatchSize),
		sdklog.Int("maxConcurrentEvaluations", controllerConfig.MaxConcurrentEvaluations))

	// Create status updater with configuration
	statusUpdater := controller.NewStatusUpdaterWithConfig(dynamicClient, controllerConfig)

	// Create event recorder
	eventRecorder := controller.NewEventRecorder(kubeClient)

	// Setup controller-runtime manager
	baseOpts := ctrl.Options{
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

	// Configure leader election using zenlead package (Profiles B/C)
	var leConfig zenlead.LeaderElectionConfig

	// Determine election ID (default if not provided)
	electionID := *leaderElectionID
	if electionID == "" {
		electionID = "gc-controller-leader-election"
	}

	// Configure based on mode
	switch *leaderElectionMode {
	case "builtin":
		leConfig = zenlead.LeaderElectionConfig{
			Mode:       zenlead.BuiltIn,
			ElectionID: electionID,
			Namespace:  namespace,
		}
		setupLog.Info("Leader election mode: builtin (Profile B)", sdklog.Operation("leader_election_config"))
	case "zenlead":
		if *leaderElectionLeaseName == "" {
			setupLog.Error(fmt.Errorf("--leader-election-lease-name is required when --leader-election-mode=zenlead"), "invalid configuration", sdklog.ErrorCode("INVALID_CONFIG"))
			os.Exit(1)
		}
		leConfig = zenlead.LeaderElectionConfig{
			Mode:      zenlead.ZenLeadManaged,
			LeaseName: *leaderElectionLeaseName,
			Namespace: namespace,
		}
		setupLog.Info("Leader election mode: zenlead managed (Profile C)", sdklog.Operation("leader_election_config"), sdklog.String("leaseName", *leaderElectionLeaseName))
	case "disabled":
		leConfig = zenlead.LeaderElectionConfig{
			Mode: zenlead.Disabled,
		}
		setupLog.Warn("Leader election disabled - single replica only (unsafe if replicas > 1)", sdklog.Operation("leader_election_config"))
	default:
		setupLog.Error(fmt.Errorf("invalid --leader-election-mode: %q (must be builtin, zenlead, or disabled)", *leaderElectionMode), "invalid configuration", sdklog.ErrorCode("INVALID_CONFIG"))
		os.Exit(1)
	}

	// Prepare manager options with leader election
		mgrOpts, err := zenlead.PrepareManagerOptions(&baseOpts, &leConfig)
	if err != nil {
		setupLog.Error(err, "Failed to prepare manager options", sdklog.ErrorCode("MANAGER_OPTIONS_ERROR"))
		os.Exit(1)
	}

	// Get replica count from environment (set by Helm/Kubernetes)
	replicaCount := 1
	if rcStr := os.Getenv("REPLICA_COUNT"); rcStr != "" {
		if rc, err := strconv.Atoi(rcStr); err == nil {
			replicaCount = rc
		}
	}

	// Enforce safe HA configuration
	if err := zenlead.EnforceSafeHA(replicaCount, mgrOpts.LeaderElection); err != nil {
		setupLog.Error(err, "Unsafe HA configuration", sdklog.ErrorCode("UNSAFE_HA_CONFIG"))
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(restCfg, mgrOpts)
	if err != nil {
		setupLog.Error(err, "Error creating controller manager", sdklog.ErrorCode("MANAGER_CREATE_ERROR"))
		os.Exit(1)
	}

	// Create GC policy reconciler (leader election handled by controller-runtime Manager)
	reconciler := controller.NewGCPolicyReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		dynamicClient,
		statusUpdater,
		eventRecorder,
		controllerConfig,
	)

	// Setup reconciler with manager
	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Error setting up reconciler", sdklog.ErrorCode("RECONCILER_SETUP_ERROR"))
		os.Exit(1)
	}

	// Add health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Error adding health check", sdklog.ErrorCode("HEALTH_CHECK_ERROR"))
		os.Exit(1)
	}

	// Add readiness check (only leader is ready)
	if err := mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
		// In controller-runtime, only the leader is ready
		// This is handled automatically by the manager
		return nil
	}); err != nil {
		setupLog.Error(err, "Error adding readiness check", sdklog.ErrorCode("READY_CHECK_ERROR"))
		os.Exit(1)
	}

	// Start webhook server if enabled (separate from controller-runtime webhook server)
	var webhookServer *gcwebhook.WebhookServer
	if *enableWebhook {
		var err error
		webhookServer, err = gcwebhook.NewWebhookServer(*webhookAddr, *webhookCertFile, *webhookKeyFile)
		if err != nil {
			setupLog.Error(err, "Error creating webhook server", sdklog.ErrorCode("WEBHOOK_CREATE_ERROR"))
			os.Exit(1)
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
					setupLog.Error(err, "Error starting webhook server", sdklog.ErrorCode("WEBHOOK_START_ERROR"))
					os.Exit(1)
				}
			}()
			setupLog.Info("Webhook server starting with TLS", sdklog.String("address", *webhookAddr), sdklog.Component("webhook"))
		} else {
			// TLS files missing - check if insecure mode is allowed
			if !*insecureWebhook {
				setupLog.Error(fmt.Errorf("webhook TLS certificates not found (cert: %s, key: %s). TLS is required for production. Use --insecure-webhook flag only for testing", *webhookCertFile, *webhookKeyFile), "TLS certificates missing", sdklog.ErrorCode("TLS_CERT_MISSING"))
				os.Exit(1)
			}
			setupLog.Warn("Webhook starting without TLS (insecure mode) - NOT RECOMMENDED FOR PRODUCTION", sdklog.Component("webhook"))
			go func() {
				if err := webhookServer.Start(ctx); err != nil {
					setupLog.Error(err, "Error starting webhook server", sdklog.ErrorCode("WEBHOOK_START_ERROR"))
					os.Exit(1)
				}
			}()
		}
	}

	// Start the manager (this blocks until context is canceled)
	setupLog.Info("Starting GC controller manager", sdklog.Operation("start"))
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Error starting manager", sdklog.ErrorCode("MANAGER_START_ERROR"))
		os.Exit(1)
	}

	setupLog.Info("GC controller shutdown complete", sdklog.Operation("shutdown"))
}
