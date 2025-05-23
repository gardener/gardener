// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	securityinstall "github.com/gardener/gardener/pkg/apis/security/install"
	seedmanagementinstall "github.com/gardener/gardener/pkg/apis/seedmanagement/install"
	settingsinstall "github.com/gardener/gardener/pkg/apis/settings/install"
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
		gardencoreinstall.AddToScheme,
		seedmanagementinstall.AddToScheme,
		settingsinstall.AddToScheme,
		securityinstall.AddToScheme,
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
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	opts := append([]ConfigFunc{WithRESTConfig(restConfig)}, fns...)
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

// RESTConfigFromClientConnectionConfiguration creates a *rest.Config from a componentbaseconfigv1alpha1.ClientConnectionConfiguration and the configured kubeconfig.
// It takes an optional list of additionally allowed kubeconfig fields.
func RESTConfigFromClientConnectionConfiguration(cfg *componentbaseconfigv1alpha1.ClientConnectionConfiguration, kubeconfig []byte, allowedFields ...string) (*rest.Config, error) {
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

// RESTConfigFromKubeconfigFile returns a rest.Config from the bytes of a kubeconfig file.
// It takes an optional list of additionally allowed kubeconfig fields.
func RESTConfigFromKubeconfigFile(kubeconfigFile string, allowedFields ...string) (*rest.Config, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigFile},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}},
	)

	if err := validateClientConfig(clientConfig, allowedFields); err != nil {
		return nil, err
	}

	return clientConfig.ClientConfig()
}

// RESTConfigFromKubeconfig returns a rest.Config from the bytes of a kubeconfig.
// It takes an optional list of additionally allowed kubeconfig fields.
func RESTConfigFromKubeconfig(kubeconfig []byte, allowedFields ...string) (*rest.Config, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}

	if err := validateClientConfig(clientConfig, allowedFields); err != nil {
		return nil, err
	}

	return clientConfig.ClientConfig()
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
		case authInfo.ClientCertificate != "" && !slices.Contains(validFields, AuthClientCertificate):
			return fmt.Errorf("client certificate files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.ClientKey != "" && !slices.Contains(validFields, AuthClientKey):
			return fmt.Errorf("client key files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.TokenFile != "" && !slices.Contains(validFields, AuthTokenFile):
			return fmt.Errorf("token files are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case (authInfo.Impersonate != "" || len(authInfo.ImpersonateGroups) > 0) && !slices.Contains(validFields, AuthImpersonate):
			return fmt.Errorf("impersonation is not supported, these are the valid fields: %+v", validFields)
		case (authInfo.AuthProvider != nil && len(authInfo.AuthProvider.Config) > 0) && !slices.Contains(validFields, AuthProvider):
			return fmt.Errorf("auth provider configurations are not supported (user %q), these are the valid fields: %+v", user, validFields)
		case authInfo.Exec != nil && !slices.Contains(validFields, AuthExec):
			return fmt.Errorf("exec configurations are not supported (user %q), these are the valid fields: %+v", user, validFields)
		}
	}
	return nil
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
			Scheme:     conf.clientOptions.Scheme,
			Mapper:     conf.clientOptions.Mapper,
			SyncPeriod: conf.cacheSyncPeriod,
		})
		if err != nil {
			return nil, err
		}
	}

	var uncachedClient client.Client
	if runtimeAPIReader == nil || runtimeClient == nil {
		uncachedClient, err = newClient(conf, nil)
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
			cachedClient, err := newClient(conf, runtimeCache)
			if err != nil {
				return nil, err
			}

			runtimeClient = &FallbackClient{
				Client: cachedClient,
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

		applier:     NewApplier(runtimeClient, conf.clientOptions.Mapper),
		podExecutor: NewPodExecutor(conf.restConfig),

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

func newClient(conf *Config, reader client.Reader) (client.Client, error) {
	cacheOptions := conf.clientOptions.Cache
	if cacheOptions == nil {
		cacheOptions = &client.CacheOptions{}
	}

	cacheOptions.Reader = reader
	conf.clientOptions.Cache = cacheOptions

	return client.New(conf.restConfig, conf.clientOptions)
}

var _ client.Client = &FallbackClient{}

// FallbackClient holds a `client.Client` and a `client.Reader` which is meant as a fallback
// in case the kind of an object is configured in `KindToNamespaces` but the namespace isn't.
type FallbackClient struct {
	client.Client
	Reader           client.Reader
	KindToNamespaces map[string]sets.Set[string]
}

// Get retrieves an obj for a given object key from the Kubernetes Cluster.
// `client.Reader` is used in case the kind of an object is configured in `KindToNamespaces` but the namespace isn't.
func (d *FallbackClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvk, err := apiutil.GVKForObject(obj, d.Scheme())
	if err != nil {
		return err
	}

	// Check if there are specific namespaces for this object's kind in the cache.
	namespaces, ok := d.KindToNamespaces[gvk.Kind]

	// If there are specific namespaces for this kind in the cache and the object's namespace is not cached,
	// use the API reader to get the object.
	if ok && !namespaces.Has(obj.GetNamespace()) {
		return d.Reader.Get(ctx, key, obj, opts...)
	}

	// Otherwise, try to get the object from the cache.
	return d.Client.Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options.
func (d *FallbackClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return d.Client.List(ctx, list, opts...)
}

// ApplyClientConnectionConfigurationToRESTConfig applies the given client connection configurations to the given
// REST config.
func ApplyClientConnectionConfigurationToRESTConfig(clientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration, rest *rest.Config) {
	if clientConnection == nil {
		return
	}

	rest.AcceptContentTypes = clientConnection.AcceptContentTypes
	rest.ContentType = clientConnection.ContentType
	rest.Burst = int(clientConnection.Burst)
	rest.QPS = clientConnection.QPS
}
