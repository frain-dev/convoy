// Command gencliref builds a minimal Cobra tree matching the production root (persistent flags
// plus server, agent, migrate, config subcommands) without PreRun hooks, then emits JSON with
// command paths, help text, and flags. Kept in sync manually with cmd/main.go flag registration.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	flagset "github.com/spf13/pflag"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cmd/agent"
	configCmd "github.com/frain-dev/convoy/cmd/config"
	"github.com/frain-dev/convoy/cmd/migrate"
	"github.com/frain-dev/convoy/cmd/server"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/cli"
)

type flagEntry struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Usage       string `json:"usage"`
	Default     string `json:"default"`
	DefValue    string `json:"def_value"`
	Hidden      bool   `json:"hidden"`
	FlagScope   string `json:"flag_scope"` // root_persistent | inherited | local
	Deprecated  string `json:"deprecated,omitempty"`
	Annotations string `json:"annotations,omitempty"`
}

type commandEntry struct {
	Path      []string    `json:"path"`
	Use       string      `json:"use"`
	Short     string      `json:"short"`
	Long      string      `json:"long,omitempty"`
	Aliases   []string    `json:"aliases,omitempty"`
	Flags     []flagEntry `json:"flags"`
	FlagOrder []string    `json:"flag_order"`
}

type outputDoc struct {
	Source   string         `json:"source"`
	Commands []commandEntry `json:"commands"`
}

