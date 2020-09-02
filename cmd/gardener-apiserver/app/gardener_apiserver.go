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
	"io"

	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/api"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/apiserver"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	"github.com/gardener/gardener/pkg/apiserver/features"
	"github.com/gardener/gardener/pkg/apiserver/storage"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	gardenversionedcoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardenexternalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	clientkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	settingsclientset "github.com/gardener/gardener/pkg/client/settings/clientset/versioned"
	settingsinformer "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"
	"github.com/gardener/gardener/pkg/openapi"
	"github.com/gardener/gardener/pkg/version"
	controllerregistrationresources "github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	"github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"
	"github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	"github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"
	"github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	plantvalidator "github.com/gardener/gardener/plugin/pkg/plant"
	seedvalidator "github.com/gardener/gardener/plugin/pkg/seed/validator"
	shootdns "github.com/gardener/gardener/plugin/pkg/shoot/dns"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/clusteropenidconnectpreset"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/openidconnectpreset"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	shoottolerationrestriction "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	shootstatedeletionvalidator "github.com/gardener/gardener/plugin/pkg/shootstate/validator"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/admission"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/options/encryptionconfig"
	"k8s.io/apiserver/pkg/server/resourceconfig"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)
	opts.Recommended.AddFlags(flags)
	opts.ExtraOptions.AddFlags(flags)
	return cmd
}

// Options has all the context and parameters needed to run a Gardener API server.
type Options struct {
	Recommended                 *genericoptions.RecommendedOptions
	ExtraOptions                *apiserver.ExtraOptions
	CoreInformerFactory         gardencoreinformers.SharedInformerFactory
	ExternalCoreInformerFactory gardenexternalcoreinformers.SharedInformerFactory
	KubeInformerFactory         kubeinformers.SharedInformerFactory
	SettingsInformerFactory     settingsinformer.SharedInformerFactory
	StdOut                      io.Writer
	StdErr                      io.Writer
}

// NewOptions returns a new Options object.
func NewOptions(out, errOut io.Writer) *Options {
	o := &Options{
		Recommended: genericoptions.NewRecommendedOptions(
			"/registry-gardener",
			api.Codecs.LegacyCodec(
				gardencorev1alpha1.SchemeGroupVersion,
				settingsv1alpha1.SchemeGroupVersion,
			),
			genericoptions.NewProcessInfo("gardener-apiserver", "garden"),
		),
		ExtraOptions: &apiserver.ExtraOptions{},
		StdOut:       out,
		StdErr:       errOut,
	}
	o.Recommended.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(
		gardencorev1beta1.SchemeGroupVersion,
		schema.GroupKind{Group: gardencorev1alpha1.GroupName},
		schema.GroupKind{Group: gardencorev1beta1.GroupName},
	)
	return o
}

// validate validates all the required options.
func (o Options) validate(args []string) error {
	errs := []error{}
	errs = append(errs, o.Recommended.Validate()...)
	errs = append(errs, o.ExtraOptions.Validate()...)

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
	extensionvalidation.Register(o.Recommended.Admission.Plugins)
	shoottolerationrestriction.Register(o.Recommended.Admission.Plugins)
	shootquotavalidator.Register(o.Recommended.Admission.Plugins)
	shootdns.Register(o.Recommended.Admission.Plugins)
	shootvalidator.Register(o.Recommended.Admission.Plugins)
	seedvalidator.Register(o.Recommended.Admission.Plugins)
	controllerregistrationresources.Register(o.Recommended.Admission.Plugins)
	plantvalidator.Register(o.Recommended.Admission.Plugins)
	openidconnectpreset.Register(o.Recommended.Admission.Plugins)
	clusteropenidconnectpreset.Register(o.Recommended.Admission.Plugins)
	shootstatedeletionvalidator.Register(o.Recommended.Admission.Plugins)
	customverbauthorizer.Register(o.Recommended.Admission.Plugins)

	allOrderedPlugins := []string{
		resourcereferencemanager.PluginName,
		extensionvalidation.PluginName,
		shoottolerationrestriction.PluginName,
		shootdns.PluginName,
		shootquotavalidator.PluginName,
		shootvalidator.PluginName,
		seedvalidator.PluginName,
		controllerregistrationresources.PluginName,
		plantvalidator.PluginName,
		deletionconfirmation.PluginName,
		openidconnectpreset.PluginName,
		clusteropenidconnectpreset.PluginName,
		shootstatedeletionvalidator.PluginName,
		customverbauthorizer.PluginName,
	}
	o.Recommended.Admission.RecommendedPluginOrder = append(o.Recommended.Admission.RecommendedPluginOrder, allOrderedPlugins...)

	return nil
}

