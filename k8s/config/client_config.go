/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	contentType = "application/vnd.kubernetes.protobuf"
	qps         = 100
	burst       = 200
)

// PrepareConfig creates and initializes clients config.
func PrepareConfig(path string) (config *restclient.Config, err error) {
	config, err = GetConfig(path)
	if err != nil {
		return nil, err
	}
	// TODO: check if transportHack from original function is needed
	return config, nil
}

// GetConfig returns clients config without additional initialization.
func GetConfig(path string) (config *restclient.Config, err error) {
	if path != "" {
		config, err = loadConfig(path)
	} else {
		config, err = restclient.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	initializeWithDefaults(config)
	return
}

func restClientConfig(path string) (*clientcmdapi.Config, error) {
	c, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %v", err)
	}
	return c, nil
}

func loadConfig(path string) (*restclient.Config, error) {
	c, err := restClientConfig(path)
	if err != nil {
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(*c, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func initializeWithDefaults(config *restclient.Config) {
	config.ContentType = contentType
	config.QPS = qps
	config.Burst = burst
}
