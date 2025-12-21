package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/controller"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file. If not set, uses in-cluster config")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig")
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

	// Add GarbageCollectionPolicy to scheme
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		klog.Fatalf("Error adding scheme: %v", err)
	}

	// Create GC controller
	gcController, err := controller.NewGCController(dynamicClient)
	if err != nil {
		klog.Fatalf("Error creating GC controller: %v", err)
	}

	// Start controller
	if err := gcController.Start(); err != nil {
		klog.Fatalf("Error starting GC controller: %v", err)
	}

	klog.Info("GC controller is running. Press Ctrl+C to stop.")

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