func (o *Options) config(kubeAPIServerConfig *rest.Config, kubeClient *kubernetes.Clientset) (*apiserver.Config, error) {
	// Create clientset for the owned API groups
	// Use loopback config to create a new Kubernetes client for the owned API groups
	gardenerAPIServerConfig := genericapiserver.NewRecommendedConfig(api.Codecs)
	o.KubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, kubeAPIServerConfig.Timeout)

	// Initialize admission plugins
	o.Recommended.ExtraAdmissionInitializers = func(c *genericapiserver.RecommendedConfig) ([]admission.PluginInitializer, error) {
		// core client
		coreClient, err := gardencoreclientset.NewForConfig(gardenerAPIServerConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		o.CoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(coreClient, gardenerAPIServerConfig.LoopbackClientConfig.Timeout)

		// versioned core client
		versionedCoreClient, err := gardenversionedcoreclientset.NewForConfig(gardenerAPIServerConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		o.ExternalCoreInformerFactory = gardenexternalcoreinformers.NewSharedInformerFactory(versionedCoreClient, gardenerAPIServerConfig.LoopbackClientConfig.Timeout)

		// settings client
		settingsClient, err := settingsclientset.NewForConfig(gardenerAPIServerConfig.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		o.SettingsInformerFactory = settingsinformer.NewSharedInformerFactory(settingsClient, gardenerAPIServerConfig.LoopbackClientConfig.Timeout)

		// dynamic client
		dynamicClient, err := dynamic.NewForConfig(kubeAPIServerConfig)
		if err != nil {
			return nil, err
		}

		return []admission.PluginInitializer{
			admissioninitializer.New(
				o.CoreInformerFactory,
				coreClient,
				o.ExternalCoreInformerFactory,
				o.SettingsInformerFactory,
				o.KubeInformerFactory,
				kubeClient,
				dynamicClient,
				gardenerAPIServerConfig.Authorization.Authorizer,
			),
		}, nil
	}

	apiConfig := &apiserver.Config{
		GenericConfig: gardenerAPIServerConfig,
		ExtraConfig:   apiserver.ExtraConfig{},
	}

	if err := o.ApplyTo(apiConfig); err != nil {
		return nil, err
	}

	return apiConfig, nil
}

func (o Options) run(stopCh <-chan struct{}) error {
	// Create clientset for the native Kubernetes API group
	// Use remote kubeconfig file (if set) or in-cluster config to create a new Kubernetes client for the native Kubernetes API groups
	kubeAPIServerConfig, err := clientcmd.BuildConfigFromFlags("", o.Recommended.Authentication.RemoteKubeConfigFile)
	if err != nil {
		return err
	}

	// kube client
	kubeClient, err := kubernetes.NewForConfig(kubeAPIServerConfig)
	if err != nil {
		return err
	}

	config, err := o.config(kubeAPIServerConfig, kubeClient)
	if err != nil {
		return err
	}
	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	if err := server.GenericAPIServer.AddPostStartHook("start-gardener-apiserver-informers", func(context genericapiserver.PostStartHookContext) error {
		o.CoreInformerFactory.Start(context.StopCh)
		o.ExternalCoreInformerFactory.Start(context.StopCh)
		o.KubeInformerFactory.Start(context.StopCh)
		o.SettingsInformerFactory.Start(context.StopCh)
		return nil
	}); err != nil {
		return err
	}

	ctx := utils.ContextFromStopChannel(stopCh)

	if err := server.GenericAPIServer.AddPostStartHook("bootstrap-garden-cluster", func(context genericapiserver.PostStartHookContext) error {
		if _, err := kubeClient.CoreV1().Namespaces().Get(ctx, gardencorev1beta1.GardenerSeedLeaseNamespace, metav1.GetOptions{}); client.IgnoreNotFound(err) != nil {
			return err
		} else if err == nil {
			// Namespace already exists
			return nil
		}

		_, err = kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardencorev1beta1.GardenerSeedLeaseNamespace,
			},
		}, clientkubernetes.DefaultCreateOptions())
		return err
	}); err != nil {
		return err
	}

	if err := server.GenericAPIServer.AddPostStartHook("bootstrap-cluster-identity", func(context genericapiserver.PostStartHookContext) error {
		if _, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, v1beta1constants.ClusterIdentity, metav1.GetOptions{}); client.IgnoreNotFound(err) != nil {
			return err
		} else if err == nil {
			// Cluster identity ConfigMap already exists
			return nil
		}

		_, err = kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.ClusterIdentity,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				v1beta1constants.ClusterIdentity: o.ExtraOptions.ClusterIdentity,
			},
		}, clientkubernetes.DefaultCreateOptions())
		return err
	}); err != nil {
		return err
	}

	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}

