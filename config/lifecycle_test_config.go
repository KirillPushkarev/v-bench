package config

import (
	"encoding/json"
	"v-bench/internal/util"
)

type LifecycleTestConfig struct {
	// User supplied fields
	ConfigPath           string   `json:"config_path"`
	Name                 string   `json:"name"`
	RootKubeConfigPath   string   `json:"root_kubeconfig_path"`
	ClusterCreateOptions []string `json:"cluster_create_options"`
	MetaInfoPath         string   `json:"meta_info_path"`
	Parallelism          int      `json:"parallelism"`
	Count                int      `json:"count"`

	// Other fields
	PathExpander util.PathExpander
}

func NewDefaultLifecycleTestConfig(configPath string, expander util.PathExpander) *LifecycleTestConfig {
	return &LifecycleTestConfig{
		ConfigPath:         configPath,
		RootKubeConfigPath: "~/.kube/config",
		PathExpander:       expander,
	}
}

func (testConfig *LifecycleTestConfig) UnmarshalJSON(data []byte) error {
	type alias *LifecycleTestConfig
	testConfigTmp := alias(testConfig)
	err := json.Unmarshal(data, testConfigTmp)
	if err != nil {
		return err
	}

	testConfig.expandPaths()

	return nil
}

func (testConfig *LifecycleTestConfig) expandPaths() {
	pathExpander := testConfig.PathExpander
	testConfig.ConfigPath = pathExpander.ExpandPath(testConfig.ConfigPath)
	testConfig.RootKubeConfigPath = pathExpander.ExpandPath(testConfig.RootKubeConfigPath)
	testConfig.MetaInfoPath = pathExpander.ExpandPath(testConfig.MetaInfoPath)
}
