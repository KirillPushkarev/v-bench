package monitoring

import (
	"bytes"
	"embed"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"k8s.io/apimachinery/pkg/util/wait"
	"os/exec"
	"text/template"
	"time"
	"v-bench/internal/util"
	"v-bench/k8s"
	"v-bench/measurement"
	"v-bench/prometheus/clients"
)

const (
	rbacManifestsPattern                  = "kube-prometheus-configs/templates/rbac/*.yaml"
	monitoringManifestsPattern            = "kube-prometheus-configs/templates/k8s/*.yaml"
	numK8sClients                         = 1
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
	executor *measurement.PrometheusQueryExecutor
}

func NewPrometheusProvisioner(kubeConfigPath string) (*PrometheusProvisioner, error) {
	prometheusFramework, err := k8s.NewFramework(kubeConfigPath, numK8sClients)
	if err != nil {
		return nil, fmt.Errorf("k8s framework creation error: %v", err)
	}
	pc := clients.NewInClusterPrometheusClient(prometheusFramework.GetClientSets().GetClient())
	executor := measurement.NewPrometheusQueryExecutor(pc)

	return &PrometheusProvisioner{executor: executor}, nil
}

func (receiver PrometheusProvisioner) Provision(dto *ProvisionerTemplateDto) error {
	log.Infof("Applying prometheus manifests")

	rbacManifests, err := fs.Glob(manifestsFs, rbacManifestsPattern)
	if err != nil {
		return err
	}
	monitoringManifests, err := fs.Glob(manifestsFs, monitoringManifestsPattern)
	if err != nil {
		return err
	}

	for _, manifest := range rbacManifests {
		err := receiver.applyManifest(dto, manifest)
		if err != nil {
			return err
		}
	}

	for _, manifest := range monitoringManifests {
		err := receiver.applyManifest(dto, manifest)
		if err != nil {
			return err
		}
	}

	err = receiver.applyEtcdServicePatch(dto)
	if err != nil {
		return err
	}

	err = receiver.waitForPrometheusToBeHealthy(dto)
	if err != nil {
		return err
	}

	log.Infof("Finished applying prometheus manifests")

	return nil
}

func (receiver PrometheusProvisioner) applyManifest(dto *ProvisionerTemplateDto, manifest string) error {
	log.Debugf("Applying prometheus manifest: %s", manifest)

	t, err := template.ParseFS(manifestsFs, manifest)
	if err != nil {
		return err
	}
	buffer := new(bytes.Buffer)
	err = t.Execute(buffer, dto)
	if err != nil {
		return err
	}

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = buffer
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	log.Debugf("Finished applying prometheus manifest: %s", manifest)
	log.Debugf("Result:")
	log.Debugf(string(out))

	return nil
}

func (receiver PrometheusProvisioner) applyEtcdServicePatch(dto *ProvisionerTemplateDto) error {
	log.Debugf("Applying prometheus manifest: %s", "kube-prometheus-configs/templates/k8s/patches/etcd-service-patch.yaml")

	cmd := exec.Command("kubectl", "patch", "service", "-n", dto.ClusterNamespace, fmt.Sprintf("%v-etcd", dto.ClusterName), "--patch", string(etcdServicePatch))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	log.Debugf("Finished applying prometheus manifest: %s", "kube-prometheus-configs/templates/k8s/patches/etcd-service-patch.yaml")
	log.Debugf(string(out))

	return nil
}

func (receiver PrometheusProvisioner) waitForPrometheusToBeHealthy(dto *ProvisionerTemplateDto) error {
	log.Info("Waiting for Prometheus stack to become healthy...")
	return wait.PollImmediate(
		checkPrometheusReadyIntervalInSeconds*time.Second,
		checkPrometheusReadyTimeoutInSeconds*time.Second,
		func() (bool, error) { return receiver.isPrometheusReady(dto) },
	)
}

// isPrometheusReady Checks that targets for control plane are ready.
func (receiver PrometheusProvisioner) isPrometheusReady(dto *ProvisionerTemplateDto) (bool, error) {
	expectedScrapePools := []string{
		fmt.Sprintf("serviceMonitor/%v/kube-apiserver/0", dto.ClusterNamespace),
		fmt.Sprintf("serviceMonitor/%v/kube-controller-manager/0", dto.ClusterNamespace),
		fmt.Sprintf("serviceMonitor/%v/etcd/0", dto.ClusterNamespace),
	}
	activeTargets, err := receiver.executor.Targets(map[string]string{"state": "active"})
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
