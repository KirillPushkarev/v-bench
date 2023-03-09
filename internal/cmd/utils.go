package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"v-bench/config"
	"v-bench/internal/util"
	"v-bench/measurement"
	"v-bench/reporting"
	"v-bench/virtual_cluster"
)

const (
	metaInfoFileName     = "system_info.yaml"
	stdoutFileName       = "stdout.log"
	maxExperimentsPerDay = 1000
)

func ReadBenchmarkConfigPaths(benchmarkConfigPath string) []string {
	benchmarkConfigFileInfo, err := os.Stat(benchmarkConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	var benchmarkConfigPaths []string
	if benchmarkConfigFileInfo.Mode().IsDir() {
		configDir := benchmarkConfigPath
		f, err := os.Open(configDir)
		if err != nil {
			log.Fatal(err)
		}
		files, err := f.Readdir(-1)
		if err != nil {
			log.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
				benchmarkConfigPaths = append(benchmarkConfigPaths, filepath.Join(configDir, file.Name()))
			}
		}
	} else {
		benchmarkConfigPaths = append(benchmarkConfigPaths, benchmarkConfigPath)
	}

	return benchmarkConfigPaths
}

func ParseBenchmarkConfigs(benchmarkConfigPaths []string) []config.TestConfig {
	var testConfigs []config.TestConfig

	for _, benchmarkConfigPath := range benchmarkConfigPaths {
		configFile, err := os.OpenFile(benchmarkConfigPath, os.O_RDWR, 0666)
		if err != nil {
			log.Fatalf("Can not open benchmark config file %v, benchmark exited. \n", benchmarkConfigPath)
		}

		decoder := json.NewDecoder(configFile)
		testConfig := config.TestConfig{ConfigPath: benchmarkConfigPath}
		err = decoder.Decode(&testConfig)
		if err != nil {
			log.Fatalf("Can not parse benchmark config file, error: \n %v \n", err)
		}
		testConfig.ExpandPaths()

		testConfigs = append(testConfigs, testConfig)

		err = configFile.Close()
		if err != nil {
			log.Fatalf("Can not close benchmark config file, error: \n %v \n", err)
		}
	}

	return testConfigs
}

func RunExperiment(vclusterManager virtual_cluster.VirtualClusterManager, benchmarkConfig config.TestConfig, benchmarkOutputPath string) {
	if benchmarkConfig.ClusterType == "virtual" {
		vclusterManager.Create(benchmarkConfig)
	}

	createInitialResources(benchmarkConfig)
	runTests(benchmarkConfig, benchmarkOutputPath)
	cleanupInitialResources(benchmarkConfig)

	if benchmarkConfig.ClusterType == "virtual" {
		vclusterManager.Delete(benchmarkConfig)
	}
}

func createInitialResources(benchmarkConfig config.TestConfig) {
	if benchmarkConfig.InitialResources.ConfigMap == 0 {
		return
	}

	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		kubeconfigPath := filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath)
		createNamespaceCmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%v", kubeconfigPath), "create", "namespaces", "initial")
		stdout, err := createNamespaceCmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		log.Info(string(stdout))

		for i := 0; i < benchmarkConfig.InitialResources.ConfigMap; i++ {
			createConfigMapCmdShellCommand := fmt.Sprintf("sed \"s/{{configmap-name}}/configmap-%v/g\" ../../k8s-specs/prepopulate/configmap-1m.yaml | kubectl --kubeconfig=%v create -f -;", i, kubeconfigPath)
			createConfigMapCmd := exec.Command("bash", "-c", createConfigMapCmdShellCommand)
			stdout, err := createConfigMapCmd.Output()
			if err != nil {
				log.Fatal(err)
			}
			log.Info(string(stdout))
		}
	}

	log.Info("Created initial resources.")
}

