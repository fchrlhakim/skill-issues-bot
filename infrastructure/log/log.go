package log

import (
	"strings"

	"github.com/sirupsen/logrus"
)

func New(level, format string) *logrus.Logger {
	logger := logrus.New()
	parsed, err := logrus.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsed = logrus.InfoLevel
	}
	logger.SetLevel(parsed)

	if strings.EqualFold(format, "json") {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
	return logger
}
