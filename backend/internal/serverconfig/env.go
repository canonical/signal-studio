// Package serverconfig defines environment variables for the server.
package serverconfig

// EnvVar describes a single environment variable.
type EnvVar struct {
	// Name is the environment variable name.
	Name string
	// Default is the default value when unset.
	Default string
	// Description is a short human-readable summary.
	Description string
}

// EnvVars lists all environment variables read by the server.
var EnvVars = []EnvVar{
	{Name: "SIGNAL_STUDIO_PORT", Default: "8080", Description: "HTTP server port"},
	{Name: "SIGNAL_STUDIO_SCRAPE_INTERVAL_SECONDS", Default: "10", Description: "Metrics polling interval (5\u201330)"},
	{Name: "SIGNAL_STUDIO_MAX_YAML_SIZE_KB", Default: "256", Description: "Maximum YAML body size"},
	{Name: "SIGNAL_STUDIO_CORS_ORIGINS", Default: "*", Description: "Allowed CORS origins (comma-separated)"},
	{Name: "SIGNAL_STUDIO_TAP_DISABLED", Default: "false", Description: "Disable the OTLP sampling tap"},
	{Name: "SIGNAL_STUDIO_TAP_GRPC_ADDR", Default: ":5317", Description: "gRPC listen address for the OTLP tap"},
	{Name: "SIGNAL_STUDIO_TAP_HTTP_ADDR", Default: ":5318", Description: "HTTP listen address for the OTLP tap"},
}
