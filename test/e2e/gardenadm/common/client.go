// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"os"

	. "github.com/onsi/gomega"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ContainerName is the name of the node container of the machine pod.
const ContainerName = "node"

// RuntimeClient is the client for runtime cluster.
var RuntimeClient kubernetes.Interface

// SetupRuntimeClient initializes the runtime client.
func SetupRuntimeClient() {
	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&componentbaseconfigv1alpha1.ClientConnectionConfiguration{Kubeconfig: os.Getenv("KUBECONFIG")}, nil, kubernetes.AuthTokenFile)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	RuntimeClient, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(restConfig),
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	komega.SetClient(RuntimeClient.Client())
}
