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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardenercoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	seedmanagementinstall "github.com/gardener/gardener/pkg/apis/seedmanagement/install"
	settingsinstall "github.com/gardener/gardener/pkg/apis/settings/install"
	kcache "github.com/gardener/gardener/pkg/client/kubernetes/cache"
	"github.com/gardener/gardener/pkg/utils"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	// KubeConfig is the key to the kubeconfig
	KubeConfig = "kubeconfig"
	// AuthClientCertificate references the AuthInfo.ClientCertificate field of a kubeconfig
	AuthClientCertificate = "client-certificate"
	// AuthClientKey references the AuthInfo.ClientKey field of a kubeconfig
	AuthClientKey = "client-key"
	// AuthTokenFile references the AuthInfo.Tokenfile field of a kubeconfig
	AuthTokenFile = "tokenFile"
	// AuthImpersonate references the AuthInfo.Impersonate field of a kubeconfig
	AuthImpersonate = "act-as"
	// AuthProvider references the AuthInfo.AuthProvider field of a kubeconfig
	AuthProvider = "auth-provider"
	// AuthExec references the AuthInfo.Exec field of a kubeconfig
	AuthExec = "exec"
)

func init() {
	// enable protobuf for Gardener API for controller-runtime clients
	protobufSchemeBuilder := runtime.NewSchemeBuilder(
		gardenercoreinstall.AddToScheme,
		seedmanagementinstall.AddToScheme,
		settingsinstall.AddToScheme,
	)

	utilruntime.Must(apiutil.AddToProtobufScheme(protobufSchemeBuilder.AddToScheme))
}

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

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	opts := append([]ConfigFunc{WithRESTConfig(config), WithClientConfig(clientConfig)}, fns...)

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
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret); err != nil {
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

// RESTConfigFromClientConnectionConfiguration creates a *rest.Config from a componentbaseconfig.ClientConnectionConfiguration and the configured kubeconfig.
// Allowed fields are not considered unsupported if used in the kubeconfig.
func RESTConfigFromClientConnectionConfiguration(cfg *componentbaseconfig.ClientConnectionConfiguration, kubeconfig []byte, allowedFields ...string) (*rest.Config, error) {
	var (
		restConfig *rest.Config
		err        error
	)

	if kubeconfig == nil {
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.Kubeconfig},
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}},
		)

		if err := validateClientConfig(clientConfig, allowedFields); err != nil {
			return nil, err
		}

		restConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
	} else {
		restConfig, err = RESTConfigFromKubeconfig(kubeconfig, allowedFields...)
		if err != nil {
			return restConfig, err
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

// RESTConfigFromKubeconfig returns a rest.Config from the bytes of a kubeconfig.
// Allowed fields are not considered unsupported if used in the kubeconfig.
func RESTConfigFromKubeconfig(kubeconfig []byte, allowedFields ...string) (*rest.Config, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}

	if err := validateClientConfig(clientConfig, allowedFields); err != nil {
		return nil, err
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restConfig, nil
}

func validateClientConfig(clientConfig clientcmd.ClientConfig, allowedFields []string) error {
	if clientConfig == nil {
		return nil
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}
	return ValidateConfigWithAllowList(rawConfig, allowedFields)
}

// ValidateConfig validates that the auth info of a given kubeconfig doesn't have unsupported fields.
func ValidateConfig(config clientcmdapi.Config) error {
	return ValidateConfigWithAllowList(config, nil)
}

// ValidateConfigWithAllowList validates that the auth info of a given kubeconfig doesn't have unsupported fields. It takes an additional list of allowed fields.
func ValidateConfigWithAllowList(config clientcmdapi.Config, allowedFields []string) error {
	validFields := []string{"client-certificate-data", "client-key-data", "token", "username", "password"}
	validFields = append(validFields, allowedFields...)

	for user, authInfo := range config.AuthInfos {
		switch {
		case authInfo.ClientCertificate != "" && !utils.ValueExists(AuthClientCertificate, validFields):
			return fmt.Errorf("client certificate files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.ClientKey != "" && !utils.ValueExists(AuthClientKey, validFields):
			return fmt.Errorf("client key files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.TokenFile != "" && !utils.ValueExists(AuthTokenFile, validFields):
			return fmt.Errorf("token files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case (authInfo.Impersonate != "" || len(authInfo.ImpersonateGroups) > 0) && !utils.ValueExists(AuthImpersonate, validFields):
			return fmt.Errorf("impersonation is not supported, these are the valid fields: %+v", validFields)
		case (authInfo.AuthProvider != nil && len(authInfo.AuthProvider.Config) > 0) && !utils.ValueExists(AuthProvider, validFields):
			return fmt.Errorf("auth provider configurations are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.Exec != nil && !utils.ValueExists(AuthExec, validFields):
			return fmt.Errorf("exec configurations are not supported (user %q), these are the valid fields: %+v", user, validFields)
		}
	}
	return nil
}

var supportedKubernetesVersions = []string{
	"1.17",
	"1.18",
	"1.19",
	"1.20",
	"1.21",
	"1.22",
	"1.23",
	"1.24",
	"1.25",
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
	conf := &Config{}

	for _, f := range fns {
		if err := f(conf); err != nil {
			return nil, err
		}
	}

	return newClientSet(conf)
}

func newClientSet(conf *Config) (Interface, error) {
	if err := validateClientConfig(conf.clientConfig, conf.allowedUserFields); err != nil {
		return nil, err
	}

	if err := setConfigDefaults(conf); err != nil {
		return nil, err
	}

	var (
		runtimeAPIReader = conf.runtimeAPIReader
		runtimeClient    = conf.runtimeClient
		runtimeCache     = conf.runtimeCache
		err              error
	)

	if runtimeCache == nil {
		runtimeCache, err = conf.newRuntimeCache(conf.restConfig, cache.Options{
			Scheme: conf.clientOptions.Scheme,
			Mapper: conf.clientOptions.Mapper,
			Resync: conf.cacheResync,
		})
		if err != nil {
			return nil, err
		}
	}

	var uncachedClient client.Client
	if runtimeAPIReader == nil || runtimeClient == nil {
		uncachedClient, err = client.New(conf.restConfig, conf.clientOptions)
		if err != nil {
			return nil, err
		}
	}

	if runtimeAPIReader == nil {
		runtimeAPIReader = uncachedClient
	}

	if runtimeClient == nil {
		if conf.disableCache {
			runtimeClient = uncachedClient
		} else {
			delegatingClient, err := client.NewDelegatingClient(client.NewDelegatingClientInput{
				CacheReader:     runtimeCache,
				Client:          uncachedClient,
				UncachedObjects: conf.uncachedObjects,
			})
			if err != nil {
				return nil, err
			}

			runtimeClient = &FallbackClient{
				Client: delegatingClient,
				Reader: runtimeAPIReader,
			}
		}
	}

	// prepare rest config with contentType defaulted to protobuf for client-go style clients that either talk to
	// kubernetes or aggregated APIs that support protobuf.
	cfg := defaultContentTypeProtobuf(conf.restConfig)

	kubernetes, err := kubernetesclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	cs := &clientSet{
		config:     conf.restConfig,
		restClient: kubernetes.Discovery().RESTClient(),

		applier: NewApplier(runtimeClient, conf.clientOptions.Mapper),

		client:    runtimeClient,
		apiReader: runtimeAPIReader,
		cache:     runtimeCache,

		kubernetes: kubernetes,
	}

	if _, err := cs.DiscoverVersion(); err != nil {
		return nil, fmt.Errorf("error discovering kubernetes version: %w", err)
	}

	return cs, nil
}

func setConfigDefaults(conf *Config) error {
	// we can't default to protobuf ContentType here, otherwise controller-runtime clients will also try to talk to
	// CRD resources (e.g. extension CRs in the Seed) using protobuf, but CRDs don't support protobuf
	// see https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#advanced-features-and-flexibility
	if err := setClientOptionsDefaults(conf.restConfig, &conf.clientOptions); err != nil {
		return err
	}
	if conf.newRuntimeCache == nil {
		conf.newRuntimeCache = NewRuntimeCache
	}
	return nil
}

func defaultContentTypeProtobuf(c *rest.Config) *rest.Config {
	config := *c
	if config.ContentType == "" {
		config.ContentType = runtime.ContentTypeProtobuf
	}
	return &config
}

var _ client.Client = &FallbackClient{}

// FallbackClient holds a `client.Client` and a `client.Reader` which is meant as a fallback
// in case Get/List requests with the ordinary `client.Client` fail (e.g. because of cache errors).
type FallbackClient struct {
	client.Client
	Reader client.Reader
}

var cacheError = &kcache.CacheError{}

// Get retrieves an obj for a given object key from the Kubernetes Cluster.
// In case of a cache error, the underlying API reader is used to execute the request again.
func (d *FallbackClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := d.Client.Get(ctx, key, obj)
	if err != nil && errors.As(err, &cacheError) {
		logf.Log.V(1).Info("Falling back to API reader because a cache error occurred", "error", err)
		return d.Reader.Get(ctx, key, obj)
	}
	return err
}

// List retrieves list of objects for a given namespace and list options.
// In case of a cache error, the underlying API reader is used to execute the request again.
func (d *FallbackClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := d.Client.List(ctx, list, opts...)
	if err != nil && errors.As(err, &cacheError) {
		logf.Log.V(1).Info("Falling back to API reader because a cache error occurred", "error", err)
		return d.Reader.List(ctx, list, opts...)
	}
	return err
}
