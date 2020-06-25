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
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	componentbaseconfig "k8s.io/component-base/config"
	apiserviceclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// UseCachedRuntimeClients is a flag for enabling cached controller-runtime clients (defaults to false).
	// If enabled, the client returned by Interface.Client() will be backed by a cache, otherwise it will be the same
	// client that will be returned by Interface.DirectClient().
	UseCachedRuntimeClients = false
)

// KubeConfig is the key to the kubeconfig
const KubeConfig = "kubeconfig"

// NewClientFromFile creates a new Client struct for a given kubeconfig. The kubeconfig will be
// read from the filesystem at location <kubeconfigPath>. If given, <masterURL> overrides the
// master URL in the kubeconfig.
// If no filepath is given, the in-cluster configuration will be taken into account.
func NewClientFromFile(masterURL, kubeconfigPath string, fns ...ConfigFunc) (Interface, error) {
	if kubeconfigPath == "" && masterURL == "" {
		kubeconfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		opts := append([]ConfigFunc{WithRESTConfig(kubeconfig)}, fns...)
		return NewWithConfig(opts...)
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: masterURL}},
	)

	if err := validateClientConfig(clientConfig); err != nil {
		return nil, err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	opts := append([]ConfigFunc{WithRESTConfig(config)}, fns...)
	return NewWithConfig(opts...)
}

// NewClientFromBytes creates a new Client struct for a given kubeconfig byte slice.
func NewClientFromBytes(kubeconfig []byte, fns ...ConfigFunc) (Interface, error) {
	config, err := RESTConfigFromClientConnectionConfiguration(nil, kubeconfig)
	if err != nil {
		return nil, err
	}

	opts := append([]ConfigFunc{WithRESTConfig(config)}, fns...)
	return NewWithConfig(opts...)
}

// NewClientFromSecret creates a new Client struct for a given kubeconfig stored as a
// Secret in an existing Kubernetes cluster. This cluster will be accessed by the <k8sClient>. It will
// read the Secret <secretName> in <namespace>. The Secret must contain a field "kubeconfig" which will
// be used.
func NewClientFromSecret(ctx context.Context, c client.Client, namespace, secretName string, fns ...ConfigFunc) (Interface, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(namespace, secretName), secret); err != nil {
		return nil, err
	}
	return NewClientFromSecretObject(secret, fns...)
}

// NewClientFromSecretObject creates a new Client struct for a given Kubernetes Secret object. The Secret must
// contain a field "kubeconfig" which will be used.
func NewClientFromSecretObject(secret *corev1.Secret, fns ...ConfigFunc) (Interface, error) {
	if kubeconfig, ok := secret.Data[KubeConfig]; ok {
		if len(kubeconfig) == 0 {
			return nil, errors.New("the secret's field 'kubeconfig' is empty")
		}

		return NewClientFromBytes(kubeconfig, fns...)
	}
	return nil, errors.New("the secret does not contain a field with name 'kubeconfig'")
}

// RESTConfigFromClientConnectionConfiguration creates a *rest.Config from a componentbaseconfig.ClientConnectionConfiguration & the configured kubeconfig
func RESTConfigFromClientConnectionConfiguration(cfg *componentbaseconfig.ClientConnectionConfiguration, kubeconfig []byte) (*rest.Config, error) {
	var (
		restConfig *rest.Config
		err        error
	)

	if kubeconfig == nil {
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.Kubeconfig},
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}},
		)

		if err := validateClientConfig(clientConfig); err != nil {
			return nil, err
		}

		restConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
	} else {
		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
		if err != nil {
			return nil, err
		}

		if err := validateClientConfig(clientConfig); err != nil {
			return nil, err
		}

		restConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	if cfg != nil {
		restConfig.Burst = int(cfg.Burst)
		restConfig.QPS = cfg.QPS
		restConfig.AcceptContentTypes = cfg.AcceptContentTypes
		restConfig.ContentType = cfg.ContentType
	}

	return restConfig, nil
}

func validateClientConfig(clientConfig clientcmd.ClientConfig) error {
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}
	return ValidateConfig(rawConfig)
}