func main() {
	outPath := flag.String("output", "", "write JSON to this path (default: stdout)")
	flag.Parse()

	root := buildRootCommand()
	var commands []commandEntry
	walkCommands(root, nil, &commands)

	sort.Slice(commands, func(i, j int) bool {
		return pathKey(commands[i].Path) < pathKey(commands[j].Path)
	})

	doc := outputDoc{
		Source:   "cobra tree: scripts/docs/gencliref (mirrors cmd/main.go root flags + server|agent|migrate|config)",
		Commands: commands,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
	data := bytes.TrimSpace(buf.Bytes())
	if *outPath != "" {
		if err := os.WriteFile(*outPath, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", *outPath, err)
			os.Exit(1)
		}
		return
	}
	os.Stdout.Write(append(data, '\n'))
}

func pathKey(p []string) string {
	return strings.Join(p, "\x00")
}

func buildRootCommand() *cobra.Command {
	app := &cli.App{Version: convoy.GetVersion()}

	root := &cobra.Command{
		Use:     "Convoy",
		Short:   "High Performance Webhooks Gateway",
		Version: app.Version,
	}

	// --- Mirror cmd/main.go persistent/root flags (lines ~44–198) ---
	var dbPort int
	var dbType string
	var dbHost string
	var dbScheme string
	var dbUsername string
	var dbPassword string
	var dbDatabase string
	var dbReadReplicasDSN []string
	var fflag []string
	var ipAllowList []string
	var ipBLockList []string
	var enableProfiling bool
	var redisPort int
	var redisHost string
	var redisType string
	var redisScheme string
	var redisUsername string
	var redisPassword string
	var redisDatabase string
	var redisSentinelMasterName string
	var redisSentinelUsername string
	var redisSentinelPassword string
	var redisAddresses string
	var tracerType string
	var sentryDSN string
	var sentrySampleRate float64
	var sentryDebug bool
	var otelSampleRate float64
	var otelCollectorURL string
	var otelAuthHeaderName string
	var otelAuthHeaderValue string
	var dataDogAgentUrl string
	var metricsBackend string
	var prometheusMetricsSampleTime uint64
	var prometheusMetricsQueryTimeout uint64
	var prometheusMetricsMaterializedViewRefreshInterval uint64
	var retentionPolicy string
	var retentionPolicyEnabled bool
	var maxRetrySeconds uint64
	var instanceIngestRate int
	var apiRateLimit int
	var licenseKey string
	var logLevel string
	var rootPath string
	var configFile string
	var enableBilling bool
	var billingURL string
	var billingAPIKey string

	fs := root.PersistentFlags()
	fs.StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")
	fs.StringVar(&licenseKey, "license-key", "", "Convoy license key")
	fs.StringVar(&logLevel, "log-level", "", "Log level")
	fs.StringVar(&rootPath, "root-path", "", "Root path for routing behind load balancers (e.g., /convoy)")
	fs.BoolVar(&enableBilling, "enable-billing", false, "Enable billing functionality")
	fs.StringVar(&billingURL, "billing-url", "", "Billing service URL (required when billing is enabled)")
	fs.StringVar(&billingAPIKey, "billing-api-key", "", "Billing service API key (required when billing is enabled)")
	fs.StringVar(&dbHost, "db-host", "", "Database Host")
	fs.StringVar(&dbType, "db-type", "", "Database provider")
	fs.StringVar(&dbScheme, "db-scheme", "", "Database Scheme")
	fs.StringVar(&dbUsername, "db-username", "", "Database Username")
	fs.StringVar(&dbPassword, "db-password", "", "Database Password")
	fs.StringVar(&dbDatabase, "db-database", "", "Database Database")
	fs.StringVar(&dbDatabase, "db-options", "", "Database Options")
	fs.IntVar(&dbPort, "db-port", 0, "Database Port")
	fs.BoolVar(&enableProfiling, "enable-profiling", false, "Enable profiling and exporting profile data to pyroscope")
	fs.StringSliceVar(&dbReadReplicasDSN, "read-replicas-dsn", []string{}, "Comma-separated list of read replica DSNs e.g. postgres://convoy:convoy@host1:5436/db,postgres://convoy:convoy@host2:5437/db")
	fs.StringVar(&redisHost, "redis-host", "", "Redis Host")
	fs.StringVar(&redisType, "redis-type", "", "Redis provider")
	fs.StringVar(&redisScheme, "redis-scheme", "", fmt.Sprintf("Redis Scheme (%s, %s, %s)", config.RedisScheme, config.RedisSentinelScheme, config.RedisSecureScheme))
	fs.StringVar(&redisUsername, "redis-username", "", "Redis Username")
	fs.StringVar(&redisPassword, "redis-password", "", "Redis Password")
	fs.StringVar(&redisDatabase, "redis-database", "", "Redis database")
	fs.IntVar(&redisPort, "redis-port", 0, "Redis Port")
	fs.StringVar(&redisSentinelMasterName, "redis-sentinel-master-name", "", "Redis Sentinel master name (required when scheme is redis-sentinel)")
	fs.StringVar(&redisSentinelUsername, "redis-sentinel-username", "", "Redis Sentinel username")
	fs.StringVar(&redisSentinelPassword, "redis-sentinel-password", "", "Redis Sentinel password")
	fs.StringVar(&redisAddresses, "redis-addresses", "", "Redis addresses (comma-separated, for cluster or sentinel)")
	fs.StringSliceVar(&fflag, "enable-feature-flag", []string{}, "List of feature flags to enable e.g. \"full-text-search,prometheus\"")
	fs.StringSliceVar(&ipAllowList, "ip-allow-list", []string{}, "List of IPs CIDRs to allow e.g. \" 0.0.0.0/0,127.0.0.0/8\"")
	fs.StringSliceVar(&ipBLockList, "ip-block-list", []string{}, "List of IPs CIDRs to block e.g. \" 0.0.0.0/0,127.0.0.0/8\"")
	fs.IntVar(&instanceIngestRate, "instance-ingest-rate", 0, "Instance ingest Rate")
	fs.IntVar(&apiRateLimit, "api-rate-limit", 0, "API rate limit")
	fs.StringVar(&tracerType, "tracer-type", "", "Tracer backend, e.g. sentry, datadog or otel")
	fs.StringVar(&sentryDSN, "sentry-dsn", "", "Sentry backend dsn")
	fs.Float64Var(&sentrySampleRate, "sentry-sample-rate", 1.0, "Sentry tracing sample rate")
	fs.BoolVar(&sentryDebug, "sentry-debug", false, "Enable Sentry debug logging")
	fs.Float64Var(&otelSampleRate, "otel-sample-rate", 1.0, "OTel tracing sample rate")
	fs.StringVar(&otelCollectorURL, "otel-collector-url", "", "OTel collector URL")
	fs.StringVar(&otelAuthHeaderName, "otel-auth-header-name", "", "OTel backend auth header name")
	fs.StringVar(&otelAuthHeaderValue, "otel-auth-header-value", "", "OTel backend auth header value")
	fs.StringVar(&dataDogAgentUrl, "datadog-agent-url", "", "Datadog agent URL")
	fs.StringVar(&metricsBackend, "metrics-backend", "prometheus", "Metrics backend e.g. prometheus. ('prometheus' feature flag required")
	fs.Uint64Var(&prometheusMetricsSampleTime, "metrics-prometheus-sample-time", 5, "Prometheus metrics sample time")
	fs.Uint64Var(&prometheusMetricsQueryTimeout, "metrics-prometheus-query-timeout", 30, "Prometheus metrics query timeout in seconds")
	fs.Uint64Var(&prometheusMetricsMaterializedViewRefreshInterval, "metrics-prometheus-materialized-view-refresh-interval", 2, "Materialized view refresh interval in minutes")
	fs.StringVar(&retentionPolicy, "retention-policy", "", "Retention Policy Duration")
	fs.BoolVar(&retentionPolicyEnabled, "retention-policy-enabled", false, "Retention Policy Enabled")
	fs.Uint64Var(&maxRetrySeconds, "max-retry-seconds", 7200, "Max retry seconds exponential backoff")

	fs.String("hcp-client-id", "", "HCP Vault client ID")
	fs.String("hcp-client-secret", "", "HCP Vault client secret")
	fs.String("hcp-org-id", "", "HCP Vault organization ID")
	fs.String("hcp-project-id", "", "HCP Vault project ID")
	fs.String("hcp-app-name", "", "HCP Vault app name")
	fs.String("hcp-secret-name", "", "HCP Vault secret name")
	fs.Duration("hcp-cache-duration", 5*time.Minute, "HCP Vault key cache duration")

	root.AddCommand(server.AddServerCommand(app))
	root.AddCommand(migrate.AddMigrateCommand(app))
	root.AddCommand(configCmd.AddConfigCommand())
	root.AddCommand(agent.AddAgentCommand(app))

	return root
}

func walkCommands(cmd *cobra.Command, path []string, out *[]commandEntry) {
	p := append(append([]string(nil), path...), cmd.Name())
	if cmd == cmd.Root() && len(path) == 0 {
		p = nil
	}

	pathCopy := append([]string(nil), p...)
	if pathCopy == nil {
		pathCopy = []string{}
	}
	*out = append(*out, commandEntry{
		Path:      pathCopy,
		Use:       strings.TrimSpace(cmd.Use),
		Short:     strings.TrimSpace(cmd.Short),
		Long:      strings.TrimSpace(cmd.Long),
		Aliases:   append([]string(nil), cmd.Aliases...),
		Flags:     collectFlags(cmd),
		FlagOrder: flagNamesInOrder(cmd),
	})

	for _, sub := range cmd.Commands() {
		if strings.EqualFold(sub.Name(), "help") || sub.Name() == "completion" {
			continue
		}
		walkCommands(sub, p, out)
	}
}

func flagNamesInOrder(cmd *cobra.Command) []string {
	root := cmd.Root()
	seen := map[string]struct{}{}
	var names []string
	add := func(fs *flagset.FlagSet) {
		fs.VisitAll(func(f *flagset.Flag) {
			if _, ok := seen[f.Name]; ok {
				return
			}
			seen[f.Name] = struct{}{}
			names = append(names, f.Name)
		})
	}
	if cmd == root {
		add(cmd.PersistentFlags())
	} else {
		add(root.PersistentFlags())
		add(cmd.LocalFlags())
	}
	return names
}

func collectFlags(cmd *cobra.Command) []flagEntry {
	root := cmd.Root()
	byName := map[string]flagEntry{}
	var order []string

	visit := func(fs *flagset.FlagSet) {
		fs.VisitAll(func(f *flagset.Flag) {
			if _, ok := byName[f.Name]; ok {
				return
			}
			onRootPersistent := root.PersistentFlags().Lookup(f.Name) != nil
			var scope string
			switch {
			case cmd == root:
				scope = "root_persistent"
			case onRootPersistent:
				scope = "inherited"
			default:
				scope = "local"
			}
			e := flagEntry{
				Name:       f.Name,
				Shorthand:  f.Shorthand,
				Usage:      f.Usage,
				Default:    f.DefValue,
				DefValue:   f.DefValue,
				Hidden:     f.Hidden,
				FlagScope:  scope,
				Deprecated: f.Deprecated,
			}
			if f.Annotations != nil {
				e.Annotations = fmt.Sprintf("%v", f.Annotations)
			}
			byName[f.Name] = e
			order = append(order, f.Name)
		})
	}

	if cmd == root {
		visit(cmd.PersistentFlags())
	} else {
		visit(root.PersistentFlags())
		visit(cmd.LocalFlags())
	}

	list := make([]flagEntry, 0, len(byName))
	for _, n := range order {
		list = append(list, byName[n])
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