func runTests(benchmarkConfig config.TestConfig, benchmarkOutputPath string) {
	experimentDirName, err := createExperimentDir(benchmarkOutputPath)
	if err != nil {
		log.Fatal(err)
	}
	testOutputPath := filepath.Join(benchmarkOutputPath, experimentDirName, benchmarkConfig.Name)
	if err := os.MkdirAll(testOutputPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	_, err = copyFile(benchmarkConfig.ConfigPath, filepath.Join(testOutputPath, filepath.Base(benchmarkConfig.ConfigPath)))
	if err != nil {
		log.Fatal(err)
	}
	_, err = copyFile(benchmarkConfig.MetaInfoPath, filepath.Join(benchmarkOutputPath, experimentDirName, metaInfoFileName))
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Created directory for test run: %v.", testOutputPath)
	log.Info("Running test for all clusters in parallel...")

	startTime := time.Now()
	clusterNames := util.Map(
		benchmarkConfig.ClusterConfigs,
		func(clusterConfig config.ClusterConfig) string {
			if clusterConfig.Name != "host" {
				return clusterConfig.Name
			} else {
				return ""
			}
		},
	)
	measurementContext := measurement.NewContext(clusterNames, startTime)
	metricCollector, _ := measurement.NewMetricCollector(benchmarkConfig.RootKubeConfigPath)

	var wg sync.WaitGroup
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		clusterConfig := clusterConfig
		wg.Add(1)

		go func() {
			if err := os.MkdirAll(filepath.Join(testOutputPath, clusterConfig.Name), os.ModePerm); err != nil {
				log.Fatal(err)
			}

			cmd := exec.Command("kbench", "-benchconfig", fmt.Sprintf("../../k-bench-test-configs/%v", benchmarkConfig.TestConfigName), "-outdir", filepath.Join(testOutputPath, clusterConfig.Name))
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath)))
			outfile, err := os.Create(filepath.Join(testOutputPath, clusterConfig.Name, stdoutFileName))
			if err != nil {
				log.Fatal(err)
			}
			cmd.Stdout = outfile

			err = cmd.Run()
			if err != nil {
				log.Fatal(err)
			}

			err = outfile.Close()
			if err != nil {
				log.Fatal(err)
			}

			wg.Done()
		}()
	}

	wg.Wait()

	metricCollector.CollectMetrics(measurementContext, CollectConfigFromTestConfig(benchmarkConfig))
	reporter := &reporting.JsonReporter{}
	reporter.Report(testOutputPath, measurementContext)

	log.Info("Finished running tests.")
}

func createExperimentDir(benchmarkOutputPath string) (string, error) {
	i := 1
	currentDate := time.Now().Format("2006_01_02")
	experimentDirName := fmt.Sprintf("%v_%03d", currentDate, i)
	exists, _ := isFileOrDirExisting(filepath.Join(benchmarkOutputPath, experimentDirName))
	for exists && i < maxExperimentsPerDay {
		i++
		experimentDirName = fmt.Sprintf("%v_%03d", currentDate, i)
		exists, _ = isFileOrDirExisting(filepath.Join(benchmarkOutputPath, experimentDirName))
	}

	if i == maxExperimentsPerDay {
		return "", errors.New("unable to find the next name for the experiment dir")
	}

	return experimentDirName, nil
}

func isFileOrDirExisting(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func copyFile(sourcePath, destinationPath string) (bytesWritten int64, err error) {
	sourceFileStat, err := os.Stat(sourcePath)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", sourcePath)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return 0, err
	}

	destination, err := os.Create(destinationPath)
	if err != nil {
		return 0, err
	}
	nBytes, err := io.Copy(destination, source)

	err = source.Close()
	if err != nil {
		return 0, err
	}
	err = destination.Close()
	if err != nil {
		return 0, err
	}

	return nBytes, err
}

func cleanupInitialResources(benchmarkConfig config.TestConfig) {
	if benchmarkConfig.InitialResources.ConfigMap == 0 {
		return
	}

	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath)), "delete", "namespaces", "initial")
		stdout, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		log.Info(string(stdout))
	}

	log.Info("Deleted initial resources.")
}

func CollectConfigFromTestConfig(testConfig config.TestConfig) *measurement.CollectConfig {
	if testConfig.ClusterType == config.HostCluster {
		return measurement.NewCollectConfig(true)
	}

	return measurement.NewCollectConfig(false)
}
