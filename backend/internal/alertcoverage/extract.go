package alertcoverage

import (
	"strings"

	"github.com/VictoriaMetrics/metricsql"
)

// ExtractMetrics parses a PromQL expression and returns the metric names
// referenced in it, plus whether the expression uses absent() or
// absent_over_time().
func ExtractMetrics(expr string) ([]string, bool, error) {
	e, err := metricsql.Parse(expr)
	if err != nil {
		return nil, false, err
	}

	seen := make(map[string]struct{})
	var names []string
	usesAbsent := false

	metricsql.VisitAll(e, func(node metricsql.Expr) {
		switch t := node.(type) {
		case *metricsql.FuncExpr:
			lower := strings.ToLower(t.Name)
			if lower == "absent" || lower == "absent_over_time" {
				usesAbsent = true
			}
		case *metricsql.MetricExpr:
			for _, lfs := range t.LabelFilterss {
				for _, lf := range lfs {
					if lf.Label != "__name__" {
						continue
					}
					if lf.IsNegative {
						continue
					}
					if lf.IsRegexp {
						// Regex __name__ filters — store the pattern prefixed
						// with "~" so callers can distinguish them.
						name := "~" + lf.Value
						if _, ok := seen[name]; !ok {
							seen[name] = struct{}{}
							names = append(names, name)
						}
					} else {
						if _, ok := seen[lf.Value]; !ok {
							seen[lf.Value] = struct{}{}
							names = append(names, lf.Value)
						}
					}
				}
			}
		}
	})

	return names, usesAbsent, nil
}
