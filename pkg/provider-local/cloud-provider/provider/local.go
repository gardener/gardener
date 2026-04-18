// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"io"

	dockerclient "github.com/docker/docker/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	cloudproviderv1alpha1 "github.com/gardener/gardener/pkg/provider-local/cloud-provider/api/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/cloud-provider/loadbalancer"
)

// Name is the name of the local cloud provider as specified in the cloud controller manager's --cloud-provider flag.
const Name = "local"

var configDecoder runtime.Decoder

func init() {
	configScheme := runtime.NewScheme()
	utilruntime.Must(cloudproviderv1alpha1.AddToScheme(configScheme))
	configDecoder = serializer.NewCodecFactory(configScheme, serializer.EnableStrict).UniversalDecoder()
}

// Register registers the cloud provider implementation for the cloud-controller-manager.
// Other implementations typically call this function from their init() function, requiring an anyonymous import of this
// package in the main package of the cloud-controller-manager. We take a more explicit approach here by calling this
// function directly from the main package.
func Register() {
	cloudprovider.RegisterCloudProvider(Name, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readConfig(config)
		if err != nil {
			return nil, err
		}

		return New(cfg)
	})
}

func readConfig(reader io.Reader) (*cloudproviderv1alpha1.CloudProviderConfig, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	cfg := &cloudproviderv1alpha1.CloudProviderConfig{}
	if err = runtime.DecodeInto(configDecoder, data, cfg); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}

	return cfg, nil
}

// New returns a new instance of the local cloud provider implementation.
func New(cfg *cloudproviderv1alpha1.CloudProviderConfig) (cloudprovider.Interface, error) {
	dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	runtimeClient, err := createRuntimeClient(cfg.RuntimeCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for runtime cluster: %w", err)
	}

	return &Local{
		loadBalancer: &loadbalancer.Provider{
			Config:        cfg,
			DockerClient:  dockerClient,
			RuntimeClient: runtimeClient,
		},
	}, nil
}

func createRuntimeClient(cfg *cloudproviderv1alpha1.RuntimeCluster) (client.Client, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.Namespace == "" {
		return nil, fmt.Errorf("runtime cluster is configured but namespace is not set")
	}

	clientSet, err := kubernetes.NewClientFromFile("", ptr.Deref(cfg.Kubeconfig, ""),
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, err
	}

	return clientSet.Client(), nil
}

// Local implements the cloudprovider.Interface.
type Local struct {
	loadBalancer *loadbalancer.Provider
}

// ProviderName returns the cloud provider ID. Selected by the --cloud-provider flag of cloud-controller-manager.
func (l *Local) ProviderName() string {
	return Name
}

// HasClusterID returns true if a ClusterID is required and set
func (l *Local) HasClusterID() bool {
	return true
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines to perform housekeeping or
// run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (l *Local) Initialize(_ cloudprovider.ControllerClientBuilder, _ <-chan struct{}) {
	// no initialization needed for now
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (l *Local) LoadBalancer() (cloudprovider.LoadBalancer, bool) { return l.loadBalancer, true }

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (l *Local) Instances() (cloudprovider.Instances, bool) { return nil, false }

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce API calls to
// the cloud provider when registering and syncing nodes. Implementation of this interface will disable calls to the
// Zones interface. Also returns true if the interface is supported, false otherwise.
func (l *Local) InstancesV2() (cloudprovider.InstancesV2, bool) { return nil, false }

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
//
// Deprecated: Zones is deprecated in favor of retrieving zone/region information from InstancesV2.
// This interface will not be called if InstancesV2 is enabled.
func (l *Local) Zones() (cloudprovider.Zones, bool) { return nil, false }

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (l *Local) Clusters() (cloudprovider.Clusters, bool) { return nil, false }

// Routes returns a routes interface along with whether the interface is supported.
func (l *Local) Routes() (cloudprovider.Routes, bool) { return nil, false }
