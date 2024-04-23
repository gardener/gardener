// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