// ValidateConfig validates that the auth info of a given kubeconfig doesn't have unsupported fields.
func ValidateConfig(config clientcmdapi.Config) error {
	validFields := []string{"client-certificate-data", "client-key-data", "token", "username", "password"}

	for user, authInfo := range config.AuthInfos {
		switch {
		case authInfo.ClientCertificate != "":
			return fmt.Errorf("client certificate files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.ClientKey != "":
			return fmt.Errorf("client key files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.TokenFile != "":
			return fmt.Errorf("token files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.Impersonate != "" || len(authInfo.ImpersonateGroups) > 0:
			return fmt.Errorf("impersonation is not supported, these are the valid fields: %+v", validFields)
		case authInfo.AuthProvider != nil && len(authInfo.AuthProvider.Config) > 0:
			return fmt.Errorf("auth provider configurations are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.Exec != nil:
			return fmt.Errorf("exec configurations are not supported (user %q), these are the valid fields: %+v", user, validFields)
		}
	}

	return nil
}

var supportedKubernetesVersions = []string{
	"1.10",
	"1.11",
	"1.12",
	"1.13",
	"1.14",
	"1.15",
	"1.16",
	"1.17",
	"1.18",
}

func checkIfSupportedKubernetesVersion(gitVersion string) error {
	for _, supportedVersion := range supportedKubernetesVersions {
		ok, err := versionutils.CompareVersions(gitVersion, "~", supportedVersion)
		if err != nil {
			return err
		}

		if ok {
			return nil
		}
	}
	return fmt.Errorf("unsupported kubernetes version %q", gitVersion)
}

// NewWithConfig returns a new Kubernetes base client.
func NewWithConfig(fns ...ConfigFunc) (Interface, error) {
	conf := &config{}

	for _, f := range fns {
		if err := f(conf); err != nil {
			return nil, err
		}
	}

	return newClientSet(conf)
}

func newClientSet(conf *config) (Interface, error) {
	if err := setConfigDefaults(conf); err != nil {
		return nil, err
	}

	runtimeCache, err := NewRuntimeCache(conf.restConfig, cache.Options{
		Scheme: conf.clientOptions.Scheme,
		Mapper: conf.clientOptions.Mapper,
		Resync: conf.cacheResync,
	})
	if err != nil {
		return nil, err
	}

	directClient, err := client.New(conf.restConfig, conf.clientOptions)
	if err != nil {
		return nil, err
	}

	var runtimeClient client.Client
	if UseCachedRuntimeClients {
		runtimeClient, err = newRuntimeClientWithCache(conf.restConfig, conf.clientOptions, runtimeCache)
		if err != nil {
			return nil, err
		}
	} else {
		runtimeClient = directClient
	}

	kubernetes, err := kubernetesclientset.NewForConfig(conf.restConfig)
	if err != nil {
		return nil, err
	}

	gardenCore, err := gardencoreclientset.NewForConfig(conf.restConfig)
	if err != nil {
		return nil, err
	}

	apiRegistration, err := apiserviceclientset.NewForConfig(conf.restConfig)
	if err != nil {
		return nil, err
	}

	apiExtension, err := apiextensionsclientset.NewForConfig(conf.restConfig)
	if err != nil {
		return nil, err
	}

	serverVersion, err := kubernetes.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	if err := checkIfSupportedKubernetesVersion(serverVersion.GitVersion); err != nil {
		return nil, err
	}

	applier := NewApplier(runtimeClient, conf.clientOptions.Mapper)
	chartRenderer := chartrenderer.NewWithServerVersion(serverVersion)
	chartApplier := NewChartApplier(chartRenderer, applier)

	if err := checkIfSupportedKubernetesVersion(serverVersion.GitVersion); err != nil {
		return nil, err
	}

	cs := &clientSet{
		config:     conf.restConfig,
		restMapper: conf.clientOptions.Mapper,
		restClient: kubernetes.Discovery().RESTClient(),

		applier:       applier,
		chartRenderer: chartRenderer,
		chartApplier:  chartApplier,

		client:       runtimeClient,
		directClient: directClient,
		cache:        runtimeCache,

		kubernetes:      kubernetes,
		gardenCore:      gardenCore,
		apiregistration: apiRegistration,
		apiextension:    apiExtension,

		version: serverVersion.GitVersion,
	}

	return cs, nil
}

func setConfigDefaults(conf *config) error {
	return setClientOptionsDefaults(conf.restConfig, &conf.clientOptions)
}
