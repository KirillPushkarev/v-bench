package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"v-bench/internal/cmd"
	"v-bench/internal/util"
	"v-bench/virtual_cluster"
)

// sliceValue stores multi-value command line arguments.
type sliceValue []string

// String makes sliceValue implement flag.Value interface.
func (s *sliceValue) String() string {
	return fmt.Sprintf("%s", *s)
}

// Set makes sliceValue implement flag.Value interface.
func (s *sliceValue) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		if len(v) > 0 {
			*s = append(*s, v)
		}
	}
	return nil
}

const (
	defaultConfigPath = "./config/default/config.json"
	defaultOutputPath = "./runs"
)

var (
	logflags sliceValue
	levels   = regexp.MustCompile("^(debug|info|warn|error|fatal)$")
	colors   = regexp.MustCompile("^(no)?colou?rs?$")
	json     = regexp.MustCompile("^json$")
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "config file or directory with config files")
	benchmarkOutputPath := flag.String("out", defaultOutputPath, "output directory")
	flag.Var(&logflags, "log", "log `flags`, several allowed [debug,info,warn,error,fatal,color,nocolor,json]")
	flag.Parse()
	parseLogFlags()

	pathExpander := util.StandardPathExpander{}

	benchmarkConfigPaths := cmd.ReadBenchmarkConfigPaths(pathExpander.ExpandPath(*benchmarkConfigPath))
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

func parseLogFlags() {
	log.SetLevel(log.InfoLevel)

	for _, f := range logflags {
		if levels.MatchString(f) {
			lvl, err := log.ParseLevel(f)
			if err != nil {
				// Should never happen since we select correct levels
				// Unless logrus commits a breaking change on level names
				panic(fmt.Errorf("invalid log level: %s", err.Error()))
			}
			log.SetLevel(lvl)
		} else if colors.MatchString(f) {
			if f[:2] == "no" {
				log.SetFormatter(&log.TextFormatter{DisableColors: true})
			} else {
				log.SetFormatter(&log.TextFormatter{ForceColors: true})
			}
		} else if json.MatchString(f) {
			log.SetFormatter(&log.JSONFormatter{})
		}
	}
}
