package logger

import (
	"testing"

	"github.com/frain-dev/convoy/config"
)

func setup(level string, t *testing.T) *ConsoleLogger {
	cfg := config.LoggerConfiguration{Type: "console"}

	cfg.ServerLog.Level = level

	lo, err := NewConsoleLogger(cfg)
	if err != nil {
		t.Errorf("Failed to initialize console logger: %v", err)
	}

	return lo

}

func testConsoleCalls(log Logger) {
	log.Info("info")
	log.Debug("debug")
	log.Warn("warn")
	log.Trace("trace")
	log.Error("error")
}

//test console logging by visually comparing outputs depending on the
//chosen log level
func TestConsole(t *testing.T) {
	infoLog := setup("info", t)
	testConsoleCalls(infoLog)

	warnLog := setup("warn", t)
	testConsoleCalls(warnLog)
}
