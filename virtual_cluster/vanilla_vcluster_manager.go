package virtual_cluster

import (
	_ "embed"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"text/template"
	"v-bench/config"
	"v-bench/k8s"
	"v-bench/measurement"
	"v-bench/virtual_cluster/monitoring"
)

type StandardVirtualClusterManager struct {
	PrometheusProvisioner *monitoring.PrometheusProvisioner
}

type TemplateDto struct {
	ClusterName      string
	ClusterNamespace string
	IngressDomain    string
}

var (
	//go:embed templates/namespace.yaml
	namespaceConfig []byte
	//go:embed templates/ingress/ingress.yaml
	ingressConfig []byte
	//go:embed templates/ingress/values.yaml
	vclusterValues []byte
)

func NewStandardVirtualClusterManager(prometheusQueryExecutor *measurement.PrometheusQueryExecutor) (*StandardVirtualClusterManager, error) {
	provisioner := monitoring.NewPrometheusProvisioner(prometheusQueryExecutor)
	return &StandardVirtualClusterManager{PrometheusProvisioner: provisioner}, nil
}

func (virtualClusterManager StandardVirtualClusterManager) Create(benchmarkConfig *config.TestConfig) {
	var wg sync.WaitGroup
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		clusterConfig := clusterConfig
		wg.Add(1)

		go func() {
			if benchmarkConfig.VirtualClusterConnType == config.VirtualClusterConnTypeIngress {
				virtualClusterManager.createWithIngressConnection(benchmarkConfig, &clusterConfig)
			} else {
				createCmdArgs := []string{"create", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--connect=false"}
				createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
				createCmd := exec.Command("vcluster", createCmdArgs...)
				createCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.RootKubeConfigPath)))
				stdoutStderr, err := createCmd.CombinedOutput()
				if err != nil {
					log.Fatal("Cluster %v; create command error: %e, create command result: %v", clusterConfig.Name, err, string(stdoutStderr))
				}
				log.Infof("Cluster %v; create command result: %v", clusterConfig.Name, string(stdoutStderr))

				connectCmdArgs := []string{"connect", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath))}
				connectCmd := exec.Command("vcluster", connectCmdArgs...)
				connectCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.RootKubeConfigPath)))
				stdoutStderr, err = connectCmd.CombinedOutput()
				if err != nil {
					log.Fatal("Cluster %v; connect command error: %e, connect command result: %v", clusterConfig.Name, err, string(stdoutStderr))
				}
				log.Infof("Cluster %v; connect command result: %v", clusterConfig.Name, string(stdoutStderr))
			}

			if benchmarkConfig.ShouldProvisionMonitoring {
				err := virtualClusterManager.PrometheusProvisioner.Provision(monitoring.NewProvisionerTemplateDto(clusterConfig.Name, clusterConfig.Namespace))
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
	var wg sync.WaitGroup

	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		clusterConfig := clusterConfig
		wg.Add(1)

		go func() {
			cmd := exec.Command("vcluster", "delete", clusterConfig.Name, "-n", clusterConfig.Namespace)
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.RootKubeConfigPath)))
			stdoutStderr, err := cmd.CombinedOutput()
			if err != nil {
				log.Fatal("Cluster %v; delete command error: %e, delete command result: %v", clusterConfig.Name, err, string(stdoutStderr))
			}
			log.Infof("Cluster %v; delete command result: %v", clusterConfig.Name, string(stdoutStderr))

			wg.Done()
		}()
	}

	wg.Wait()

	log.Info("Deleted virtual clusters.")
}

func (virtualClusterManager StandardVirtualClusterManager) createWithIngressConnection(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig) {
	virtualClusterManager.createNamespace(clusterConfig)
	virtualClusterManager.createIngress(benchmarkConfig, clusterConfig)

	t, err := template.New("vclusterValues").Parse(string(vclusterValues))
	if err != nil {
		log.Fatal(err)
	}
	data := TemplateDto{
		ClusterName:      clusterConfig.Name,
		ClusterNamespace: clusterConfig.Namespace,
		IngressDomain:    benchmarkConfig.IngressDomain,
	}
	valuesFile, err := os.CreateTemp("", "vcluster-values-*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	err = t.Execute(valuesFile, data)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err = os.Remove(valuesFile.Name())
		if err != nil {
			log.Error("Can't remove temp file: %s", err)
		}
	}()

	createCmdArgs := []string{"create", clusterConfig.Name, "-n", clusterConfig.Namespace, "--connect=false"}
	createCmdArgs = append(createCmdArgs, "-f", valuesFile.Name())
	createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
	createCmd := exec.Command("vcluster", createCmdArgs...)
	createCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.RootKubeConfigPath)))
	stdoutStderr, err := createCmd.CombinedOutput()
	if err != nil {
		log.Fatal("Cluster %v; create command error: %e, create command result: %v", clusterConfig.Name, err, string(stdoutStderr))
	}
	log.Infof("Cluster %v; create command result: %v", clusterConfig.Name, string(stdoutStderr))

	connectCmdArgs := []string{"connect", clusterConfig.Name, "-n", clusterConfig.Namespace, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath))}
	connectCmdArgs = append(connectCmdArgs, fmt.Sprintf("--server=https://%s.%s", clusterConfig.Name, benchmarkConfig.IngressDomain))
	connectCmd := exec.Command("vcluster", connectCmdArgs...)
	connectCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", filepath.Join(benchmarkConfig.RootKubeConfigPath)))
	stdoutStderr, err = connectCmd.CombinedOutput()
	if err != nil {
		log.Fatal("Cluster %v; connect command error: %e, connect command result: %v", clusterConfig.Name, err, string(stdoutStderr))
	}
	log.Infof("Cluster %v; connect command result: %v", clusterConfig.Name, string(stdoutStderr))
}

func (virtualClusterManager StandardVirtualClusterManager) createNamespace(clusterConfig *config.ClusterConfig) {
	data := TemplateDto{
		ClusterNamespace: clusterConfig.Namespace,
	}
	err := k8s.ApplyManifest(clusterConfig, "namespaceConfig", string(namespaceConfig), data)
	if err != nil {
		log.Fatal("Cluster %v; can't create Namespace. Error: %v", clusterConfig.Name, err)
	}
}

func (virtualClusterManager StandardVirtualClusterManager) createIngress(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig) {
	data := TemplateDto{
		ClusterName:      clusterConfig.Name,
		ClusterNamespace: clusterConfig.Namespace,
		IngressDomain:    benchmarkConfig.IngressDomain,
	}
	err := k8s.ApplyManifest(clusterConfig, "ingressConfig", string(ingressConfig), data)
	if err != nil {
		log.Fatal("Cluster %v; can't create Ingress. Error: %v", clusterConfig.Name, err)
	}
}
