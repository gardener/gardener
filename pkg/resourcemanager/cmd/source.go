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
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var _ Option = &SourceClientOptions{}

// SourceClientOptions contains options needed to construct the source client.
type SourceClientOptions struct {
	kubeconfigPath    string
	cacheResyncPeriod time.Duration
	namespace         string

	sourceClient *SourceClientConfig
}

// SourceClientConfig contains the constructed source clients including a kubernetes ClientSet and Scheme.
type SourceClientConfig struct {
	RESTConfig        *rest.Config
	CacheResyncPeriod *time.Duration
	ClientSet         *kubernetes.Clientset
	Scheme            *runtime.Scheme
	Namespace         string
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *SourceClientOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.kubeconfigPath, "kubeconfig", "", "path to the kubeconfig for the source cluster")
	fs.StringVar(&o.namespace, "namespace", "", "namespace in which the ManagedResources should be observed (defaults to all namespaces)")
	fs.DurationVar(&o.cacheResyncPeriod, "cache-resync-period", 24*time.Hour, "duration how often the controller's cache is resynced")
}

// Complete builds the source client based on the given flag values and saves it for retrieval via Completed.
func (o *SourceClientOptions) Complete() error {
	restConfig, err := getSourceRESTConfig(o.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("unable to create REST config for source cluster: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("could not create discovery client: %+v", err)
	}

	scheme := getSourceScheme()

	cacheResyncPeriod := o.cacheResyncPeriod
	o.sourceClient = &SourceClientConfig{
		RESTConfig:        restConfig,
		CacheResyncPeriod: &cacheResyncPeriod,
		ClientSet:         clientSet,
		Scheme:            scheme,
		Namespace:         o.namespace,
	}
	return nil
}

// Completed returns the constructed source clients including a kubernetes ClientSet and Scheme.
func (o *SourceClientOptions) Completed() *SourceClientConfig {
	return o.sourceClient
}

func getSourceScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(kubernetesscheme.AddToScheme(scheme))
	utilruntime.Must(resourcesv1alpha1.AddToScheme(scheme))
	return scheme
}

func getSourceRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("could not instantiate rest config: %w", err)
		}
		return cfg, nil
	}
	return config.GetConfig()
}

// ApplyManagerOptions sets the values of this SourceClientConfig on the given manager.Options.
func (c *SourceClientConfig) ApplyManagerOptions(opts *manager.Options) {
	opts.Scheme = c.Scheme
	opts.SyncPeriod = c.CacheResyncPeriod
	opts.Namespace = c.Namespace
}

// ApplyClientSet sets clientSet to the ClientSet of this config.
func (c *SourceClientConfig) ApplyClientSet(clientSet *kubernetes.Clientset) {
	*clientSet = *c.ClientSet
}
