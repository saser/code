// Package glogadapter contains an implementation of the pgx.Logger interface
// for the glog package.
package glogadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/jackc/pgx/v4"
)

// Logger implements pgx.Logger.
type Logger struct{}

// NewLogger returns a new Logger ready for use.
func NewLogger() *Logger {
	return &Logger{}
}

// Log implements pgx.Logger.Log. LogLevelTrace and LogLevelDebug are written to
// the INFO level with a "(level)" prefix. LogLevelNone, as well as the zero
// value for LogLevel, are written to the ERROR level with a "(level)" prefix.
func (l *Logger) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
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
	case pgx.LogLevelTrace, pgx.LogLevelDebug:
		glog.InfoDepth(2, "("+level.String()+") "+s)
	case pgx.LogLevelInfo:
		glog.InfoDepth(2, s)
	case pgx.LogLevelWarn:
		glog.WarningDepth(2, s)
	case pgx.LogLevelError:
		glog.ErrorDepth(2, s)
	default:
		glog.ErrorDepth(2, "("+level.String()+") "+s)
	}
}
