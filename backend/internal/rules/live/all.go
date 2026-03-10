package live

import "github.com/canonical/signal-studio/internal/rules"

// AllRules returns all live rules that require metrics data.
func AllRules() []rules.Rule {
	return []rules.Rule{
		&HighDropRate{},
		&LogVolumeDominance{},
		&QueueNearCapacity{},
		&ReceiverExporterMismatch{},
		&ExporterSustainedFailures{},
		&ReceiverBackpressure{},
		&ZeroThroughput{},
	}
}
