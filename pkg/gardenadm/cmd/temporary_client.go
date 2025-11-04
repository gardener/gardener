// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// NewClientSetFromBootstrapToken returns a Kubernetes client set based on the provided  bootstrap token.
func NewClientSetFromBootstrapToken(controlPlaneAddress string, certificateAuthority []byte, bootstrapToken string, scheme *runtime.Scheme) (kubernetes.Interface, error) {
	return kubernetes.NewWithConfig(kubernetes.WithRESTConfig(&rest.Config{
		Host:            controlPlaneAddress,
		TLSClientConfig: rest.TLSClientConfig{CAData: certificateAuthority},
		BearerToken:     bootstrapToken,
	}), kubernetes.WithClientOptions(client.Options{Scheme: scheme}), kubernetes.WithDisabledCachedClient())
}
