package catalog

import "github.com/canonical/signal-studio/internal/rules"

// AllRules returns all catalog rules that require tap catalog + filter analyses.
func AllRules() []rules.Rule {
	return []rules.Rule{
		&InternalMetricsNotFiltered{},
		&HighAttributeCount{},
		&PointCountOutlier{},
		&FilterKeepsEverything{},
		&FilterDropsEverything{},
		&NoFilterHighVolume{},
		&ManyHistograms{},
		&ShortScrapeInterval{},
		&LoopDeviceMetrics{},
	}
}
