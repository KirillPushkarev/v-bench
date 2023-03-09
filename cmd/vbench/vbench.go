package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"v-bench/internal/cmd"
	"v-bench/virtual_cluster"
)

const (
	defaultConfigPath = "./config/default/config.json"
	defaultOutputPath = "./runs"
)

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	benchmarkOutputPath := flag.String("config", defaultOutputPath, "benchmark output path")
	flag.Parse()

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(benchmarkConfigPath)
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	for _, benchmarkConfig := range benchmarkConfigs {
		vclusterManager, err := virtual_cluster.NewStandardVirtualClusterManager(benchmarkConfig.RootKubeConfigPath)
		if err != nil {
			log.Fatal(err)
		}
		cmd.RunExperiment(vclusterManager, benchmarkConfig, *benchmarkOutputPath)
	}
}
