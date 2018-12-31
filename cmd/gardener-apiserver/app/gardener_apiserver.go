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

package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apiserver"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/internalversion"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/openapi"
	"github.com/gardener/gardener/pkg/version"
	deletionconfirmation "github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	resourcereferencemanager "github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	shootdnshostedzone "github.com/gardener/gardener/plugin/pkg/shoot/dnshostedzone"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	shootseedmanager "github.com/gardener/gardener/plugin/pkg/shoot/seedmanager"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"

	"github.com/spf13/cobra"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewCommandStartGardenerAPIServer creates a *cobra.Command object with default parameters.
func NewCommandStartGardenerAPIServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	opts := NewOptions(out, errOut)

	cmd := &cobra.Command{
		Use:   "gardener-apiserver",
		Short: "Launch the Gardener API server",
		Long: `In essence, the Gardener is an extension API server along with a bundle
of Kubernetes controllers which introduce new API objects in an existing Kubernetes
cluster (which is called Garden cluster) in order to use them for the management of
further Kubernetes clusters (which are called Shoot clusters).
To do that reliably and to offer a certain quality of service, it requires to control
the main components of a Kubernetes cluster (etcd, API server, controller manager, scheduler).
These so-called control plane components are hosted in Kubernetes clusters themselves
(which are called Seed clusters).`,
		RunE: func(c *cobra.Command, args []string) error {
			if err := opts.complete(); err != nil {
				return err
			}
			if err := opts.validate(args); err != nil {
				return err
			}
			return opts.run(stopCh)
		},
	}

	flags := cmd.Flags()
	utilfeature.DefaultFeatureGate.AddFlag(flags)
	opts.Recommended.AddFlags(flags)
	return cmd
}

// Options has all the context and parameters needed to run a Gardener API server.
type Options struct {
	Recommended           *genericoptions.RecommendedOptions
	GardenInformerFactory gardeninformers.SharedInformerFactory
	KubeInformerFactory   kubeinformers.SharedInformerFactory
	StdOut                io.Writer
	StdErr                io.Writer
}

// NewOptions returns a new Options object.
func NewOptions(out, errOut io.Writer) *Options {
	return &Options{
		Recommended: genericoptions.NewRecommendedOptions(fmt.Sprintf("/registry/%s", garden.GroupName), api.Codecs.LegacyCodec(gardenv1beta1.SchemeGroupVersion), genericoptions.NewProcessInfo("gardener-apiserver", "garden")),
		StdOut:      out,
		StdErr:      errOut,
	}
}

// validate validates all the required options.
func (o Options) validate(args []string) error {
	errs := []error{}
	errs = append(errs, o.Recommended.Validate()...)

	// Require server certificate specification
	keyCert := &o.Recommended.SecureServing.ServerCert.CertKey
	if len(keyCert.CertFile) == 0 || len(keyCert.KeyFile) == 0 {
		errs = append(errs, errors.New("must specify both --tls-cert-file and --tls-private-key-file"))
	}

	return utilerrors.NewAggregate(errs)
}

func (o *Options) complete() error {
	// Admission plugin registration
	resourcereferencemanager.Register(o.Recommended.Admission.Plugins)
	deletionconfirmation.Register(o.Recommended.Admission.Plugins)
	shootquotavalidator.Register(o.Recommended.Admission.Plugins)
	shootseedmanager.Register(o.Recommended.Admission.Plugins)
	shootdnshostedzone.Register(o.Recommended.Admission.Plugins)
	shootvalidator.Register(o.Recommended.Admission.Plugins)

	allOrderedPlugins := []string{
		resourcereferencemanager.PluginName,
		deletionconfirmation.PluginName,
		shootdnshostedzone.PluginName,
		shootquotavalidator.PluginName,
		shootseedmanager.PluginName,
		shootvalidator.PluginName,
	}

	recommendedPluginOrder := sets.NewString(o.Recommended.Admission.RecommendedPluginOrder...)
	recommendedPluginOrder.Insert(allOrderedPlugins...)
	o.Recommended.Admission.RecommendedPluginOrder = recommendedPluginOrder.List()

	return nil
}

func (o *Options) config() (*apiserver.Config, error) {
	// Create clientset for the garden.sapcloud.io API group
	// Use loopback config to create a new Kubernetes client for the garden.sapcloud.io API group
	gardenerAPIServerConfig := genericapiserver.NewRecommendedConfig(api.Codecs)

	// Create clientset for the native Kubernetes API group
	// Use remote kubeconfig file (if set) or in-cluster config to create a new Kubernetes client for the native Kubernetes API groups
	kubeAPIServerConfig, err := clientcmd.BuildConfigFromFlags("", o.Recommended.Authentication.RemoteKubeConfigFile)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, kubeAPIServerConfig.Timeout)
	o.KubeInformerFactory = kubeInformerFactory

	// Initialize admission plugins
	o.Recommended.ExtraAdmissionInitializers = func(c *genericapiserver.RecommendedConfig) ([]admission.PluginInitializer, error) {
		gardenClient, err := gardenclientset.NewForConfig(gardenerAPIServerConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		gardenInformerFactory := gardeninformers.NewSharedInformerFactory(gardenClient, gardenerAPIServerConfig.LoopbackClientConfig.Timeout)
		o.GardenInformerFactory = gardenInformerFactory
		return []admission.PluginInitializer{admissioninitializer.New(gardenInformerFactory, gardenClient, kubeInformerFactory, kubeClient, gardenerAPIServerConfig.Authorization.Authorizer)}, nil
	}

	gardenerVersion := version.Get()
	gardenerAPIServerConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	gardenerAPIServerConfig.OpenAPIConfig.Info.Title = "Gardener"
	gardenerAPIServerConfig.OpenAPIConfig.Info.Version = gardenerVersion.GitVersion
	gardenerAPIServerConfig.SwaggerConfig = genericapiserver.DefaultSwaggerConfig()
	gardenerAPIServerConfig.Version = &gardenerVersion

	if err := o.Recommended.ApplyTo(gardenerAPIServerConfig, api.Scheme); err != nil {
		return nil, err
	}

	return &apiserver.Config{
		GenericConfig: gardenerAPIServerConfig,
		ExtraConfig:   apiserver.ExtraConfig{},
	}, nil
}

func (o Options) run(stopCh <-chan struct{}) error {
	config, err := o.config()
	if err != nil {
		return err
	}
	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHook("start-gardener-apiserver-informers", func(context genericapiserver.PostStartHookContext) error {
		o.GardenInformerFactory.Start(context.StopCh)
		o.KubeInformerFactory.Start(context.StopCh)
		return nil
	})

	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
