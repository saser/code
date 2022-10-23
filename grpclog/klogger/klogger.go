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
	grpclog.SetLoggerV2(&klogger{})
}

type klogger struct{}

func (g *klogger) Info(args ...interface{}) {
	klog.InfoDepth(d, args...)
}

func (g *klogger) Infoln(args ...interface{}) {
	klog.InfoDepth(d, fmt.Sprintln(args...))
}

func (g *klogger) Infof(format string, args ...interface{}) {
	klog.InfoDepth(d, fmt.Sprintf(format, args...))
}

func (g *klogger) InfoDepth(depth int, args ...interface{}) {
	klog.InfoDepth(depth+d, args...)
}

func (g *klogger) Warning(args ...interface{}) {
	klog.WarningDepth(d, args...)
}

func (g *klogger) Warningln(args ...interface{}) {
	klog.WarningDepth(d, fmt.Sprintln(args...))
}

func (g *klogger) Warningf(format string, args ...interface{}) {
	klog.WarningDepth(d, fmt.Sprintf(format, args...))
}

func (g *klogger) WarningDepth(depth int, args ...interface{}) {
	klog.WarningDepth(depth+d, args...)
}

func (g *klogger) Error(args ...interface{}) {
	klog.ErrorDepth(d, args...)
}

func (g *klogger) Errorln(args ...interface{}) {
	klog.ErrorDepth(d, fmt.Sprintln(args...))
}

func (g *klogger) Errorf(format string, args ...interface{}) {
	klog.ErrorDepth(d, fmt.Sprintf(format, args...))
}

func (g *klogger) ErrorDepth(depth int, args ...interface{}) {
	klog.ErrorDepth(depth+d, args...)
}

func (g *klogger) Fatal(args ...interface{}) {
	klog.FatalDepth(d, args...)
}

func (g *klogger) Fatalln(args ...interface{}) {
	klog.FatalDepth(d, fmt.Sprintln(args...))
}

func (g *klogger) Fatalf(format string, args ...interface{}) {
	klog.FatalDepth(d, fmt.Sprintf(format, args...))
}

func (g *klogger) FatalDepth(depth int, args ...interface{}) {
	klog.FatalDepth(depth+d, args...)
}

func (g *klogger) V(l int) bool {
	return klog.V(klog.Level(l)).Enabled()
}
