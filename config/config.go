package config

type ClusterConfig struct {
	Name           string `json:"name"`
	KubeConfigPath string `json:"kubeconfig"`
}

type ClusterType string

const (
	HostCluster    ClusterType = "host"
	VirtualCluster ClusterType = "host"
)

type TestConfig struct {
	ConfigPath           string
	Name                 string          `json:"name"`
	ClusterType          ClusterType     `json:"cluster_type"`
	RootKubeConfigPath   string          `json:"root_kubeconfig_path"`
	KubeconfigBasePath   string          `json:"kubeconfig_base_path"`
	RunsBasePath         string          `json:"runs_base_path"`
	ClusterConfigs       []ClusterConfig `json:"clusters"`
	ClusterCreateOptions []string        `json:"cluster_create_options"`
	InitialResources     struct {
		ConfigMap int `json:"configmap"`
	} `json:"initial_resources"`
	TestConfigName string `json:"test_config"`
	MetaInfoPath   string `json:"meta_info_file"`
}
