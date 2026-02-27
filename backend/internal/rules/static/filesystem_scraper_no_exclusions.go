package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// FilesystemScraperNoExclusions fires when a hostmetrics receiver has a
// filesystem scraper without exclude_mount_points or exclude_fs_types, which
// typically results in noisy metrics from virtual filesystems like /proc, /sys,
// and snap mounts.
type FilesystemScraperNoExclusions struct{}

func (r *FilesystemScraperNoExclusions) ID() string { return "filesystem-scraper-no-exclusions" }

func (r *FilesystemScraperNoExclusions) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, recv := range cfg.Receivers {
		if config.ComponentType(name) != "hostmetrics" {
			continue
		}
		if recv.Config == nil {
			continue
		}
		scrapersRaw, ok := recv.Config["scrapers"]
		if !ok {
			continue
		}
		scrapers, ok := scrapersRaw.(map[string]any)
		if !ok {
			continue
		}
		fsRaw, ok := scrapers["filesystem"]
		if !ok {
			continue
		}
		// filesystem scraper is present — check for exclusions
		hasExclusions := false
		if fsCfg, ok := fsRaw.(map[string]any); ok {
			_, hasMountPoints := fsCfg["exclude_mount_points"]
			_, hasFsTypes := fsCfg["exclude_fs_types"]
			hasExclusions = hasMountPoints || hasFsTypes
		}
		// fsRaw could also be nil (empty scraper config like `filesystem:`)
		if !hasExclusions {
			findings = append(findings, rules.Finding{
				RuleID:     r.ID(),
				Title:      "Filesystem scraper lacks exclusions",
				Severity:   rules.SeverityInfo,
				Confidence: rules.ConfidenceHigh,
				Evidence:   fmt.Sprintf("Receiver %q has a filesystem scraper without exclude_mount_points or exclude_fs_types.", name),
				Implication: "Virtual and pseudo-filesystems generate high-cardinality metrics that are rarely useful for " +
					"monitoring. They inflate storage costs and clutter dashboards. Excluding virtual filesystems typically " +
					"removes 50-80% of filesystem metric series.\n" +
					"However, if monitoring virtual filesystems is intentional (e.g. for container storage " +
					"tracking), exclusions should be adjusted rather than added blindly.",

				Scope: fmt.Sprintf("receiver:%s", name),
				Snippet: fmt.Sprintf(`receivers:
  %s:
    scrapers:
      filesystem:
        exclude_mount_points:
          match_type: regexp
          mount_points:
            - ^/(dev|proc|sys|run)($|/)
            - ^/snap/
        exclude_fs_types:
          match_type: strict
          fs_types:
            - autofs
            - binfmt_misc
            - bpf
            - cgroup2?
            - configfs
            - debugfs
            - devpts
            - devtmpfs
            - fusectl
            - hugetlbfs
            - mqueue
            - nsfs
            - overlay
            - proc
            - procfs
            - pstore
            - rpc_pipefs
            - securityfs
            - selinuxfs
            - squashfs
            - sysfs
            - tmpfs
            - tracefs`, name),
				Recommendation: "Add exclusion filters to the filesystem scraper configuration.",
			})
		}
	}
	return findings
}