// ApplyTo applies the options to the given config.
func (o *Options) ApplyTo(config *apiserver.Config) error {
	gardenerAPIServerConfig := config.GenericConfig

	gardenerVersion := version.Get()
	gardenerAPIServerConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	gardenerAPIServerConfig.OpenAPIConfig.Info.Title = "Gardener"
	gardenerAPIServerConfig.OpenAPIConfig.Info.Version = gardenerVersion.GitVersion
	gardenerAPIServerConfig.Version = &gardenerVersion

	if err := o.Recommended.SecureServing.ApplyTo(&gardenerAPIServerConfig.SecureServing, &gardenerAPIServerConfig.LoopbackClientConfig); err != nil {
		return err
	}
	if err := o.Recommended.Authentication.ApplyTo(&gardenerAPIServerConfig.Authentication, gardenerAPIServerConfig.SecureServing, gardenerAPIServerConfig.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Recommended.Authorization.ApplyTo(&gardenerAPIServerConfig.Authorization); err != nil {
		return err
	}
	if err := o.Recommended.Audit.ApplyTo(&gardenerAPIServerConfig.Config, gardenerAPIServerConfig.ClientConfig, gardenerAPIServerConfig.SharedInformerFactory, o.Recommended.ProcessInfo, o.Recommended.Webhook); err != nil {
		return err
	}
	if err := o.Recommended.Features.ApplyTo(&gardenerAPIServerConfig.Config); err != nil {
		return err
	}
	if err := o.Recommended.CoreAPI.ApplyTo(gardenerAPIServerConfig); err != nil {
		return err
	}
	if initializers, err := o.Recommended.ExtraAdmissionInitializers(gardenerAPIServerConfig); err != nil {
		return err
	} else if err := o.Recommended.Admission.ApplyTo(&gardenerAPIServerConfig.Config, gardenerAPIServerConfig.SharedInformerFactory, gardenerAPIServerConfig.ClientConfig, features.FeatureGate, initializers...); err != nil {
		return err
	}

	resourceConfig := serverstorage.NewResourceConfig()
	resourceConfig.EnableVersions(
		gardencorev1alpha1.SchemeGroupVersion,
		settingsv1alpha1.SchemeGroupVersion,
	)

	mergedResourceConfig, err := resourceconfig.MergeAPIResourceConfigs(resourceConfig, nil, api.Scheme)
	if err != nil {
		return err
	}

	resourceEncodingConfig := serverstorage.NewDefaultResourceEncodingConfig(api.Scheme)
	// TODO: `ShootState` is not yet promoted to `core.gardener.cloud/v1beta1` - this can be removed once `ShootState` got promoted.
	resourceEncodingConfig.SetResourceEncoding(gardencore.Resource("shootstates"), gardencorev1alpha1.SchemeGroupVersion, gardencore.SchemeGroupVersion)

	storageFactory := &storage.GardenerStorageFactory{
		DefaultStorageFactory: serverstorage.NewDefaultStorageFactory(
			o.Recommended.Etcd.StorageConfig,
			o.Recommended.Etcd.DefaultStorageMediaType,
			api.Codecs,
			resourceEncodingConfig,
			mergedResourceConfig,
			nil,
		),
	}

	if len(o.Recommended.Etcd.EncryptionProviderConfigFilepath) != 0 {
		transformerOverrides, err := encryptionconfig.GetTransformerOverrides(o.Recommended.Etcd.EncryptionProviderConfigFilepath)
		if err != nil {
			return err
		}
		for groupResource, transformer := range transformerOverrides {
			storageFactory.SetTransformer(groupResource, transformer)
		}
	}

	return o.Recommended.Etcd.ApplyWithStorageFactoryTo(storageFactory, &gardenerAPIServerConfig.Config)
}
