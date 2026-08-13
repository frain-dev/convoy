package postgres

import (
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	log "github.com/frain-dev/convoy/pkg/logger"
)

// captureLogger records what the notice handler logs. The embedded interface is
// left unimplemented on purpose: the handler is only allowed to reach for Info
// and Warn, and any other call panics here rather than passing quietly. It locks
// because pgx calls the handler on whichever connection produced the notice.
type captureLogger struct {
	log.Logger
	mu   sync.Mutex
	info [][]any
	warn [][]any
}

func (c *captureLogger) Info(args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info = append(c.info, args)
}

func (c *captureLogger) Warn(args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.warn = append(c.warn, args)
}

func (c *captureLogger) infos() [][]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.info
}

func (c *captureLogger) warns() [][]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.warn
}

func TestNoticeHandlerReportsProgress(t *testing.T) {
	logger := &captureLogger{}

	noticeHandler(logger, nil)(nil, &pgconn.Notice{
		Severity:            "NOTICE",
		SeverityUnlocalized: "NOTICE",
		Code:                "00000",
		Message:             "Successfully partitioned events table...",
		Detail:              "301 partitions created",
		Hint:                "run convoy utils partition to retry",
	})

	require.Empty(t, logger.warns())
	require.Len(t, logger.infos(), 1)
	require.Equal(t, []any{
		"postgres notice",
		"severity", "NOTICE",
		"code", "00000",
		"message", "Successfully partitioned events table...",
		"detail", "301 partitions created",
		"hint", "run convoy utils partition to retry",
	}, logger.infos()[0])
}

// Detail and hint are usually empty. Emitting the keys anyway would put two
// blank fields on every line of a conversion that logs one notice per partition.
func TestNoticeHandlerOmitsEmptyDetailAndHint(t *testing.T) {
	logger := &captureLogger{}

	noticeHandler(logger, nil)(nil, &pgconn.Notice{
		Severity:            "NOTICE",
		SeverityUnlocalized: "NOTICE",
		Code:                "00000",
		Message:             "phase 2 of 6",
	})

	require.Len(t, logger.infos(), 1)
	require.Equal(t, []any{
		"postgres notice",
		"severity", "NOTICE",
		"code", "00000",
		"message", "phase 2 of 6",
	}, logger.infos()[0])
}

// Severity is translated by the server's lc_messages, so routing on it would
// silently demote every warning to progress on a non-English instance.
func TestNoticeHandlerRoutesWarningsByUnlocalizedSeverity(t *testing.T) {
	logger := &captureLogger{}

	noticeHandler(logger, nil)(nil, &pgconn.Notice{
		Severity:            "WARNUNG",
		SeverityUnlocalized: "WARNING",
		Code:                "01000",
		Message:             "there is no transaction in progress",
	})

	require.Empty(t, logger.infos(), "a warning was logged as progress")
	require.Len(t, logger.warns(), 1)
	require.Contains(t, logger.warns()[0], "there is no transaction in progress")
}

// pgx calls the handler on a connection goroutine, so a panic here takes down
// more than the statement that produced the notice.
func TestNoticeHandlerToleratesNoNotice(t *testing.T) {
	logger := &captureLogger{}

	require.NotPanics(t, func() { noticeHandler(logger, nil)(nil, nil) })
	require.Empty(t, logger.infos())
	require.Empty(t, logger.warns())
}

// Without a logger there is nowhere to send notices, so pgx should be left with
// no handler at all rather than one that dereferences nil per notice.
func TestNoticeHandlerWithoutLoggerIsNotInstalled(t *testing.T) {
	require.Nil(t, noticeHandler(nil, nil))
}

// A registered observer reads the same notices the log does. Progress recording
// depends on this, because notices are the only signal a long DDL statement emits
// that escapes its transaction.
func TestNoticeSinkObservesNoticesWithoutTakingThemFromTheLog(t *testing.T) {
	logger := &captureLogger{}
	sink := &noticeSink{}
	handler := noticeHandler(logger, sink)

	var seen []string
	sink.set(func(n *pgconn.Notice) { seen = append(seen, n.Message) })

	handler(nil, &pgconn.Notice{SeverityUnlocalized: "NOTICE", Message: "phase 1 of 6"})

	require.Equal(t, []string{"phase 1 of 6"}, seen)
	require.Len(t, logger.infos(), 1, "registering an observer stopped the notice reaching the log")
}

// The observer is cleared when a run ends. Notices from unrelated statements must
// not keep landing on a finished run's progress row.
func TestNoticeSinkStopsObservingWhenCleared(t *testing.T) {
	sink := &noticeSink{}
	handler := noticeHandler(&captureLogger{}, sink)

	var count int
	sink.set(func(*pgconn.Notice) { count++ })
	handler(nil, &pgconn.Notice{SeverityUnlocalized: "NOTICE", Message: "during"})

	sink.set(nil)
	handler(nil, &pgconn.Notice{SeverityUnlocalized: "NOTICE", Message: "after"})

	require.Equal(t, 1, count)
}

// pgx calls the handler on whichever connection produced the notice, so the
// observer is read and written from different goroutines.
func TestNoticeSinkIsSafeUnderConcurrentUse(t *testing.T) {
	sink := &noticeSink{}
	handler := noticeHandler(&captureLogger{}, sink)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			sink.set(func(*pgconn.Notice) {})
		}()
		go func() {
			defer wg.Done()
			handler(nil, &pgconn.Notice{SeverityUnlocalized: "NOTICE", Message: "concurrent"})
		}()
	}
	wg.Wait()
}
