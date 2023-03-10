package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"v-bench/internal/util"
)

func TestTestConfig_UnmarshalJSON(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name       string
		testConfig *TestConfig
		args       args
		want       *TestConfig
	}{
		{
			name:       "Test case #1",
			testConfig: &TestConfig{ConfigPath: "./config.json", RootKubeConfigPath: "~/.kube/config", ShouldProvisionMonitoring: true, PathExpander: &util.NoOpPathExpander{}},
			args: args{data: []byte(
				`{
				  "name": "v.1.e.s.12.0",
				  "cluster_type": "virtual",
				  "root_kubeconfig_path": "~/.kube/config",
				  "kubeconfig_base_path": "./k8s-configs/vcluster/local",
				  "clusters": [
					{
					  "name": "dropapp-1",
					  "kubeconfig": "dropapp-1.yaml"
					}
				  ],
				  "cluster_create_options": [
					"--distro",
					"k8s"
				  ],
				  "initial_resources": {
					"configmap": 0
				  },
				  "test_config": "./k-bench-test-configs/cp_heavy_12client_test",
				  "meta_info_file": "./system-info/local_system_info.yaml"
				}`),
			},
			want: &TestConfig{
				ConfigPath:           "./config.json",
				Name:                 "v.1.e.s.12.0",
				ClusterType:          "virtual",
				RootKubeConfigPath:   "~/.kube/config",
				KubeconfigBasePath:   "./k8s-configs/vcluster/local",
				ClusterConfigs:       []ClusterConfig{{Name: "dropapp-1", Namespace: "vcluster-dropapp-1", KubeConfigPath: "dropapp-1.yaml"}},
				ClusterCreateOptions: []string{"--distro", "k8s"},
				InitialResources: struct {
					ConfigMap int `json:"configmap"`
				}{ConfigMap: 0},
				TestConfigName:            "./k-bench-test-configs/cp_heavy_12client_test",
				MetaInfoPath:              "./system-info/local_system_info.yaml",
				ShouldProvisionMonitoring: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.testConfig.UnmarshalJSON(test.args.data)

			assert.Nil(t, err)

			test.testConfig.PathExpander = nil
			assert.Equal(t, test.want, test.testConfig)
		})
	}
}
