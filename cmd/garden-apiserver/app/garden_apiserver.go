// Copyright 2018 The Gardener Authors.
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
	shootdnshostedzone "github.com/gardener/gardener/plugin/pkg/shoot/dnshostedzone"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	shootseedfinder "github.com/gardener/gardener/plugin/pkg/shoot/seedfinder"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewCommandStartGardenAPIServer creates a *cobra.Command object with default parameters.
func NewCommandStartGardenAPIServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	opts := NewOptions(out, errOut)

	cmd := &cobra.Command{
		Use:   "garden-apiserver",
		Short: "Launch the Garden API server",
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
	opts.Recommended.AddFlags(flags)
	opts.Admission.AddFlags(flags)
	return cmd
}

// Options has all the context and parameters needed to run a Garden API server.
type Options struct {
	Admission   *genericoptions.AdmissionOptions
	Recommended *genericoptions.RecommendedOptions
	StdOut      io.Writer
	StdErr      io.Writer
}

// NewOptions returns a new Options object.
func NewOptions(out, errOut io.Writer) *Options {
	return &Options{
		Recommended: genericoptions.NewRecommendedOptions(fmt.Sprintf("/registry/%s", garden.GroupName), api.Codecs.LegacyCodec(gardenv1beta1.SchemeGroupVersion)),
		Admission:   genericoptions.NewAdmissionOptions(),
		StdOut:      out,
		StdErr:      errOut,
	}
}

// validate validates all the required options.
func (o Options) validate(args []string) error {
	errors := []error{}
	errors = append(errors, o.Recommended.Validate()...)
	errors = append(errors, o.Admission.Validate()...)
	return utilerrors.NewAggregate(errors)
}

func (o *Options) complete() error {
	return nil
}

func (o Options) config() (*apiserver.Config, gardeninformers.SharedInformerFactory, kubeinformers.SharedInformerFactory, error) {
	// Require server certificate specification
	keyCert := &o.Recommended.SecureServing.ServerCert.CertKey
	if len(keyCert.CertFile) == 0 || len(keyCert.KeyFile) == 0 {
		return nil, nil, nil, errors.New("need to specify --tls-cert-file and --tls-private-key-file")
	}

	// Create clientset for the garden.sapcloud.io API group
	// Use loopback config to create a new Kubernetes client for the garden.sapcloud.io API group
	gardenAPIServerConfig := genericapiserver.NewRecommendedConfig(api.Codecs)
	if err := o.Recommended.ApplyTo(gardenAPIServerConfig); err != nil {
		return nil, nil, nil, err
	}
	gardenClient, err := gardenclientset.NewForConfig(gardenAPIServerConfig.LoopbackClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	gardenInformerFactory := gardeninformers.NewSharedInformerFactory(gardenClient, gardenAPIServerConfig.LoopbackClientConfig.Timeout)

	// Create clientset for the native Kubernetes API group
	// Use remote kubeconfig file (if set) or in-cluster config to create a new Kubernetes client for the native Kubernetes API groups
	kubeAPIServerConfig, err := clientcmd.BuildConfigFromFlags("", o.Recommended.Authentication.RemoteKubeConfigFile)
	if err != nil {
		return nil, nil, nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, kubeAPIServerConfig.Timeout)

	// Admission plugin registration
	shootquotavalidator.Register(o.Admission.Plugins)
	shootseedfinder.Register(o.Admission.Plugins)
	shootdnshostedzone.Register(o.Admission.Plugins)
	shootvalidator.Register(o.Admission.Plugins)

	// Initialize admission plugins
	admissionInitializer := admissioninitializer.New(gardenInformerFactory, kubeInformerFactory)
	if err := o.Admission.ApplyTo(&gardenAPIServerConfig.Config, gardenAPIServerConfig.SharedInformerFactory, gardenAPIServerConfig.ClientConfig, api.Scheme, admissionInitializer); err != nil {
		return nil, nil, nil, err
	}

	return &apiserver.Config{
		GenericConfig: gardenAPIServerConfig,
		ExtraConfig:   apiserver.ExtraConfig{},
	}, gardenInformerFactory, kubeInformerFactory, nil
}

func (o Options) run(stopCh <-chan struct{}) error {
	config, gardenInformerFactory, kubeInformerFactory, err := o.config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHook("start-garden-apiserver-informers", func(context genericapiserver.PostStartHookContext) error {
		gardenInformerFactory.Start(context.StopCh)
		kubeInformerFactory.Start(context.StopCh)
		return nil
	})

	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
