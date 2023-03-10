package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"time"
	"v-bench/internal/cmd"
	"v-bench/internal/util"
	"v-bench/measurement"
	"v-bench/reporting"
	"v-bench/virtual_cluster/monitoring"
)

const (
	defaultConfigPath  = "./config/metrics/config.json"
	defaultOutputPath  = "./"
	defaultClusterName = ""
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	benchmarkOutputPath := flag.String("out", defaultOutputPath, "output path")
	clusterName := flag.String("cluster", defaultClusterName, "cluster name")
	flag.Parse()

	pathExpander := util.StandardPathExpander{}

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(pathExpander.ExpandPath(*benchmarkConfigPath))
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	promProvisioner, err := monitoring.NewPrometheusProvisioner(benchmarkConfigs[0].RootKubeConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	if benchmarkConfigs[0].ShouldProvisionMonitoring {
		err := promProvisioner.Provision(monitoring.NewProvisionerTemplateDto(benchmarkConfigs[0].ClusterConfigs[0].Name, benchmarkConfigs[0].ClusterConfigs[0].Namespace))
		if err != nil {
			log.Fatal(err)
		}
	}

	startTime := time.Now().Add(-5 * time.Minute)
	measurementContext := measurement.NewContext([]string{*clusterName}, startTime)
	metricCollector, _ := measurement.NewMetricCollector(benchmarkConfigs[0].RootKubeConfigPath)
	metricCollector.CollectMetrics(measurementContext, cmd.CollectConfigFromTestConfig(benchmarkConfigs[0]))

	reporter := &reporting.JsonReporter{}
	reporter.Report(pathExpander.ExpandPath(*benchmarkOutputPath), measurementContext)
}
