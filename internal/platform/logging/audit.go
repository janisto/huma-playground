package logging

import (
	"context"

	"go.uber.org/zap"
)

// LogAuditEvent logs a structured audit event for security and compliance.
//
// Args:
//   - action: The action performed (e.g., "create", "update", "delete")
//   - userID: The user performing the action
//   - resourceType: The type of resource (e.g., "profile")
//   - resourceID: The ID of the resource
//   - result: The result of the action ("success" or "failure")
//   - details: Optional additional details
func LogAuditEvent(
	ctx context.Context,
	action, userID, resourceType, resourceID, result string,
	details map[string]any,
) {
	logger := LoggerFromContext(ctx)

	logger.Info("Audit event",
		zap.String("audit.action", action),
		zap.String("audit.user_id", userID),
		zap.String("audit.resource_type", resourceType),
		zap.String("audit.resource_id", resourceID),
		zap.String("audit.result", result),
		zap.Any("audit.details", details),
	)
}
