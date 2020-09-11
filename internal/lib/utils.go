package lib

import (
	"go.uber.org/zap"
)

// NewDevelopmentLogger returns a sugared zap logger for development.
func NewDevelopmentLogger() *zap.SugaredLogger {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return l.Sugar()
}
