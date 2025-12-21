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

package main

import (
	"context"
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
	"github.com/kube-zen/zen-gc/pkg/controller"
)

var (
	// Version information (set via build flags)
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

var (
	kubeconfig           = flag.String("kubeconfig", "", "Path to kubeconfig file. If not set, uses in-cluster config")
	masterURL            = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig")
	metricsAddr          = flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to")
	enableLeaderElection = flag.Bool("enable-leader-election", true, "Enable leader election for HA")
	leaderElectionNS     = flag.String("leader-election-namespace", "", "Namespace for leader election lease (defaults to POD_NAMESPACE)")
)

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

	// Create status updater
	statusUpdater := controller.NewStatusUpdater(dynamicClient)

	// Create event recorder
	eventRecorder := controller.NewEventRecorder(kubeClient)

	// Create GC controller
	gcController, err := controller.NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		klog.Fatalf("Error creating GC controller: %v", err)
	}

	// Start metrics server
	go startMetricsServer(*metricsAddr)

	// Setup leader election if enabled
	if *enableLeaderElection {
		leaderElection, err := controller.NewLeaderElection(kubeClient, namespace, "gc-controller-leader-election")
		if err != nil {
			klog.Fatalf("Error creating leader election: %v", err)
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

		// Run leader election (blocks until context is cancelled)
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
	klog.Info("Shutting down GC controller...")

	// Stop controller
	gcController.Stop()

	klog.Info("GC controller stopped")
}

// buildConfig builds a Kubernetes config from the given master URL and kubeconfig path
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

// startMetricsServer starts the Prometheus metrics server
func startMetricsServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	klog.Infof("Starting metrics server on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("Error starting metrics server: %v", err)
	}
}
