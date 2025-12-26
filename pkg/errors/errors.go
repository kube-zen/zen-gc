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

// Package errors provides structured error types for the GC controller with policy and resource context.
package errors

import (
	"errors"
	"fmt"
)

// GCError represents a garbage collection error with context.
type GCError struct {
	// Type categorizes the error (e.g., "informer_creation_failed", "deletion_failed")
	Type string

	// PolicyNamespace is the namespace of the policy (if applicable)
	PolicyNamespace string

	// PolicyName is the name of the policy (if applicable)
	PolicyName string

	// ResourceNamespace is the namespace of the resource (if applicable)
	ResourceNamespace string

	// ResourceName is the name of the resource (if applicable)
	ResourceName string

	// Message is the error message
	Message string

	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *GCError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *GCError) Unwrap() error {
	return e.Err
}

// WithPolicy adds policy context to an error.
func WithPolicy(err error, namespace, name string) *GCError {
	var gcErr *GCError
	if errors.As(err, &gcErr) && gcErr != nil {
		gcErr.PolicyNamespace = namespace
		gcErr.PolicyName = name
		return gcErr
	}
	return &GCError{
		Message:         err.Error(),
		Err:             err,
		PolicyNamespace: namespace,
		PolicyName:      name,
	}
}

// WithResource adds resource context to an error.
func WithResource(err error, namespace, name string) *GCError {
	var gcErr *GCError
	if errors.As(err, &gcErr) && gcErr != nil {
		gcErr.ResourceNamespace = namespace
		gcErr.ResourceName = name
		return gcErr
	}
	return &GCError{
		Message:           err.Error(),
		Err:               err,
		ResourceNamespace: namespace,
		ResourceName:      name,
	}
}

// New creates a new GCError.
func New(errType, message string) *GCError {
	return &GCError{
		Type:    errType,
		Message: message,
	}
}

// Wrap wraps an error with a message and type.
func Wrap(err error, errType, message string) *GCError {
	return &GCError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

// Wrapf wraps an error with a formatted message and type.
func Wrapf(err error, errType, format string, args ...interface{}) *GCError {
	return &GCError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}
