package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"v-bench/cli"
	"v-bench/internal/cmd"
	"v-bench/internal/util"
	"v-bench/virtual_cluster"
)

const (
	defaultConfigPath = "./config/default/config.json"
	defaultOutputPath = "./runs"
)

func main() {
	benchmarkConfigUnfoldedPath := flag.String("config", defaultConfigPath, "config file or directory with config files")
	benchmarkOutputPath := flag.String("out", defaultOutputPath, "output directory")
	var logFlags cli.SliceValue
	flag.Var(&logFlags, "log", "log `flags`, several allowed [debug,info,warn,error,fatal,color,nocolor,json]")
	flag.Parse()
	cli.ApplyLogFlags(logFlags)

	pathExpander := util.StandardPathExpander{}

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(pathExpander.ExpandPath(*benchmarkConfigUnfoldedPath))
	benchmarkConfigs := cmd.ParseBenchmarkConfigs(benchmarkConfigPaths)

	experimentDirName, err := cmd.CreateExperimentDir(*benchmarkOutputPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, benchmarkConfig := range benchmarkConfigs {
		err, prometheusQueryExecutor := cmd.CreatePrometheusQueryExecutor(benchmarkConfig)
		if err != nil {
			log.Fatal(err)
		}
		vclusterManager := virtual_cluster.NewStandardVirtualClusterManager(prometheusQueryExecutor)

		cmd.RunExperiment(vclusterManager, benchmarkConfig, pathExpander.ExpandPath(*benchmarkOutputPath), experimentDirName, prometheusQueryExecutor)
	}
}
