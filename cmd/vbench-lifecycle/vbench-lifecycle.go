package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"v-bench/cli"
	"v-bench/config"
	"v-bench/internal/cmd"
	"v-bench/internal/util"
	"v-bench/measurement"
	"v-bench/reporting"
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
	metaInfoFileName  = "system_info.yaml"
)

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "config file or directory with config files")
	benchmarkOutputPath := flag.String("out", defaultOutputPath, "output directory")
	var logFlags cli.SliceValue
	flag.Var(&logFlags, "log", "log `flags`, several allowed [debug,info,warn,error,fatal,color,nocolor,json]")
	flag.Parse()
	cli.ApplyLogFlags(logFlags)

	pathExpander := util.StandardPathExpander{}

	benchmarkConfig := cmd.ParseLifecycleBenchmarkConfigs([]string{*benchmarkConfigPath})[0]
	experimentDirName, err := cmd.CreateExperimentDir(*benchmarkOutputPath)
	if err != nil {
		log.Fatal(err)
	}

	runTests(&pathExpander, benchmarkConfig, experimentDirName, benchmarkOutputPath)
}

func runTests(pathExpander util.PathExpander, benchmarkConfig *config.LifecycleTestConfig, experimentDirName string, benchMarkOutputPath *string) {
	createTimes := make([]time.Duration, 0)
	deleteTimes := make([]time.Duration, 0)
	cMutex := sync.Mutex{}
	dMutex := sync.Mutex{}
	var wg sync.WaitGroup

	testOutputPath := filepath.Join(*benchMarkOutputPath, experimentDirName, benchmarkConfig.Name)
	if err := os.MkdirAll(testOutputPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	_, err := cmd.CopyFile(benchmarkConfig.ConfigPath, filepath.Join(testOutputPath, filepath.Base(benchmarkConfig.ConfigPath)))
	if err != nil {
		log.Fatal(err)
	}
	_, err = cmd.CopyFile(benchmarkConfig.MetaInfoPath, filepath.Join(*benchMarkOutputPath, experimentDirName, metaInfoFileName))
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Created directory for test run: %v.", testOutputPath)
	log.Info("Running virtual cluster lifecycle tests.")

	for t := 0; t < benchmarkConfig.Parallelism; t++ {
		wg.Add(benchmarkConfig.Count)

		t := t
		go func() {
			for i := 0; i < benchmarkConfig.Count; i++ {
				vclusterManager := virtual_cluster.NewStandardVirtualClusterManager(measurement.NewNoOpQueryExecutor())
				benchmarkConfigAdapter := &config.TestConfig{
					ConfigPath:           benchmarkConfig.ConfigPath,
					Name:                 benchmarkConfig.Name,
					ClusterType:          "virtual",
					RootKubeConfigPath:   benchmarkConfig.RootKubeConfigPath,
					KubeconfigBasePath:   "",
					ClusterConfigs:       nil,
					ClusterCreateOptions: benchmarkConfig.ClusterCreateOptions,
					InitialResources: struct {
						ConfigMap int `json:"configmap"`
					}{},
					TestConfigName:            "",
					MetaInfoPath:              benchmarkConfig.MetaInfoPath,
					ShouldProvisionMonitoring: false,
					PrometheusConnType:        config.PrometheusConnTypeDirect,
					VirtualClusterConnType:    config.VirtualClusterConnTypeDirect,
					IngressDomain:             "",
					PathExpander:              pathExpander,
				}
				clusterConfig := &config.ClusterConfig{
					Name:           fmt.Sprintf("vcluster-%v-%v", t, i),
					Namespace:      fmt.Sprintf("vcluster-%v-%v", t, i),
					KubeConfigPath: "",
				}

				start := time.Now()
				vclusterManager.CreateSingle(benchmarkConfigAdapter, clusterConfig, false)
				elapsed := time.Since(start)
				cMutex.Lock()
				createTimes = append(createTimes, elapsed)
				cMutex.Unlock()

				start = time.Now()
				vclusterManager.DeleteSingle(benchmarkConfigAdapter, clusterConfig)
				elapsed = time.Since(start)
				dMutex.Lock()
				deleteTimes = append(deleteTimes, elapsed)
				dMutex.Unlock()

				wg.Done()
			}
		}()
	}

	wg.Wait()

	reporter := &reporting.LifeCycleTestJsonReporter{}
	reporter.Report(benchmarkConfig, testOutputPath, createTimes, deleteTimes)

	log.Info("Finished running virtual cluster lifecycle tests.")
}
