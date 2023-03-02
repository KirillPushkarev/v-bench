package virtual_cluster

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"path/filepath"
	"v-bench/config"
)

type StandardVirtualClusterManager struct{}

func NewStandardVirtualClusterManager() *StandardVirtualClusterManager {
	return &StandardVirtualClusterManager{}
}

func (StandardVirtualClusterManager) Create(benchmarkConfig config.TestConfig) {
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		createCmdArgs := []string{"create", clusterConfig.Name, "--connect=false"}
		createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
		createCmd := exec.Command("vcluster", createCmdArgs...)
		stdout, err := createCmd.CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}
		log.Info(string(stdout))

		connectCmd := exec.Command("vcluster", "connect", clusterConfig.Name, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath)))
		stdout, err = connectCmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		log.Info(string(stdout))
	}

	log.Info("Created virtual clusters.")
}

func (StandardVirtualClusterManager) Delete(benchmarkConfig config.TestConfig) {
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		cmd := exec.Command("vcluster", "delete", clusterConfig.Name)
		stdout, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		log.Info(string(stdout))
	}

	log.Info("Deleted virtual clusters.")
}
