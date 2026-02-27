package report

import (
	"io"

	"github.com/canonical/signal-studio/internal/analyze"
)

// Formatter writes a report in a specific output format.
type Formatter interface {
	Format(report *analyze.Report, w io.Writer) error
}
