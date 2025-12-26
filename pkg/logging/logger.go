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

// Package logging provides structured logging with correlation IDs and consistent field formatting.
package logging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
)

// Logger provides structured logging with consistent fields and correlation IDs.
type Logger struct {
	fields map[string]interface{}
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// NewLogger creates a new logger instance.
func NewLogger() *Logger {
	return &Logger{
		fields: make(map[string]interface{}),
	}
}

// WithFields creates a new logger with additional fields.
func (l *Logger) WithFields(fields ...Field) *Logger {
	newLogger := &Logger{
		fields: make(map[string]interface{}),
	}
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Add new fields
	for _, f := range fields {
		newLogger.fields[f.Key] = f.Value
	}
	return newLogger
}

// WithField creates a new logger with a single additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(Field{Key: key, Value: value})
}

// WithPolicy adds policy-related fields to the logger.
func (l *Logger) WithPolicy(namespace, name string) *Logger {
	return l.WithFields(
		Field{Key: "policy_namespace", Value: namespace},
		Field{Key: "policy_name", Value: name},
	)
}

// WithResource adds resource-related fields to the logger.
func (l *Logger) WithResource(namespace, name, apiVersion, kind string) *Logger {
	return l.WithFields(
		Field{Key: "resource_namespace", Value: namespace},
		Field{Key: "resource_name", Value: name},
		Field{Key: "resource_api_version", Value: apiVersion},
		Field{Key: "resource_kind", Value: kind},
	)
}

// WithCorrelationID adds a correlation ID to the logger for tracing.
func (l *Logger) WithCorrelationID(correlationID string) *Logger {
	return l.WithField("correlation_id", correlationID)
}

// WithError adds error information to the logger.
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithFields(
		Field{Key: "error", Value: err.Error()},
		Field{Key: "error_type", Value: getErrorType(err)},
	)
}

// WithDuration adds duration information to the logger.
func (l *Logger) WithDuration(duration time.Duration) *Logger {
	return l.WithField("duration_ms", duration.Milliseconds())
}

// Info logs at info level with structured fields.
func (l *Logger) Info(msg string) {
	klog.InfoS(msg, l.fieldsToKlogArgs()...)
}

// Infof logs at info level with formatted message and structured fields.
func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.InfoS(msg, l.fieldsToKlogArgs()...)
}

// Error logs at error level with structured fields.
func (l *Logger) Error(msg string) {
	klog.ErrorS(nil, msg, l.fieldsToKlogArgs()...)
}

// Errorf logs at error level with formatted message and structured fields.
func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.ErrorS(nil, msg, l.fieldsToKlogArgs()...)
}

// Warning logs at warning level with structured fields.
func (l *Logger) Warning(msg string) {
	klog.InfoS(msg, append([]interface{}{"level", "warning"}, l.fieldsToKlogArgs()...)...)
}

// Warningf logs at warning level with formatted message and structured fields.
func (l *Logger) Warningf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.InfoS(msg, append([]interface{}{"level", "warning"}, l.fieldsToKlogArgs()...)...)
}

// V returns a logger with the specified verbosity level.
func (l *Logger) V(level int) VerboseLogger {
	return VerboseLogger{
		logger: l,
		level:  level,
	}
}

// fieldsToKlogArgs converts fields map to klog key-value pairs.
func (l *Logger) fieldsToKlogArgs() []interface{} {
	args := make([]interface{}, 0, len(l.fields)*2)
	for k, v := range l.fields {
		args = append(args, k, v)
	}
	return args
}

// VerboseLogger provides verbosity-aware logging.
type VerboseLogger struct {
	logger *Logger
	level  int
}

// safeKlogLevel converts int to klog.Level safely, clamping to valid range.
// Verbosity levels are small integers (0-10), so overflow is not a concern.
func safeKlogLevel(level int) klog.Level {
	// Clamp to valid range for klog verbosity (typically 0-10)
	if level < 0 {
		level = 0
	} else if level > 10 {
		level = 10
	}
	//nolint:gosec // G115: Verbosity levels are bounded (0-10), safe to convert int to int32
	return klog.Level(level)
}

// Info logs at the specified verbosity level.
func (vl VerboseLogger) Info(msg string) {
	klog.V(safeKlogLevel(vl.level)).InfoS(msg, vl.logger.fieldsToKlogArgs()...)
}

// Infof logs at the specified verbosity level with formatted message.
func (vl VerboseLogger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.V(safeKlogLevel(vl.level)).InfoS(msg, vl.logger.fieldsToKlogArgs()...)
}

// getErrorType extracts error type information.
func getErrorType(err error) string {
	if err == nil {
		return ""
	}
	// Try to get error type from GCError (handles both direct and wrapped errors)
	var gcerr *gcerrors.GCError
	if errors.As(err, &gcerr) && gcerr != nil {
		if gcerr.Type != "" {
			return gcerr.Type
		}
	}
	return "unknown"
}

// FromContext creates a logger from context, extracting correlation ID if present.
func FromContext(ctx context.Context) *Logger {
	logger := NewLogger()
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		logger = logger.WithCorrelationID(correlationID)
	}
	return logger
}

// ContextKey is the type for context keys.
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation ID.
	CorrelationIDKey ContextKey = "correlation_id"
)

// WithCorrelationID adds a correlation ID to the context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetCorrelationID extracts correlation ID from context.
func GetCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// GenerateCorrelationID generates a new correlation ID.
func GenerateCorrelationID() string {
	return fmt.Sprintf("gc-%d", time.Now().UnixNano())
}
