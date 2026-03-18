package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

func InitLogger() {
	Log.SetOutput(os.Stdout)
	Log.SetLevel(logrus.DebugLevel)
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
