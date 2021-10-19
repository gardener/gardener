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

var _ Option = &TargetClusterOptions{}

// TargetClusterOptions contains options needed to construct the target config.
type TargetClusterOptions struct {
	kubeconfigPath      string
	namespace           string
	disableCachedClient bool
	cacheResyncPeriod   time.Duration

	config *TargetClusterConfig
}

// TargetClusterConfig contains the constructed target clients including a RESTMapper and Scheme.
// Before the first usage, Start and WaitForCacheSync should be called to ensure that the cache is running
// and has been populated successfully.
type TargetClusterConfig struct {
	Cluster cluster.Cluster
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *TargetClusterOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.kubeconfigPath, "target-kubeconfig", "", "path to the kubeconfig for the target cluster")
	fs.StringVar(&o.namespace, "target-namespace", "", "namespace in which controllers for the target cluster act on objects (defaults to all namespaces)")
	fs.BoolVar(&o.disableCachedClient, "target-disable-cache", false, "disable the cache for target cluster client and always talk directly to the API server (defaults to false)")
	fs.DurationVar(&o.cacheResyncPeriod, "target-cache-resync-period", 24*time.Hour, "duration how often the controller's cache for the target cluster is resynced")
}

// Complete builds the target config based on the given flag values and saves it for retrieval via Completed.
func (o *TargetClusterOptions) Complete() error {
	restConfig, err := getTargetRESTConfig(o.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("unable to create REST config for target cluster: %w", err)
	}
	// TODO: make this configurable
	restConfig.QPS = 100.0
	restConfig.Burst = 130

	cluster, err := cluster.New(
		restConfig,
		func(opts *cluster.Options) {
			opts.Namespace = o.namespace
			opts.Scheme = getTargetScheme()
			opts.MapperProvider = getTargetRESTMapper
			opts.SyncPeriod = &o.cacheResyncPeriod
			opts.ClientDisableCacheFor = []client.Object{
				&corev1.Event{},
				&eventsv1beta1.Event{},
				&eventsv1.Event{},
			}

			if o.disableCachedClient {
				opts.NewClient = func(_ cache.Cache, config *rest.Config, opts client.Options, _ ...client.Object) (client.Client, error) {
					return client.New(config, opts)
				}
			}
		},
	)
	if err != nil {
		return fmt.Errorf("could not instantiate target cluster: %w", err)
	}

	o.config = &TargetClusterConfig{Cluster: cluster}
	return nil
}

// Completed returns the constructed target clients including a RESTMapper and Scheme.
// Before the first usage, Start and WaitForCacheSync should be called to ensure that the cache is running
// and has been populated successfully.
func (o *TargetClusterOptions) Completed() *TargetClusterConfig {
	return o.config
}

func getTargetScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(kubernetesscheme.AddToScheme(scheme))
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
