// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"errors"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes/base"
	"github.com/gardener/gardener/pkg/client/kubernetes/v110"
	"github.com/gardener/gardener/pkg/client/kubernetes/v111"
	"github.com/gardener/gardener/pkg/client/kubernetes/v112"
	"github.com/gardener/gardener/pkg/client/kubernetes/v19"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClientFromFile creates a new Client struct for a given kubeconfig. The kubeconfig will be
// read from the filesystem at location <kubeconfigPath>.
// If no filepath is given, the in-cluster configuration will be taken into account.
func NewClientFromFile(kubeconfigPath string) (Client, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{},
	)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return newKubernetesClient(config)
}

// NewClientFromBytes creates a new Client struct for a given kubeconfig byte slice.
func NewClientFromBytes(kubeconfig []byte) (Client, error) {
	configObj, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*configObj, &clientcmd.ConfigOverrides{})
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return newKubernetesClient(config)
}

// NewClientFromSecret creates a new Client struct for a given kubeconfig stored as a
// Secret in an existing Kubernetes cluster. This cluster will be accessed by the <k8sClient>. It will
// read the Secret <secretName> in <namespace>. The Secret must contain a field "kubeconfig" which will
// be used.
func NewClientFromSecret(k8sClient Client, namespace, secretName string) (Client, error) {
	secret, err := k8sClient.GetSecret(namespace, secretName)
	if err != nil {
		return nil, err
	}
	return NewClientFromSecretObject(secret)
}

// NewClientFromSecretObject creates a new Client struct for a given Kubernetes Secret object. The Secret must
// contain a field "kubeconfig" which will be used.
func NewClientFromSecretObject(secret *corev1.Secret) (Client, error) {
	if kubeconfig, ok := secret.Data["kubeconfig"]; ok {
		return NewClientFromBytes(kubeconfig)
	}
	return nil, errors.New("The secret does not contain a field with name 'kubeconfig'")
}

// newKubernetesClient takes a REST config and returns a <Client>
// struct which implements convenience methods for creating, listing, updating or deleting resources.
func newKubernetesClient(config *rest.Config) (Client, error) {
	var k8sClient Client

	baseClient, err := kubernetesbase.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	gitVersion := baseClient.Version()

	k8s19, err := utils.CompareVersions(gitVersion, "~", "1.9")
	if err != nil {
		return nil, err
	}
	k8s110, err := utils.CompareVersions(gitVersion, "~", "1.10")
	if err != nil {
		return nil, err
	}
	k8s111, err := utils.CompareVersions(gitVersion, "~", "1.11")
	if err != nil {
		return nil, err
	}
	k8s112, err := utils.CompareVersions(gitVersion, "~", "1.12")
	if err != nil {
		return nil, err
	}

	switch {
	case k8s19:
		k8sClient = kubernetesv19.NewFrom(baseClient)
	case k8s110:
		k8sClient = kubernetesv110.NewFromBase(baseClient)
	case k8s111:
		k8sClient = kubernetesv111.NewFromBase(baseClient)
	case k8s112:
		k8sClient = kubernetesv112.NewFromBase(baseClient)
	default:
		return nil, fmt.Errorf("Kubernetes cluster has version %s which is not supported", gitVersion)
	}

	if err := k8sClient.DiscoverAPIGroups(); err != nil {
		if len(k8sClient.GetAPIResourceList()) == 0 {
			return nil, err
		}
		logger.Logger.Debugf("Got a non-empty API resource list during bootstrapping of a Kubernetes client, but also an error: %s", err.Error())
	}

	return k8sClient, nil
}
