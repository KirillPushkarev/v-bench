package reporting

import (
	"v-bench/config"
	"v-bench/measurement"
)

type Reporter interface {
	Report(benchmarkConfig *config.TestConfig, outputPath string, hostMeasurementContext *measurement.Context, virtualMeasurementContext *measurement.Context)
}
