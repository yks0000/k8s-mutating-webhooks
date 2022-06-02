package main

import (
	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
)

func init() {
	logger = logrus.New()
	logger.SetFormatter(
		&logrus.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		})
	logger.SetLevel(logrus.TraceLevel)
}