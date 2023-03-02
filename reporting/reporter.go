package reporting

import (
	"v-bench/measurement"
)

type Reporter interface {
	Report(outputPath string, measurementContext *measurement.Context)
}
