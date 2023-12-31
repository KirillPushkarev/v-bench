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

package k8s

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	// ensure auth plugins are loaded
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Framework allows for interacting with Kubernetes cluster via official Kubernetes clients.
type Framework struct {
	clients        *MultiClientSet
	dynamicClients *MultiDynamicClient
}

func NewFramework(kubeConfigPath string, clientsNumber int) (*Framework, error) {
	log.Debugf("Creating framework with %d clients and %q kubeconfig.", clientsNumber, kubeConfigPath)
	var err error
	f := Framework{}
	if f.clients, err = NewMultiClientSet(kubeConfigPath, clientsNumber); err != nil {
		return nil, fmt.Errorf("clients creation error: %v", err)
	}
	if f.dynamicClients, err = NewMultiDynamicClient(kubeConfigPath, clientsNumber); err != nil {
		return nil, fmt.Errorf("dynamic clients creation error: %v", err)
	}

	return &f, nil
}

func (f *Framework) GetClients() *MultiClientSet {
	return f.clients
}

func (f *Framework) GetDynamicClients() *MultiDynamicClient {
	return f.dynamicClients
}
