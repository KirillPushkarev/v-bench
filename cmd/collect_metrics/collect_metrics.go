package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"time"
	"v-bench/config"
	"v-bench/internal/cmd"
	"v-bench/internal/util"
	"v-bench/measurement"
	"v-bench/reporting"
)

const (
	defaultConfigPath = "./config/metrics/config.json"
	defaultOutputPath = "./"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	benchmarkOutputPath := flag.String("out", defaultOutputPath, "output path")
	flag.Parse()

	pathExpander := util.StandardPathExpander{}

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(pathExpander.ExpandPath(*benchmarkConfigPath))
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	benchmarkConfig := benchmarkConfigs[0]
	collectMetrics(benchmarkConfig, pathExpander.ExpandPath(*benchmarkOutputPath))
}

func collectMetrics(benchmarkConfig *config.TestConfig, benchmarkOutputPath string) {
	startTime := time.Now().Add(-5 * time.Minute)
	clusterNames := util.Map(
		benchmarkConfig.ClusterConfigs,
		func(clusterConfig config.ClusterConfig) string {
			return clusterConfig.Name
		},
	)
	hostClusterNames := []string{""}
	virtualClusterNames := util.Filter(clusterNames, func(clusterName string) bool { return clusterName != "host" })
	hostMeasurementContext := measurement.NewContext(hostClusterNames, startTime)
	virtualMeasurementContext := measurement.NewContext(virtualClusterNames, startTime)
	err, prometheusQueryExecutor := cmd.CreatePrometheusQueryExecutor(benchmarkConfig)
	if err != nil {
		log.Fatal(err)
	}
	metricCollector := measurement.NewMetricCollector(prometheusQueryExecutor)

	metricCollector.CollectMetrics(hostMeasurementContext, measurement.NewCollectConfig(true))
	if benchmarkConfig.ClusterType == config.ClusterTypeVirtual {
		metricCollector.CollectMetrics(virtualMeasurementContext, measurement.NewCollectConfig(false))
	}

	reporter := &reporting.JsonReporter{}
	reporter.Report(benchmarkConfig, benchmarkOutputPath, hostMeasurementContext, virtualMeasurementContext)

	log.Info("Finished collecting metrics.")
}
