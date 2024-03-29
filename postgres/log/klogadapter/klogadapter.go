// Package klogadapter contains an implementation of the pgx.Logger interface
// for the glog package.
package klogadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/tracelog"
	"k8s.io/klog/v2"
)

// Logger implements tracelog.Logger.
type Logger struct{}

// NewLogger returns a new Logger ready for use.
func NewLogger() *Logger {
	return &Logger{}
}

// Log implements tracelog.Logger.Log. LogLevelTrace and LogLevelDebug are
// written to the INFO level with a "(level)" prefix. LogLevelNone, as well as
// the zero value for LogLevel, are written to the ERROR level with a "(level)"
// prefix.
func (l *Logger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	var sb strings.Builder
	sb.WriteString(msg)
	sb.WriteString(" [")
	writeSeparator := false
	for k, v := range data {
		if writeSeparator {
			sb.WriteString(", ")
		}
		sb.WriteString(k + "=" + fmt.Sprint(v))
		writeSeparator = true
	}
	sb.WriteString("]")
	s := sb.String()
	switch level {
	case tracelog.LogLevelTrace, tracelog.LogLevelDebug:
		klog.InfoDepth(2, "("+level.String()+") "+s)
	case tracelog.LogLevelInfo:
		klog.InfoDepth(2, s)
	case tracelog.LogLevelWarn:
		klog.WarningDepth(2, s)
	case tracelog.LogLevelError:
		klog.ErrorDepth(2, s)
	default:
		klog.ErrorDepth(2, "("+level.String()+") "+s)
	}
}
