package api

// Route describes a single API endpoint.
type Route struct {
	// Method is the HTTP method (GET, POST, etc.).
	Method string
	// Path is the URL path pattern.
	Path string
	// Description is a short human-readable summary.
	Description string
	// Section groups the route for documentation.
	Section string
}

// Routes lists all API endpoints registered by the router.
var Routes = []Route{
	{Method: "POST", Path: "/api/config/analyze", Description: "Parse YAML and return config + findings", Section: "Config Analysis"},
	{Method: "GET", Path: "/api/health", Description: "Health check", Section: "Health"},

	{Method: "POST", Path: "/api/metrics/connect", Description: "Start scraping a Prometheus endpoint", Section: "Live Metrics"},
	{Method: "POST", Path: "/api/metrics/disconnect", Description: "Stop scraping", Section: "Live Metrics"},
	{Method: "GET", Path: "/api/metrics/snapshot", Description: "Latest computed rates and queue data", Section: "Live Metrics"},
	{Method: "GET", Path: "/api/metrics/status", Description: "Connection status", Section: "Live Metrics"},
	{Method: "POST", Path: "/api/metrics/reset", Description: "Reset metrics store", Section: "Live Metrics"},

	{Method: "POST", Path: "/api/alert-coverage", Description: "Analyze alerting rules against config", Section: "Alert Coverage"},

	{Method: "POST", Path: "/api/tap/start", Description: "Start OTLP sampling tap", Section: "OTLP Sampling Tap"},
	{Method: "POST", Path: "/api/tap/stop", Description: "Stop tap", Section: "OTLP Sampling Tap"},
	{Method: "GET", Path: "/api/tap/status", Description: "Tap status + window timing", Section: "OTLP Sampling Tap"},
	{Method: "GET", Path: "/api/tap/catalog", Description: "Discovered metrics + attribute metadata", Section: "OTLP Sampling Tap"},
	{Method: "POST", Path: "/api/tap/reset", Description: "Reset tap catalog", Section: "OTLP Sampling Tap"},
	{Method: "POST", Path: "/api/tap/remotetap/connect", Description: "Connect to a remotetapprocessor WebSocket stream", Section: "OTLP Sampling Tap"},
	{Method: "POST", Path: "/api/tap/remotetap/disconnect", Description: "Disconnect from remotetap stream", Section: "OTLP Sampling Tap"},
}
