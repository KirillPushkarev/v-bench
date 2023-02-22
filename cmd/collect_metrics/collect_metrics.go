package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"time"
	"v-bench/internal/cmd"
	"v-bench/measurement"
)

const (
	defaultConfigPath = "./config/metrics/config.json"
)

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	flag.Parse()

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(benchmarkConfigPath)
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	startTime := time.Now().Add(-5 * time.Minute)
	measurementContext := measurement.NewContext(startTime)
	metricCollector := measurement.NewMetricCollector()
	metricCollector.CollectMetrics(benchmarkConfigs[0], measurementContext)

	metricsSummary, _ := json.MarshalIndent(measurementContext.Metrics, "", "\t")
	fmt.Print(string(metricsSummary))
}
