// Package klogger is a grpclog implementation for k8s.io/klog/v2. It is very
// similar to google.golang.org/grpc/grpclog/glogger but uses klog instead.
package klogger

import (
	"fmt"

	"google.golang.org/grpc/grpclog"
	"k8s.io/klog/v2"
)

const d = 2

func init() {
	grpclog.SetLoggerV2(&glogger{})
}

type glogger struct{}

func (g *glogger) Info(args ...interface{}) {
	klog.InfoDepth(d, args...)
}

func (g *glogger) Infoln(args ...interface{}) {
	klog.InfoDepth(d, fmt.Sprintln(args...))
}

func (g *glogger) Infof(format string, args ...interface{}) {
	klog.InfoDepth(d, fmt.Sprintf(format, args...))
}

func (g *glogger) InfoDepth(depth int, args ...interface{}) {
	klog.InfoDepth(depth+d, args...)
}

func (g *glogger) Warning(args ...interface{}) {
	klog.WarningDepth(d, args...)
}

func (g *glogger) Warningln(args ...interface{}) {
	klog.WarningDepth(d, fmt.Sprintln(args...))
}

func (g *glogger) Warningf(format string, args ...interface{}) {
	klog.WarningDepth(d, fmt.Sprintf(format, args...))
}

func (g *glogger) WarningDepth(depth int, args ...interface{}) {
	klog.WarningDepth(depth+d, args...)
}

func (g *glogger) Error(args ...interface{}) {
	klog.ErrorDepth(d, args...)
}

func (g *glogger) Errorln(args ...interface{}) {
	klog.ErrorDepth(d, fmt.Sprintln(args...))
}

func (g *glogger) Errorf(format string, args ...interface{}) {
	klog.ErrorDepth(d, fmt.Sprintf(format, args...))
}

func (g *glogger) ErrorDepth(depth int, args ...interface{}) {
	klog.ErrorDepth(depth+d, args...)
}

func (g *glogger) Fatal(args ...interface{}) {
	klog.FatalDepth(d, args...)
}

func (g *glogger) Fatalln(args ...interface{}) {
	klog.FatalDepth(d, fmt.Sprintln(args...))
}

func (g *glogger) Fatalf(format string, args ...interface{}) {
	klog.FatalDepth(d, fmt.Sprintf(format, args...))
}

func (g *glogger) FatalDepth(depth int, args ...interface{}) {
	klog.FatalDepth(depth+d, args...)
}

func (g *glogger) V(l int) bool {
	return klog.V(klog.Level(l)).Enabled()
}
