package virtual_cluster

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"path/filepath"
	"sync"
	"v-bench/config"
	"v-bench/virtual_cluster/monitoring"
)

type StandardVirtualClusterManager struct {
	PrometheusProvisioner *monitoring.PrometheusProvisioner
}

func NewStandardVirtualClusterManager(kubeconfigPath string) (*StandardVirtualClusterManager, error) {
	provisioner, err := monitoring.NewPrometheusProvisioner(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return &StandardVirtualClusterManager{PrometheusProvisioner: provisioner}, nil
}

func (virtualClusterManager StandardVirtualClusterManager) Create(benchmarkConfig *config.TestConfig) {
	var wg sync.WaitGroup
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		clusterConfig := clusterConfig
		wg.Add(1)

		go func() {
			createCmdArgs := []string{"create", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--connect=false"}
			createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
			createCmd := exec.Command("vcluster", createCmdArgs...)
			stdout, err := createCmd.CombinedOutput()
			if err != nil {
				log.Fatal(err)
			}
			log.Infof("Cluster %v; create command result: %v", clusterConfig.Name, string(stdout))

			connectCmdArgs := []string{"connect", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath))}
			connectCmd := exec.Command("vcluster", connectCmdArgs...)
			stdout, err = connectCmd.Output()
			if err != nil {
				log.Fatal(err)
			}
			log.Infof("Cluster %v; connect command result: %v", clusterConfig.Name, string(stdout))

			if benchmarkConfig.ShouldProvisionMonitoring {
				err = virtualClusterManager.PrometheusProvisioner.Provision(monitoring.NewProvisionerTemplateDto(clusterConfig.Name, clusterConfig.Namespace))
				if err != nil {
					log.Fatal(err)
				}
			}

			wg.Done()
		}()
	}

	wg.Wait()

	log.Info("Created virtual clusters.")
}

func (StandardVirtualClusterManager) Delete(benchmarkConfig *config.TestConfig) {
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		cmd := exec.Command("vcluster", "delete", clusterConfig.Name, "--namespace", clusterConfig.Namespace)
		stdout, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		log.Info(string(stdout))
	}

	log.Info("Deleted virtual clusters.")
}
