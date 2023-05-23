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
	"v-bench/internal/util"
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
	vclusterValuesConfig []byte
)

func NewStandardVirtualClusterManager(queryExecutor measurement.QueryExecutor) *StandardVirtualClusterManager {
	provisioner := monitoring.NewPrometheusProvisioner(queryExecutor)
	return &StandardVirtualClusterManager{PrometheusProvisioner: provisioner}
}

func (virtualClusterManager StandardVirtualClusterManager) Create(benchmarkConfig *config.TestConfig) {
	var wg sync.WaitGroup
	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		clusterConfig := clusterConfig
		wg.Add(1)

		go func() {
			indexOfValuesFlag := util.IndexOf(benchmarkConfig.ClusterCreateOptions, "-f")
			if indexOfValuesFlag != -1 {
				if indexOfValuesFlag >= len(benchmarkConfig.ClusterCreateOptions)-1 {
					log.Fatal("Illegal cluster create options")
				}

				valuesInputPath := benchmarkConfig.ClusterCreateOptions[indexOfValuesFlag+1]
				t, err := template.New("vclusterValuesConfig").ParseFiles(valuesInputPath)
				if err != nil {
					log.Fatal(err)
				}
				data := TemplateDto{
					ClusterName:      clusterConfig.Name,
					ClusterNamespace: clusterConfig.Namespace,
					IngressDomain:    benchmarkConfig.IngressDomain,
				}
				valuesOutputFile, err := virtualClusterManager.executeTemplateToTempFile(t, data)
				defer func() {
					err = os.Remove(valuesOutputFile.Name())
					if err != nil {
						log.Error("Can't remove temp file: %s", err)
					}
				}()
				benchmarkConfig.ClusterCreateOptions[indexOfValuesFlag+1] = valuesOutputFile.Name()
			}

			if benchmarkConfig.VirtualClusterConnType == config.VirtualClusterConnTypeIngress {
				virtualClusterManager.createWithIngressConnection(benchmarkConfig, &clusterConfig, true, indexOfValuesFlag != -1)
			} else {
				createCmdArgs := []string{"create", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--connect=false"}
				createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
				createCmd := exec.Command("vcluster", createCmdArgs...)
				createCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
				stdoutStderr, err := createCmd.CombinedOutput()
				if err != nil {
					log.Fatalf("Cluster %v; create command error: %v, create command result: %v", clusterConfig.Name, err, string(stdoutStderr))
				}
				log.Debugf("Cluster %v; create command result: %v", clusterConfig.Name, string(stdoutStderr))

				connectCmdArgs := []string{"connect", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath))}
				connectCmd := exec.Command("vcluster", connectCmdArgs...)
				connectCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
				stdoutStderr, err = connectCmd.CombinedOutput()
				if err != nil {
					log.Fatalf("Cluster %v; connect command error: %v, connect command result: %v", clusterConfig.Name, err, string(stdoutStderr))
				}
				log.Debugf("Cluster %v; connect command result: %v", clusterConfig.Name, string(stdoutStderr))
			}

			if benchmarkConfig.ShouldProvisionMonitoring {
				err := virtualClusterManager.PrometheusProvisioner.Provision(benchmarkConfig.RootKubeConfigPath, monitoring.NewProvisionerTemplateDto(clusterConfig.Name, clusterConfig.Namespace))
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

func (virtualClusterManager StandardVirtualClusterManager) executeTemplateToTempFile(t *template.Template, data TemplateDto) (*os.File, error) {
	valuesOutputFile, err := os.CreateTemp("", "vcluster-values-*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	err = t.Execute(valuesOutputFile, data)
	if err != nil {
		log.Fatal(err)
	}
	err = valuesOutputFile.Close()
	if err != nil {
		log.Error(err)
	}
	return valuesOutputFile, err
}

func (virtualClusterManager StandardVirtualClusterManager) CreateSingle(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig, shouldConnect bool) {
	if benchmarkConfig.VirtualClusterConnType == config.VirtualClusterConnTypeIngress {
		virtualClusterManager.createWithIngressConnection(benchmarkConfig, clusterConfig, shouldConnect, false)
	} else {
		createCmdArgs := []string{"create", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--connect=false"}
		createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
		createCmd := exec.Command("vcluster", createCmdArgs...)
		createCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
		stdoutStderr, err := createCmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Cluster %v; create command error: %v, create command result: %v", clusterConfig.Name, err, string(stdoutStderr))
		}
		log.Debugf("Cluster %v; create command result: %v", clusterConfig.Name, string(stdoutStderr))

		if shouldConnect {
			connectCmdArgs := []string{"connect", clusterConfig.Name, "--namespace", clusterConfig.Namespace, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath))}
			connectCmd := exec.Command("vcluster", connectCmdArgs...)
			connectCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
			stdoutStderr, err = connectCmd.CombinedOutput()
			if err != nil {
				log.Fatalf("Cluster %v; connect command error: %v, connect command result: %v", clusterConfig.Name, err, string(stdoutStderr))
			}
			log.Debugf("Cluster %v; connect command result: %v", clusterConfig.Name, string(stdoutStderr))
		}
	}

	if benchmarkConfig.ShouldProvisionMonitoring {
		err := virtualClusterManager.PrometheusProvisioner.Provision(benchmarkConfig.RootKubeConfigPath, monitoring.NewProvisionerTemplateDto(clusterConfig.Name, clusterConfig.Namespace))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (StandardVirtualClusterManager) Delete(benchmarkConfig *config.TestConfig) {
	var wg sync.WaitGroup

	for _, clusterConfig := range benchmarkConfig.ClusterConfigs {
		clusterConfig := clusterConfig
		wg.Add(1)

		go func() {
			deleteClusterCmd := exec.Command("vcluster", "delete", clusterConfig.Name, "-n", clusterConfig.Namespace)
			deleteClusterCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
			stdoutStderr, err := deleteClusterCmd.CombinedOutput()
			if err != nil {
				log.Fatalf("Cluster %v; error on deleting cluster: %v, result: %v", clusterConfig.Name, err, string(stdoutStderr))
			}
			log.Debugf("Cluster %v; deleted cluster, result: %v", clusterConfig.Name, string(stdoutStderr))

			deleteNamespaceCmd := exec.Command("kubectl", "delete", "namespace", clusterConfig.Namespace)
			deleteNamespaceCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
			stdoutStderr, err = deleteNamespaceCmd.CombinedOutput()
			if err != nil {
				log.Fatalf("Cluster %v; error on deleting namespace for cluster: %v, result: %v", clusterConfig.Name, err, string(stdoutStderr))
			}
			log.Debugf("Cluster %v; deleted namespace for cluster, result: %v", clusterConfig.Name, string(stdoutStderr))

			wg.Done()
		}()
	}

	wg.Wait()

	log.Info("Deleted virtual clusters.")
}

func (StandardVirtualClusterManager) DeleteSingle(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig) {
	deleteClusterCmd := exec.Command("vcluster", "delete", clusterConfig.Name, "-n", clusterConfig.Namespace)
	deleteClusterCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
	stdoutStderr, err := deleteClusterCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Cluster %v; error on deleting cluster: %v, result: %v", clusterConfig.Name, err, string(stdoutStderr))
	}
	log.Debugf("Cluster %v; deleted cluster, result: %v", clusterConfig.Name, string(stdoutStderr))

	deleteNamespaceCmd := exec.Command("kubectl", "delete", "namespace", clusterConfig.Namespace)
	deleteNamespaceCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
	stdoutStderr, err = deleteNamespaceCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Cluster %v; error on deleting namespace for cluster: %v, result: %v", clusterConfig.Name, err, string(stdoutStderr))
	}
	log.Debugf("Cluster %v; deleted namespace for cluster, result: %v", clusterConfig.Name, string(stdoutStderr))
}

func (virtualClusterManager StandardVirtualClusterManager) createWithIngressConnection(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig, shouldConnect bool, isCustomValues bool) {
	virtualClusterManager.createNamespace(benchmarkConfig, clusterConfig)
	virtualClusterManager.createIngress(benchmarkConfig, clusterConfig)

	var (
		t                *template.Template
		valuesOutputFile *os.File
		err              error
	)

	if !isCustomValues {
		t, err = template.New("vclusterValuesConfig").Parse(string(vclusterValuesConfig))
		if err != nil {
			log.Fatal(err)
		}
		data := TemplateDto{
			ClusterName:      clusterConfig.Name,
			ClusterNamespace: clusterConfig.Namespace,
			IngressDomain:    benchmarkConfig.IngressDomain,
		}
		valuesOutputFile, err = virtualClusterManager.executeTemplateToTempFile(t, data)
		defer func() {
			err = os.Remove(valuesOutputFile.Name())
			if err != nil {
				log.Error("Can't remove temp file: %s", err)
			}
		}()
	}

	createCmdArgs := []string{"create", clusterConfig.Name, "-n", clusterConfig.Namespace, "--connect=false"}
	if !isCustomValues {
		createCmdArgs = append(createCmdArgs, "-f", valuesOutputFile.Name())
	}
	createCmdArgs = append(createCmdArgs, benchmarkConfig.ClusterCreateOptions...)
	createCmd := exec.Command("vcluster", createCmdArgs...)
	createCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
	stdoutStderr, err := createCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Cluster %v; create command error: %v, create command result: %v", clusterConfig.Name, err, string(stdoutStderr))
	}
	log.Infof("Cluster %v; create command result: %v", clusterConfig.Name, string(stdoutStderr))

	if shouldConnect {
		connectCmdArgs := []string{"connect", clusterConfig.Name, "-n", clusterConfig.Namespace, "--update-current=false", fmt.Sprintf("--kube-config=%v", filepath.Join(benchmarkConfig.KubeconfigBasePath, clusterConfig.KubeConfigPath))}
		connectCmdArgs = append(connectCmdArgs, fmt.Sprintf("--server=https://%s.%s", clusterConfig.Name, benchmarkConfig.IngressDomain))
		connectCmd := exec.Command("vcluster", connectCmdArgs...)
		connectCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%v", benchmarkConfig.RootKubeConfigPath))
		stdoutStderr, err = connectCmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Cluster %v; connect command error: %v, connect command result: %v", clusterConfig.Name, err, string(stdoutStderr))
		}
		log.Infof("Cluster %v; connect command result: %v", clusterConfig.Name, string(stdoutStderr))
	}
}

func (virtualClusterManager StandardVirtualClusterManager) createNamespace(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig) {
	data := TemplateDto{
		ClusterNamespace: clusterConfig.Namespace,
	}
	err := k8s.ApplyManifestFromString(k8s.RootCluster, benchmarkConfig.RootKubeConfigPath, "namespaceConfig", string(namespaceConfig), data, k8s.MethodApply)
	if err != nil {
		log.Fatalf("Cluster %v; can't create Namespace. Error: %v", clusterConfig.Name, err)
	}
}

func (virtualClusterManager StandardVirtualClusterManager) createIngress(benchmarkConfig *config.TestConfig, clusterConfig *config.ClusterConfig) {
	data := TemplateDto{
		ClusterName:      clusterConfig.Name,
		ClusterNamespace: clusterConfig.Namespace,
		IngressDomain:    benchmarkConfig.IngressDomain,
	}
	err := k8s.ApplyManifestFromString(k8s.RootCluster, benchmarkConfig.RootKubeConfigPath, "ingressConfig", string(ingressConfig), data, k8s.MethodApply)
	if err != nil {
		log.Fatalf("Cluster %v; can't create Ingress. Error: %v", clusterConfig.Name, err)
	}
}
