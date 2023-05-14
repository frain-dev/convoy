package cli

import (
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/tracer"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// App is the core dependency of the entire binary.
type App struct {
	Version string
	DB      database.Database
	Queue   queue.Queuer
	Logger  log.StdLogger
	Tracer  tracer.Tracer
	Cache   cache.Cache
}

type ConvoyCli struct {
	cmd *cobra.Command
}

func NewCli(app *App, db *postgres.Postgres) *ConvoyCli {
	cmd := &cobra.Command{
		Use:     "Convoy",
		Version: app.Version,
		Short:   "High Performance Webhooks Gateway",
	}

	return &ConvoyCli{cmd: cmd}
}

func (c *ConvoyCli) Flags() *flag.FlagSet {
	return c.cmd.PersistentFlags()
}

func (c *ConvoyCli) PersistentPreRunE(fn func(*cobra.Command, []string) error) {
	c.cmd.PersistentPreRunE = fn
}

func (c *ConvoyCli) PersistentPostRunE(fn func(*cobra.Command, []string) error) {
	c.cmd.PersistentPostRunE = fn
}

func (c *ConvoyCli) AddCommand(subCmd *cobra.Command) {
	c.cmd.AddCommand(subCmd)
}

func (c *ConvoyCli) Execute() error {
	return c.cmd.Execute()
}
