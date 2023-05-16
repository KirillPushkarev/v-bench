package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"v-bench/internal/cmd"
	"v-bench/internal/util"
	"v-bench/virtual_cluster"
)

const (
	defaultConfigPath = "./config/default/config.json"
	defaultOutputPath = "./runs"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	benchmarkOutputPath := flag.String("out", defaultOutputPath, "benchmark output path")
	flag.Parse()

	pathExpander := util.StandardPathExpander{}

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(pathExpander.ExpandPath(*benchmarkConfigPath))
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	for _, benchmarkConfig := range benchmarkConfigs {
		err, prometheusQueryExecutor := cmd.CreatePrometheusQueryExecutor(benchmarkConfig)
		if err != nil {
			log.Fatal(err)
		}

		vclusterManager, err := virtual_cluster.NewStandardVirtualClusterManager(prometheusQueryExecutor)
		if err != nil {
			log.Fatal(err)
		}

		cmd.RunExperiment(vclusterManager, benchmarkConfig, pathExpander.ExpandPath(*benchmarkOutputPath), prometheusQueryExecutor)
	}
}
