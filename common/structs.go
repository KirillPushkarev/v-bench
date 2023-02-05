package common

type ClusterConfig struct {
	Name           string `json:"name"`
	KubeConfigPath string `json:"kubeconfig"`
}

type TestConfig struct {
	ClusterType          string          `json:"cluster_type"`
	KubeconfigBasePath   string          `json:"kubeconfig_base_path"`
	RunsBasePath         string          `json:"runs_base_path"`
	ClusterConfigs       []ClusterConfig `json:"clusters"`
	ClusterCreateOptions string          `json:"cluster_create_options"`
	InitialResources     struct {
		ConfigMap int `json:"configmap"`
	} `json:"initial_resources"`
	TestConfigName string `json:"test_config"`
	MetaInfoPath   string `json:"meta_info_file"`
}
