package lib

import (
	"go.uber.org/zap"
)

// LoggerAdapter adapts an inner zap logger to the interfaces:
// 1. github.com/etcd-io/etcd/raft.Logger
// 2. google.golang.org/grpc/grpclog.LoggerV2
type LoggerAdapter struct {
	logger *zap.SugaredLogger
}

func NewLoggerAdapter(logger *zap.SugaredLogger) LoggerAdapter {
	return LoggerAdapter{
		logger: logger.Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar(),
	}
}

// Debug logs to DEBUG log. Arguments are handled in the manner of fmt.Print.
func (la LoggerAdapter) Debug(args ...interface{}) {
	la.logger.Debug(args)
}

// Debugf logs to DEBUG log. Arguments are handled in the manner of fmt.Printf.
func (la LoggerAdapter) Debugf(format string, args ...interface{}) {
	la.logger.Debugf(format, args...)
}

// Info logs to INFO log. Arguments are handled in the manner of fmt.Print.
func (la LoggerAdapter) Info(args ...interface{}) {
	la.logger.Info(args)
}

// Infoln logs to INFO log. Arguments are handled in the manner of fmt.Println.
func (la LoggerAdapter) Infoln(args ...interface{}) {
	la.logger.Info(args)
}

// Infof logs to INFO log. Arguments are handled in the manner of fmt.Printf.
func (la LoggerAdapter) Infof(format string, args ...interface{}) {
	la.logger.Infof(format, args...)
}

// Warning logs to WARNING log. Arguments are handled in the manner of fmt.Print.
func (la LoggerAdapter) Warning(args ...interface{}) {
	la.logger.Warn(args...)
}

// Warningln logs to WARNING log. Arguments are handled in the manner of fmt.Println.
func (la LoggerAdapter) Warningln(args ...interface{}) {
	la.logger.Warn(args...)
}

// Warningf logs to WARNING log. Arguments are handled in the manner of fmt.Printf.
func (la LoggerAdapter) Warningf(format string, args ...interface{}) {
	la.logger.Warnf(format, args...)
}

// Error logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func (la LoggerAdapter) Error(args ...interface{}) {
	la.logger.Error(args...)
}

// Errorln logs to ERROR log. Arguments are handled in the manner of fmt.Println.
func (la LoggerAdapter) Errorln(args ...interface{}) {
	la.logger.Error(args...)
}

// Errorf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func (la LoggerAdapter) Errorf(format string, args ...interface{}) {
	la.logger.Errorf(format, args...)
}

// Fatal logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func (la LoggerAdapter) Fatal(args ...interface{}) {
	la.logger.Fatal(args...)
}

// Fatalln logs to ERROR log. Arguments are handled in the manner of fmt.Println.
func (la LoggerAdapter) Fatalln(args ...interface{}) {
	la.logger.Fatal(args...)
}

// Fatalf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func (la LoggerAdapter) Fatalf(format string, args ...interface{}) {
	la.logger.Fatalf(format, args...)
}

// V reports whether verbosity level l is at least the requested verbose level.
func (la LoggerAdapter) V(l int) bool {
	return true
}

// Panic logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func (la LoggerAdapter) Panic(v ...interface{}) {
	la.logger.Panic(v...)
}

// Panicf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func (la LoggerAdapter) Panicf(format string, v ...interface{}) {
	la.logger.Panicf(format, v...)
}
