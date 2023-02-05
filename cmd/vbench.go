package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"v-bench/common"
	"v-bench/virtual_cluster"
)

const defaultConfigPath = "./config/default/config.json"
const metaInfoFileName = "system_info.yaml"
const stdoutFileName = "stdout.log"

func main() {
	benchmarkConfigPath := flag.String("config", defaultConfigPath, "benchmark config file")
	flag.Parse()

	benchmarkConfigPaths := readBenchmarkConfigs(benchmarkConfigPath)
	benchmarkConfigs := parseBenchmarkConfigs(benchmarkConfigPaths)

	vclusterManager := &virtual_cluster.StandardVirtualClusterManager{}

	for _, benchmarkConfig := range benchmarkConfigs {
		runExperiment(benchmarkConfig, vclusterManager)
	}
}

func readBenchmarkConfigs(benchmarkConfigPath *string) []string {
	benchmarkConfigFileInfo, err := os.Stat(*benchmarkConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	var benchmarkConfigs []string
	if benchmarkConfigFileInfo.Mode().IsDir() {
		configDir := *benchmarkConfigPath
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
				benchmarkConfigs = append(benchmarkConfigs, configDir+"/"+file.Name())
			}
		}
	} else {
		benchmarkConfigs = append(benchmarkConfigs, *benchmarkConfigPath)
	}

	return benchmarkConfigs
}

func parseBenchmarkConfigs(benchmarkConfigPaths []string) []common.TestConfig {
	var testConfigs []common.TestConfig

	for _, benchmarkConfigPath := range benchmarkConfigPaths {
		configFile, err := os.OpenFile(benchmarkConfigPath, os.O_RDWR, 0666)
		if err != nil {
			fmt.Printf("Can not open benchmark config file %v, benchmark exited. \n",
				benchmarkConfigPath)
			os.Exit(1)
		}

		decoder := json.NewDecoder(configFile)
		testConfig := common.TestConfig{}
		err = decoder.Decode(&testConfig)

		if err != nil {
			fmt.Printf("Can not parse benchmark config file, error: \n %v \n", err)
			os.Exit(1)
		}

		testConfigs = append(testConfigs, testConfig)

		err = configFile.Close()
		if err != nil {
			fmt.Printf("Can not close benchmark config file, error: \n %v \n", err)
		}
	}

	return testConfigs
}

func runExperiment(benchmarkConfig common.TestConfig, vclusterManager virtual_cluster.VirtualClusterManager) {
	if benchmarkConfig.ClusterType == "virtual" {
		vclusterManager.Create(benchmarkConfig)
	}

	createInitialResources(benchmarkConfig)
	runTests(benchmarkConfig)
	cleanupInitialResources(benchmarkConfig)

	if benchmarkConfig.ClusterType == "virtual" {
		vclusterManager.Create(benchmarkConfig)
	}
}

func createInitialResources(benchmarkConfig common.TestConfig) {

}

func runTests(benchmarkConfig common.TestConfig) {
	experimentDirName := getExperimentDirName(benchmarkConfig)
	testOutputPath := filepath.Join(benchmarkConfig.RunsBasePath, experimentDirName, benchmarkConfig.TestConfigName)
	if err := os.MkdirAll(testOutputPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	_, err := copyFile(benchmarkConfig.MetaInfoPath, filepath.Join(benchmarkConfig.RunsBasePath, experimentDirName, metaInfoFileName))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created directory for test run: %v.\n", testOutputPath)
	fmt.Printf("Running test for all clusters in parallel...\n")

	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		if err := os.MkdirAll(filepath.Join(testOutputPath, clusterConfig.Name), os.ModePerm); err != nil {
			log.Fatal(err)
		}

		// TODO: переделать на Wait или горутины, разобраться с передачей env и перенаправлением stdout. Блокирующий вариант: Output() или Run(), неблокирующий вариант: Start() + Wait().
		createCmd := exec.Command("kbench", "-benchconfig", fmt.Sprintf("../../k-bench-test-configs/%v", benchmarkConfig.TestConfigName), "-outdir", filepath.Join(testOutputPath, clusterConfig.Name))
		createCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath)))
		outfile, err := os.Create(filepath.Join(testOutputPath, clusterConfig.Name, stdoutFileName))
		if err != nil {
			panic(err)
		}
		createCmd.Stdout = outfile

		err = createCmd.Run()
		if err != nil {
			panic(err)
		}

		err = outfile.Close()
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Finished running tests.")
}

func getExperimentDirName(benchmarkConfig common.TestConfig) string {
	i := 1
	currentDate := time.Now().Format("2006_01_02")
	experimentDirName := fmt.Sprintf("%v_%03d", currentDate, i)
	for exists, _ := isFileOrDirExisting(filepath.Join(benchmarkConfig.RunsBasePath, experimentDirName)); !exists; exists, _ = isFileOrDirExisting(filepath.Join(benchmarkConfig.RunsBasePath, experimentDirName)) {
		i++
		experimentDirName = fmt.Sprintf("%v_%03d", currentDate, i)
	}

	return experimentDirName
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

func copyFile(src, dst string) (written int64, err error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func cleanupInitialResources(benchmarkConfig common.TestConfig) {

}
