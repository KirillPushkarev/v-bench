package monitoring

import (
	"embed"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"k8s.io/apimachinery/pkg/util/wait"
	"os/exec"
	"time"
	"v-bench/internal/util"
	"v-bench/k8s"
	"v-bench/measurement"
)

const (
	rbacManifestsPattern                  = "kube-prometheus-configs/templates/rbac/*.yaml"
	monitoringManifestsPattern            = "kube-prometheus-configs/templates/k8s/*.yaml"
	checkPrometheusReadyIntervalInSeconds = 30
	checkPrometheusReadyTimeoutInSeconds  = 300
)

var (
	//go:embed kube-prometheus-configs/templates
	manifestsFs embed.FS
	//go:embed kube-prometheus-configs/templates/k8s/patches/etcd-service-patch.yaml
	etcdServicePatch []byte
)

type ProvisionerTemplateDto struct {
	ClusterName      string
	ClusterNamespace string
}

func NewProvisionerTemplateDto(clusterName string, clusterNamespace string) *ProvisionerTemplateDto {
	return &ProvisionerTemplateDto{ClusterName: clusterName, ClusterNamespace: clusterNamespace}
}

type PrometheusProvisioner struct {
	prometheusQueryExecutor *measurement.PrometheusQueryExecutor
}

func NewPrometheusProvisioner(prometheusQueryExecutor *measurement.PrometheusQueryExecutor) *PrometheusProvisioner {
	return &PrometheusProvisioner{prometheusQueryExecutor: prometheusQueryExecutor}
}

func (provisioner PrometheusProvisioner) Provision(kubeconfigPath string, dto *ProvisionerTemplateDto) error {
	log.Infof("Cluster %v; applying prometheus manifests", dto.ClusterName)

	rbacManifests, err := fs.Glob(manifestsFs, rbacManifestsPattern)
	if err != nil {
		return err
	}
	monitoringManifests, err := fs.Glob(manifestsFs, monitoringManifestsPattern)
	if err != nil {
		return err
	}

	for _, manifest := range rbacManifests {
		err := k8s.ApplyManifestFromEmbeddedFile(k8s.RootCluster, kubeconfigPath, manifestsFs, manifest, dto, k8s.MethodApply)
		if err != nil {
			return err
		}
	}

	for _, manifest := range monitoringManifests {
		err := k8s.ApplyManifestFromEmbeddedFile(k8s.RootCluster, kubeconfigPath, manifestsFs, manifest, dto, k8s.MethodApply)
		if err != nil {
			return err
		}
	}

	provisioner.applyEtcdServicePatch(dto)

	err = provisioner.waitForPrometheusToBeHealthy(dto)
	if err != nil {
		return err
	}

	log.Infof("Cluster %v; finished applying prometheus manifests", dto.ClusterName)

	return nil
}

func (provisioner PrometheusProvisioner) applyEtcdServicePatch(dto *ProvisionerTemplateDto) {
	log.Debugf("Cluster %v; applying prometheus manifest: %s", dto.ClusterName, "kube-prometheus-configs/templates/k8s/patches/etcd-service-patch.yaml")

	cmd := exec.Command("kubectl", "patch", "service", "-n", dto.ClusterNamespace, fmt.Sprintf("%v-etcd", dto.ClusterName), "--patch", string(etcdServicePatch))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Cluster %v; error on applying manifest: %v, command result: %v", dto.ClusterName, err, string(stdoutStderr))
	}

	log.Debugf("Cluster %v; finished applying prometheus manifest: %s", dto.ClusterName, "kube-prometheus-configs/templates/k8s/patches/etcd-service-patch.yaml")
}

func (provisioner PrometheusProvisioner) waitForPrometheusToBeHealthy(dto *ProvisionerTemplateDto) error {
	log.Infof("Cluster %v; waiting for Prometheus stack to become healthy...", dto.ClusterName)
	return wait.PollImmediate(
		checkPrometheusReadyIntervalInSeconds*time.Second,
		checkPrometheusReadyTimeoutInSeconds*time.Second,
		func() (bool, error) { return provisioner.isPrometheusReady(dto) },
	)
}

// isPrometheusReady Checks that targets for control plane are ready.
func (provisioner PrometheusProvisioner) isPrometheusReady(dto *ProvisionerTemplateDto) (bool, error) {
	expectedScrapePools := []string{
		fmt.Sprintf("serviceMonitor/%v/kube-apiserver/0", dto.ClusterNamespace),
		fmt.Sprintf("serviceMonitor/%v/kube-controller-manager/0", dto.ClusterNamespace),
		fmt.Sprintf("serviceMonitor/%v/etcd/0", dto.ClusterNamespace),
	}
	activeTargets, err := provisioner.prometheusQueryExecutor.Targets("active")
	if err != nil {
		return false, err
	}
	readyTargetsCount := 0
	for _, target := range activeTargets {
		if util.Contains(expectedScrapePools, target.ScrapePool) {
			readyTargetsCount++
		}
	}

	return len(expectedScrapePools) == readyTargetsCount, nil
}
