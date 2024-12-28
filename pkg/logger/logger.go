package logger

import (
	"github.com/sirupsen/logrus"
)

// Log is the global logger instance
var Log = logrus.New()

func init() {
	// Set default format
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// SetLevel sets the logging level
func SetLevel(level string) {
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warning":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
}
