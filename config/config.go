package config

import (
	"encoding/json"
	"fmt"
	"v-bench/internal/util"
)

type ClusterConfig struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	KubeConfigPath string `json:"kubeconfig"`
}

type ClusterType string

const (
	ClusterTypeHost    ClusterType = "host"
	ClusterTypeVirtual ClusterType = "virtual"
)

type VirtualClusterConnType string

const (
	VirtualClusterConnTypeDirect  VirtualClusterConnType = "direct"
	VirtualClusterConnTypeIngress VirtualClusterConnType = "ingress"
)

type PrometheusConnType string

const (
	PrometheusConnTypeDirect PrometheusConnType = "direct"
	PrometheusConnTypeProxy  PrometheusConnType = "proxy"
)

type TestConfig struct {
	// User supplied fields
	ConfigPath           string          `json:"config_path"`
	Name                 string          `json:"name"`
	ClusterType          ClusterType     `json:"cluster_type"`
	RootKubeConfigPath   string          `json:"root_kubeconfig_path"`
	KubeconfigBasePath   string          `json:"kubeconfig_base_path"`
	ClusterConfigs       []ClusterConfig `json:"clusters"`
	ClusterCreateOptions []string        `json:"cluster_create_options"`
	InitialResources     struct {
		ConfigMap int `json:"configmap"`
	} `json:"initial_resources"`
	TestConfigName            string                 `json:"test_config"`
	MetaInfoPath              string                 `json:"meta_info_path"`
	ShouldProvisionMonitoring bool                   `json:"should_provision_monitoring"`
	PrometheusConnType        PrometheusConnType     `json:"prometheus_conn_type"`
	VirtualClusterConnType    VirtualClusterConnType `json:"vcluster_conn_type"`
	IngressDomain             string                 `json:"ingress_domain"`

	// Other fields
	PathExpander util.PathExpander
}

func NewDefaultTestConfig(configPath string, expander util.PathExpander) *TestConfig {
	return &TestConfig{
		ConfigPath:                configPath,
		RootKubeConfigPath:        "~/.kube/config",
		ShouldProvisionMonitoring: true,
		PrometheusConnType:        PrometheusConnTypeProxy,
		VirtualClusterConnType:    VirtualClusterConnTypeDirect,
		PathExpander:              expander,
	}
}

func (testConfig *TestConfig) UnmarshalJSON(data []byte) error {
	type alias *TestConfig
	testConfigTmp := alias(testConfig)
	err := json.Unmarshal(data, testConfigTmp)
	if err != nil {
		return err
	}

	testConfig.expandPaths()

	for i := range testConfig.ClusterConfigs {
		clusterConfig := &testConfig.ClusterConfigs[i]
		if clusterConfig.Namespace == "" {
			clusterConfig.Namespace = fmt.Sprintf("vcluster-%v", clusterConfig.Name)
		}
	}

	return nil
}

func (testConfig *TestConfig) expandPaths() {
	pathExpander := testConfig.PathExpander
	testConfig.ConfigPath = pathExpander.ExpandPath(testConfig.ConfigPath)
	testConfig.RootKubeConfigPath = pathExpander.ExpandPath(testConfig.RootKubeConfigPath)
	testConfig.KubeconfigBasePath = pathExpander.ExpandPath(testConfig.KubeconfigBasePath)
	for _, clusterConfig := range testConfig.ClusterConfigs {
		clusterConfig.KubeConfigPath = pathExpander.ExpandPath(clusterConfig.KubeConfigPath)
	}
	testConfig.MetaInfoPath = pathExpander.ExpandPath(testConfig.MetaInfoPath)
}
