package rules

import "github.com/canonical/signal-studio/internal/config"

// HasProcessorType checks if any processor in the list matches the given type.
func HasProcessorType(processors []string, typeName string) bool {
	for _, p := range processors {
		if config.ComponentType(p) == typeName {
			return true
		}
	}
	return false
}
