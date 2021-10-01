// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	volumesnapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	apiregistrationinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

var _ Option = &TargetClientOptions{}

// TargetClientOptions contains options needed to construct the target client.
type TargetClientOptions struct {
	KubeconfigPath    string
	DisableCache      bool
	CacheResyncPeriod time.Duration

	targetClient *TargetClientConfig
}

// TargetClientConfig contains the constructed target clients including a RESTMapper and Scheme.
// Before the first usage, Start and WaitForCacheSync should be called to ensure that the cache is running
// and has been populated successfully.
type TargetClientConfig struct {
	Client     client.Client
	RESTMapper meta.RESTMapper
	Scheme     *runtime.Scheme

	cache cache.Cache
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *TargetClientOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.KubeconfigPath, "target-kubeconfig", "", "path to the kubeconfig for the target cluster")
	fs.BoolVar(&o.DisableCache, "target-disable-cache", false, "disable the cache for target cluster and always talk directly to the API server (defaults to false)")
	fs.DurationVar(&o.CacheResyncPeriod, "target-cache-resync-period", 24*time.Hour, "duration how often the controller's cache for the target cluster is resynced")
}

// Complete builds the target client based on the given flag values and saves it for retrieval via Completed.
func (o *TargetClientOptions) Complete() error {
	tcc, err := NewTargetClientConfig(o.KubeconfigPath, o.DisableCache, o.CacheResyncPeriod)
	if err != nil {
		return err
	}

	o.targetClient = tcc
	return nil
}

// Completed returns the constructed target clients including a RESTMapper and Scheme.
// Before the first usage, Start and WaitForCacheSync should be called to ensure that the cache is running
// and has been populated successfully.
func (o *TargetClientOptions) Completed() *TargetClientConfig {
	return o.targetClient
}

// NewTargetClientConfig creates a new target client config.
func NewTargetClientConfig(kubeconfigPath string, disableCache bool, cacheResyncPeriod time.Duration) (*TargetClientConfig, error) {
	restConfig, err := getTargetRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create REST config for target cluster: %w", err)
	}

	// TODO: make this configurable
	restConfig.QPS = 100.0
	restConfig.Burst = 130

	restMapper, err := getTargetRESTMapper(restConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create REST mapper for target cluster: %w", err)
	}

	scheme := getTargetScheme()

	var (
		targetCache  cache.Cache
		targetClient client.Client
	)

	if disableCache {
		// create direct client for target cluster
		targetClient, err = client.New(restConfig, client.Options{
			Mapper: restMapper,
			Scheme: scheme,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to create client for target cluster: %w", err)
		}
	} else {
		// create cached client for target cluster
		targetCache, err = cache.New(restConfig, cache.Options{
			Mapper: restMapper,
			Resync: &cacheResyncPeriod,
			Scheme: scheme,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to create client cache for target cluster: %w", err)
		}

		targetClient, err = newCachedClient(targetCache, *restConfig, client.Options{
			Mapper: restMapper,
			Scheme: scheme,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to create client for target cluster: %w", err)
		}
	}

	return &TargetClientConfig{
		Client:     targetClient,
		RESTMapper: restMapper,
		Scheme:     scheme,
		cache:      targetCache,
	}, nil
}

func getTargetScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(kubernetesscheme.AddToScheme(scheme)) // add most of the standard k8s APIs
	apiextensionsinstall.Install(scheme)
	apiregistrationinstall.Install(scheme)
	utilruntime.Must(hvpav1alpha1.AddToScheme(scheme))
	utilruntime.Must(volumesnapshotv1beta1.AddToScheme(scheme))

	return scheme
}

func getTargetRESTMapper(config *rest.Config) (meta.RESTMapper, error) {
	// use dynamic rest mapper for target cluster, which will automatically rediscover resources on NoMatchErrors
	// but is rate-limited to not issue to many discovery calls (rate-limit shared across all reconciliations)
	return apiutil.NewDynamicRESTMapper(
		config,
		apiutil.WithLazyDiscovery,
		apiutil.WithLimiter(rate.NewLimiter(rate.Every(1*time.Minute), 1)), // rediscover at maximum every minute
	)
}

func getTargetRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	if len(kubeconfigPath) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	if kubeconfig := os.Getenv("KUBECONFIG"); len(kubeconfig) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("could not create config for cluster")
}

func newCachedClient(cache cache.Cache, config rest.Config, options client.Options) (client.Client, error) {
	return cluster.DefaultNewClient(cache, &config, options,
		&corev1.Event{},
		&eventsv1beta1.Event{},
		&eventsv1.Event{},
	)
}

// Start starts the target cache if the client is cached.
func (c *TargetClientConfig) Start(ctx context.Context) error {
	if c.cache == nil {
		return nil
	}
	return c.cache.Start(ctx)
}

// WaitForCacheSync waits for the caches of the target cache to be synced initially.
func (c *TargetClientConfig) WaitForCacheSync(ctx context.Context) bool {
	if c.cache == nil {
		return true
	}
	return c.cache.WaitForCacheSync(ctx)
}

// Apply sets the values of this TargetClientConfig on the given config.
func (c *TargetClientConfig) Apply(conf *TargetClientConfig) {
	conf.Client = c.Client
	conf.RESTMapper = c.RESTMapper
	conf.Scheme = c.Scheme
}
