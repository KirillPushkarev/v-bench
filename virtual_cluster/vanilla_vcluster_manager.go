package virtual_cluster

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"v-bench/common"
)

type StandardVirtualClusterManager struct{}

func NewStandardVirtualClusterManager() *StandardVirtualClusterManager {
	return &StandardVirtualClusterManager{}
}

func (StandardVirtualClusterManager) Create(benchmarkConfig common.TestConfig) {
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		createCmdArgs := []string{"create", clusterConfig.Name, "--connect=false"}
		createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
		createCmd := exec.Command("vcluster", createCmdArgs...)
		stdout, err := createCmd.CombinedOutput()
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(stdout))

		connectCmd := exec.Command("vcluster", "connect", clusterConfig.Name, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath)))
		stdout, err = connectCmd.Output()
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(stdout))
	}

	fmt.Println("Created virtual clusters.")
}

func (StandardVirtualClusterManager) Delete(benchmarkConfig common.TestConfig) {
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		cmd := exec.Command("vcluster", "delete", clusterConfig.Name)
		stdout, err := cmd.Output()
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(stdout))
	}

	fmt.Println("Deleted virtual clusters.")
}
