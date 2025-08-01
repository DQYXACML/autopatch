package core

import (
	tracingUtils "github.com/DQYXACML/autopatch/tracing/utils"
)

// NewJumpTracer creates a new jump tracer using the utils package
func NewJumpTracer() *tracingUtils.JumpTracer {
	return tracingUtils.NewJumpTracer()
}
