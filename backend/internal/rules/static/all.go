package static

import "github.com/canonical/signal-studio/internal/rules"

// AllRules returns all static configuration rules.
func AllRules() []rules.Rule {
	return []rules.Rule{
		// Base rules
		&MissingMemoryLimiter{},
		&MissingBatch{},
		&NoTraceSampling{},
		&UnusedComponents{},
		&MultipleExportersNoRouting{},
		&NoLogSeverityFilter{},
		// Extended rules
		&MemoryLimiterNotFirst{},
		&BatchBeforeSampling{},
		&BatchNotNearEnd{},
		&ReceiverEndpointWildcard{},
		&DebugExporterInPipeline{},
		&PprofExtensionEnabled{},
		&MemoryLimiterWithoutLimits{},
		&ExporterNoSendingQueue{},
		&ExporterNoRetry{},
		&UndefinedComponentRef{},
		&EmptyPipeline{},
		&FilterErrorModePropagateRule{},
		&ScrapeIntervalMismatch{},
		&ExporterInsecureTLS{},
		&NoHealthCheckExtension{},
		&ExporterEndpointLocalhost{},
		&ExporterNoCompression{},
		&TailSamplingWithoutMemoryLimiter{},
		&ConnectorLoop{},
		&NoHealthCheckTraceFilter{},
		&FilesystemScraperNoExclusions{},
	}
}
