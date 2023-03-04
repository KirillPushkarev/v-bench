package main

import (
	"flag"
	"time"
	"v-bench/internal/cmd"
	"v-bench/measurement"
	"v-bench/reporting"
)

const (
	defaultConfigPath  = "./config/metrics/config.json"
	defaultOutputPath  = "./"
	defaultClusterName = ""
)

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	outputPath := flag.String("out", defaultOutputPath, "output path")
	clusterName := flag.String("cluster", defaultClusterName, "cluster name")
	flag.Parse()

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(benchmarkConfigPath)
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	startTime := time.Now().Add(-5 * time.Minute)
	measurementContext := measurement.NewContext([]string{*clusterName}, startTime)
	metricCollector, _ := measurement.NewMetricCollector(benchmarkConfigs[0].RootKubeConfigPath)
	metricCollector.CollectMetrics(measurementContext, measurement.NewCollectConfig(false))

	reporter := &reporting.JsonReporter{}
	reporter.Report(*outputPath, measurementContext)
}
