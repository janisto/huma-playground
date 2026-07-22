package audit

import (
	"context"

	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"
)

// LogEvent logs a structured audit event for security and compliance.
func LogEvent(
	ctx context.Context,
	action, userID, resourceType, resourceID, result string,
	details map[string]any,
) {
	obs.Logger(ctx).Info("Audit event",
		zap.String("audit.action", action),
		zap.String("audit.user_id", userID),
		zap.String("audit.resource_type", resourceType),
		zap.String("audit.resource_id", resourceID),
		zap.String("audit.result", result),
		zap.Any("audit.details", details),
	)
}
