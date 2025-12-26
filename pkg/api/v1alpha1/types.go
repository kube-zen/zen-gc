package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Matched",type="integer",JSONPath=".status.resourcesMatched"
// +kubebuilder:printcolumn:name="Deleted",type="integer",JSONPath=".status.resourcesDeleted"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// GarbageCollectionPolicy is the Schema for the garbagecollectionpolicies API.
type GarbageCollectionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GarbageCollectionPolicySpec   `json:"spec,omitempty"`
	Status GarbageCollectionPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GarbageCollectionPolicyList contains a list of GarbageCollectionPolicy.
type GarbageCollectionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GarbageCollectionPolicy `json:"items"`
}

// GarbageCollectionPolicySpec defines the desired state of GarbageCollectionPolicy.
type GarbageCollectionPolicySpec struct {
	// TargetResource defines which resources the GC policy applies to
	TargetResource TargetResourceSpec `json:"targetResource"`

	// TTL configuration
	TTL TTLSpec `json:"ttl"`

	// Optional: Additional conditions that must be met before deletion
	Conditions *ConditionsSpec `json:"conditions,omitempty"`

	// GC behavior configuration
	Behavior BehaviorSpec `json:"behavior,omitempty"`

	// Paused indicates whether the policy evaluation is paused.
	// When true, the controller will skip evaluating this policy.
	// Defaults to false.
	Paused bool `json:"paused,omitempty"`
}

// TargetResourceSpec defines the target resource for GC.
type TargetResourceSpec struct {
	// API version of the target resource (e.g., "v1", "apps/v1", "batch/v1")
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`

	// Kind of the target resource (e.g., "ConfigMap", "Pod", "Job", "Secret")
	Kind string `json:"kind" yaml:"kind"`

	// Optional: Namespace scope (for namespaced resources)
	// Use "*" for all namespaces, or specify a specific namespace
	Namespace string `json:"namespace,omitempty"`

	// Optional: Label selector to filter resources
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Optional: Field selector (for resources that support it)
	FieldSelector *FieldSelectorSpec `json:"fieldSelector,omitempty"`
}

// FieldSelectorSpec defines field-based selection.
type FieldSelectorSpec struct {
	// Field selector expressions
	// Example: metadata.namespace=zen-system
	MatchFields map[string]string `json:"matchFields,omitempty"`
}

// TTLSpec defines time-to-live configuration.
type TTLSpec struct {
	// Option 1: Fixed TTL (seconds after creation)
	SecondsAfterCreation *int64 `json:"secondsAfterCreation,omitempty"`

	// Option 2: Dynamic TTL from resource fields
	// JSONPath to TTL field, e.g., "spec.ttlSecondsAfterCreation"
	FieldPath string `json:"fieldPath,omitempty"`

	// Option 3: Mapped TTL based on resource field values
	// Used with fieldPath to map field values to TTL seconds
	Mappings map[string]int64 `json:"mappings,omitempty"`

	// Default TTL for mappings (used when no mapping matches)
	Default *int64 `json:"default,omitempty"`

	// Option 4: Relative to another timestamp field
	// JSONPath to timestamp field, e.g., "status.lastProcessedAt"
	RelativeTo string `json:"relativeTo,omitempty"`

	// Seconds after the relativeTo timestamp
	SecondsAfter *int64 `json:"secondsAfter,omitempty"`
}

// ConditionsSpec defines additional conditions for deletion.
type ConditionsSpec struct {
	// Only delete resources in specific phases/states
	Phase []string `json:"phase,omitempty"`

	// Only delete if resource has specific labels
	HasLabels []LabelCondition `json:"hasLabels,omitempty"`

	// Only delete if resource has specific annotations
	HasAnnotations []AnnotationCondition `json:"hasAnnotations,omitempty"`

	// Complex condition logic (AND)
	And []FieldCondition `json:"and,omitempty"`
}

// LabelCondition defines a label-based condition.
type LabelCondition struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Operator string `json:"operator,omitempty"` // Exists, Equals, In, NotIn
}

// AnnotationCondition defines an annotation-based condition.
type AnnotationCondition struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// FieldCondition defines a field-based condition.
type FieldCondition struct {
	FieldPath string   `json:"fieldPath"`
	Operator  string   `json:"operator"` // Equals, NotEquals, In, NotIn, Exists
	Value     string   `json:"value,omitempty"`
	Values    []string `json:"values,omitempty"`
}

// BehaviorSpec defines GC execution behavior.
type BehaviorSpec struct {
	// Rate limiting: max deletions per second
	MaxDeletionsPerSecond int `json:"maxDeletionsPerSecond,omitempty"`

	// Batch size: delete resources in batches
	BatchSize int `json:"batchSize,omitempty"`

	// Dry run: don't actually delete, just log
	DryRun bool `json:"dryRun,omitempty"`

	// Finalizer: add finalizer before deletion (for graceful cleanup)
	Finalizer string `json:"finalizer,omitempty"`

	// Deletion propagation policy
	PropagationPolicy string `json:"propagationPolicy,omitempty"` // Foreground, Background, Orphan

	// Grace period in seconds before force deletion
	GracePeriodSeconds *int64 `json:"gracePeriodSeconds,omitempty"`
}

// GarbageCollectionPolicyStatus defines the observed state of GarbageCollectionPolicy.
type GarbageCollectionPolicyStatus struct {
	// Policy status phase
	Phase string `json:"phase,omitempty"` // Active, Paused, Error

	// Statistics
	ResourcesMatched int64 `json:"resourcesMatched,omitempty"`
	ResourcesDeleted int64 `json:"resourcesDeleted,omitempty"`
	ResourcesPending int64 `json:"resourcesPending,omitempty"`

	// Last GC run timestamp
	LastGCRun *metav1.Time `json:"lastGCRun,omitempty"`

	// Next GC run timestamp
	NextGCRun *metav1.Time `json:"nextGCRun,omitempty"`

	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
