package logging

import (
	"fmt"
	"os"
	"regexp"
	"sync"

	"go.uber.org/zap"
)

const traceparentHeader = "traceparent"

// W3C Trace Context format: {version}-{trace-id}-{parent-id}-{trace-flags}
// Example: 00-ab42124a3c573678d4d8b21ba52df3bf-d21f7bc17caa5aba-01
var traceHeaderRe = regexp.MustCompile(`^([0-9a-fA-F]{2})-([0-9a-fA-F]{32})-([0-9a-fA-F]{16})-([0-9a-fA-F]{2})$`)

var (
	projectIDOnce   sync.Once
	cachedProjectID string
)

func loggerWithTrace(base *zap.Logger, header, projectID, requestID string) *zap.Logger {
	if base == nil {
		base = zap.NewNop()
	}
	fields := traceFields(header, projectID)
	if requestID != "" {
		fields = append(fields, zap.String("requestId", requestID))
	}
	if len(fields) == 0 {
		return base
	}
	return base.With(fields...)
}

func traceFields(header, projectID string) []zap.Field {
	if projectID == "" {
		return nil
	}
	matches := traceHeaderRe.FindStringSubmatch(header)
	if len(matches) != 5 {
		return nil
	}
	traceID := matches[2]
	spanID := matches[3]
	flags := matches[4]
	sampled := flags == "01"
	resource := fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)

	return []zap.Field{
		zap.String("logging.googleapis.com/trace", resource),
		zap.String("logging.googleapis.com/spanId", spanID),
		zap.Bool("logging.googleapis.com/trace_sampled", sampled),
	}
}

func traceResource(header, projectID string) string {
	if projectID == "" {
		return ""
	}
	matches := traceHeaderRe.FindStringSubmatch(header)
	if len(matches) != 5 {
		return ""
	}
	return fmt.Sprintf("projects/%s/traces/%s", projectID, matches[2])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveProjectID() string {
	projectIDOnce.Do(func() {
		cachedProjectID = firstNonEmpty(
			os.Getenv("FIREBASE_PROJECT_ID"),
			os.Getenv("GOOGLE_CLOUD_PROJECT"),
			os.Getenv("GCP_PROJECT"),
			os.Getenv("GCLOUD_PROJECT"),
			os.Getenv("PROJECT_ID"),
		)
	})
	return cachedProjectID
}
