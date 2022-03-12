package convoy

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
)

var DefaultLevels = []log.Level{
	log.ErrorLevel,
	log.PanicLevel,
	log.FatalLevel,
	log.WarnLevel,
}

type SentryHook struct {
	LogLevels []log.Level
}

func NewSentryHook(levels []log.Level) *SentryHook {
	return &SentryHook{LogLevels: levels}
}

func (s *SentryHook) Levels() []log.Level {
	return s.LogLevels
}

func (s *SentryHook) Fire(entry *log.Entry) error {
	msg, err := entry.String()
	if err != nil {
		return fmt.Errorf("failed to get entry string - %w", err)
	}
	sentry.CaptureMessage(msg)
	return nil
}
