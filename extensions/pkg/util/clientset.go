// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	componentbaseconfig "k8s.io/component-base/config"
)

// NewRESTConfigFromKubeconfig creates a new REST config from a given Kubeconfig and returns it.
func NewRESTConfigFromKubeconfig(kubeconfig []byte) (*rest.Config, error) {
	configObj, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*configObj, &clientcmd.ConfigOverrides{})

	return createRESTConfig(clientConfig, nil)
}

// ApplyClientConnectionConfigurationToRESTConfig applies the given client connection configurations to the given
// REST config.
func ApplyClientConnectionConfigurationToRESTConfig(clientConnection *componentbaseconfig.ClientConnectionConfiguration, rest *rest.Config) {
	if clientConnection == nil {
		return
	}

	rest.AcceptContentTypes = clientConnection.AcceptContentTypes
	rest.ContentType = clientConnection.ContentType
	rest.Burst = int(clientConnection.Burst)
	rest.QPS = clientConnection.QPS
}

// createRESTConfig creates a Config object for a rest client. If a clientConnection configuration object is passed
// as well then the specified fields will be taken over as well.
func createRESTConfig(clientConfig clientcmd.ClientConfig, clientConnection *componentbaseconfig.ClientConnectionConfiguration) (*rest.Config, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	if clientConnection != nil {
		config.Burst = int(clientConnection.Burst)
		config.QPS = clientConnection.QPS
		config.AcceptContentTypes = clientConnection.AcceptContentTypes
		config.ContentType = clientConnection.ContentType
	}

	return config, nil
}
