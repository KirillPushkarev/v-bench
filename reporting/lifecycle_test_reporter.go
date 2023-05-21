package reporting

import (
	"time"
	"v-bench/config"
)

type LifeCycleReporter interface {
	Report(benchmarkConfig *config.LifecycleTestConfig, outputPath string, createDurations []time.Duration, deleteDurations []time.Duration)
}
