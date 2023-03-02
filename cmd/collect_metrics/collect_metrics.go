package main

import (
	"flag"
	"time"
	"v-bench/internal/cmd"
	"v-bench/measurement"
	"v-bench/reporting"
)

const (
	defaultConfigPath = "./config/metrics/config.json"
	defaultOutputPath = "./"
)

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	flag.Parse()

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(benchmarkConfigPath)
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	startTime := time.Now().Add(-5 * time.Minute)
	measurementContext := measurement.NewContext(startTime)
	metricCollector, _ := measurement.NewMetricCollector(benchmarkConfigs[0].RootKubeConfigPath)
	metricCollector.CollectMetrics(measurementContext)

	reporter := &reporting.JsonReporter{}
	reporter.Report(defaultOutputPath, measurementContext)
}
