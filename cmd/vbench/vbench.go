package main

import (
	"flag"
	"v-bench/internal/cmd"
	"v-bench/virtual_cluster"
)

const (
	defaultConfigPath = "./config/default/config.json"
	defaultOutputPath = "./"
)

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	flag.Parse()

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(benchmarkConfigPath)
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	vclusterManager := virtual_cluster.NewStandardVirtualClusterManager()

	for _, benchmarkConfig := range benchmarkConfigs {
		cmd.RunExperiment(benchmarkConfig, vclusterManager)
	}
}
