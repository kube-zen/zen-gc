package controller

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/kube-zen/zen-gc/pkg/logging"
)

// LeaderElection manages leader election for GC controller HA.
// Deprecated: This implementation has been replaced by controller-runtime Manager's built-in leader election.
// The Manager handles leader election automatically. This code is kept for reference only.
type LeaderElection struct {
	client    kubernetes.Interface
	namespace string
	name      string
	identity  string
	onStarted func(context.Context)
	onStopped func()
	isLeader  bool
	mu        sync.RWMutex
}

// NewLeaderElection creates a new leader election manager.
func NewLeaderElection(
	client kubernetes.Interface,
	namespace string,
	name string,
) (*LeaderElection, error) {
	// Get pod name from environment (set by Kubernetes)
	identity := os.Getenv("POD_NAME")
	if identity == "" {
		// Fallback to hostname
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		identity = hostname
	}

	// Add unique suffix to avoid conflicts
	identity = fmt.Sprintf("%s-%d", identity, time.Now().Unix())

	return &LeaderElection{
		client:    client,
		namespace: namespace,
		name:      name,
		identity:  identity,
		isLeader:  false,
	}, nil
}

// Run starts the leader election process.
func (le *LeaderElection) Run(ctx context.Context) error {
	// Create lease lock
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      le.name,
			Namespace: le.namespace,
		},
		Client: le.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: le.identity,
		},
	}

	// Leader election configuration
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				le.mu.Lock()
				le.isLeader = true
				le.mu.Unlock()

				// Record metrics
				recordLeaderElectionStatus(true)
				recordLeaderElectionTransition()

				logger := logging.NewLogger().
					WithField("identity", le.identity).
					WithField("namespace", le.namespace).
					WithField("name", le.name)
				logger.Info("Became leader")

				if le.onStarted != nil {
					le.onStarted(ctx)
				}
			},
			OnStoppedLeading: func() {
				le.mu.Lock()
				le.isLeader = false
				le.mu.Unlock()

				// Record metrics
				recordLeaderElectionStatus(false)
				recordLeaderElectionTransition()

				logger := logging.NewLogger().WithField("identity", le.identity)
				logger.Info("Lost leadership")

				if le.onStopped != nil {
					le.onStopped()
				}
			},
			OnNewLeader: func(identity string) {
				logger := logging.NewLogger().
					WithField("new_leader", identity).
					WithField("current_identity", le.identity)
				logger.Info("New leader elected")
			},
		},
	}

	// Run leader election (blocks until context is canceled).
	leaderelection.RunOrDie(ctx, lec)

	return nil
}

// IsLeader returns whether this instance is the leader.
func (le *LeaderElection) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

// SetCallbacks sets the callbacks for leader election.
func (le *LeaderElection) SetCallbacks(onStarted func(context.Context), onStopped func()) {
	le.onStarted = onStarted
	le.onStopped = onStopped
}

// Identity returns the leader election identity.
func (le *LeaderElection) Identity() string {
	return le.identity
}
