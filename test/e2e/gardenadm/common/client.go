// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"os"

	. "github.com/onsi/gomega"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ContainerName is the name of the node container of the machine pod.
const ContainerName = "node"

// RuntimeClient is the client for runtime cluster.
var RuntimeClient kubernetes.Interface

// SetupRuntimeClient initializes the runtime client.
func SetupRuntimeClient() {
	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&componentbaseconfig.ClientConnectionConfiguration{Kubeconfig: os.Getenv("KUBECONFIG")}, nil, kubernetes.AuthTokenFile)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	RuntimeClient, err = kubernetes.NewWithConfig(kubernetes.WithRESTConfig(restConfig), kubernetes.WithDisabledCachedClient())
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}
