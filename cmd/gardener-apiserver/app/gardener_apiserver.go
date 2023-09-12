// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/quota/v1/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/resourceconfig"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/api"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/apiserver"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	"github.com/gardener/gardener/pkg/apiserver/storage"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	gardencoreversionedclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreexternalinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	settingsclientset "github.com/gardener/gardener/pkg/client/settings/clientset/versioned"
	settingsinformers "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/openapi"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// NewCommandStartGardenerAPIServer creates a *cobra.Command object with default parameters.
func NewCommandStartGardenerAPIServer() *cobra.Command {
	opts := NewOptions()

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
			verflag.PrintAndExitIfRequested()

			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Run(c.Context())
		},
		SilenceUsage: true,
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	// has to be after opts.AddFlags because controller-runtime registers "--kubeconfig" on flag.CommandLine
	// see https://github.com/kubernetes-sigs/controller-runtime/blob/v0.8.0/pkg/client/config/config.go#L38
	flags.AddGoFlagSet(flag.CommandLine)

	return cmd
}

// Options has all the context and parameters needed to run a Gardener API server.
type Options struct {
	Recommended                   *genericoptions.RecommendedOptions
	ServerRunOptions              *genericoptions.ServerRunOptions
	ExtraOptions                  *apiserver.ExtraOptions
	CoreInformerFactory           gardencoreinformers.SharedInformerFactory
	ExternalCoreInformerFactory   gardencoreexternalinformers.SharedInformerFactory
	KubeInformerFactory           kubeinformers.SharedInformerFactory
	SeedManagementInformerFactory seedmanagementinformers.SharedInformerFactory
	SettingsInformerFactory       settingsinformers.SharedInformerFactory

	Logs *logsv1.LoggingConfiguration
}

// NewOptions returns a new Options object.
func NewOptions() *Options {
	o := &Options{
		Recommended: genericoptions.NewRecommendedOptions(
			"/registry-gardener",
			api.Codecs.LegacyCodec(
				seedmanagementv1alpha1.SchemeGroupVersion,
				settingsv1alpha1.SchemeGroupVersion,
				operationsv1alpha1.SchemeGroupVersion,
			),
		),
		ServerRunOptions: genericoptions.NewServerRunOptions(),
		ExtraOptions:     &apiserver.ExtraOptions{},
		Logs:             logsv1.NewLoggingConfiguration(),
	}
	o.Recommended.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(
		gardencorev1beta1.SchemeGroupVersion,
		schema.GroupKind{Group: gardencorev1beta1.GroupName},
	)
	apiserver.RegisterAllAdmissionPlugins(o.Recommended.Admission.Plugins)
	o.Recommended.Admission.DefaultOffPlugins = sets.NewString(plugin.AllPluginNames()...).Difference(plugin.DefaultOnPlugins())
	o.Recommended.Admission.RecommendedPluginOrder = plugin.AllPluginNames()

	return o
}

// AddFlags adds all flags to the given FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.Recommended.AddFlags(flags)
	o.ServerRunOptions.AddUniversalFlags(flags)
	o.ExtraOptions.AddFlags(flags)
	logsv1.AddFlags(o.Logs, flags)
}

// Validate validates all the required options.
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.Recommended.Validate()...)
	errs = append(errs, o.ServerRunOptions.Validate()...)
	errs = append(errs, o.ExtraOptions.Validate()...)

	// Require server certificate specification
	keyCert := &o.Recommended.SecureServing.ServerCert.CertKey
	if len(keyCert.CertFile) == 0 || len(keyCert.KeyFile) == 0 {
		errs = append(errs, errors.New("must specify both --tls-cert-file and --tls-private-key-file"))
	}

	// Activate logging as soon as possible
	if err := logsv1.ValidateAndApply(o.Logs, nil); err != nil {
		return err
	}

	return utilerrors.NewAggregate(errs)
}

func (o *Options) config(kubeAPIServerConfig *rest.Config, kubeClient *kubernetes.Clientset) (*apiserver.Config, error) {
	// Create clientset for the owned API groups
	// Use loopback config to create a new Kubernetes client for the owned API groups
	gardenerAPIServerConfig := genericapiserver.NewRecommendedConfig(api.Codecs)
	o.KubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, kubeAPIServerConfig.Timeout)

	apiConfig := &apiserver.Config{
		GenericConfig:       gardenerAPIServerConfig,
		ExtraConfig:         apiserver.ExtraConfig{},
		KubeInformerFactory: o.KubeInformerFactory,
	}

	if err := o.ApplyTo(apiConfig); err != nil {
		return nil, err
	}

	protobufLoopbackConfig := *gardenerAPIServerConfig.LoopbackClientConfig
	if protobufLoopbackConfig.ContentType == "" {
		protobufLoopbackConfig.ContentType = runtime.ContentTypeProtobuf
	}

	// core client
	coreClient, err := gardencoreclientset.NewForConfig(&protobufLoopbackConfig)
	if err != nil {
		return nil, err
	}
	o.CoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(coreClient, protobufLoopbackConfig.Timeout)
	apiConfig.CoreInformerFactory = o.CoreInformerFactory

	// versioned core client
	versionedCoreClient, err := gardencoreversionedclientset.NewForConfig(&protobufLoopbackConfig)
	if err != nil {
		return nil, err
	}
	o.ExternalCoreInformerFactory = gardencoreexternalinformers.NewSharedInformerFactory(versionedCoreClient, protobufLoopbackConfig.Timeout)

	// seedmanagement client
	seedManagementClient, err := seedmanagementclientset.NewForConfig(&protobufLoopbackConfig)
	if err != nil {
		return nil, err
	}
	o.SeedManagementInformerFactory = seedmanagementinformers.NewSharedInformerFactory(seedManagementClient, protobufLoopbackConfig.Timeout)

	// settings client
	settingsClient, err := settingsclientset.NewForConfig(&protobufLoopbackConfig)
	if err != nil {
		return nil, err
	}
	o.SettingsInformerFactory = settingsinformers.NewSharedInformerFactory(settingsClient, protobufLoopbackConfig.Timeout)

	// dynamic client
	dynamicClient, err := dynamic.NewForConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}

	// Initialize admission plugins
	o.Recommended.ExtraAdmissionInitializers = func(c *genericapiserver.RecommendedConfig) ([]admission.PluginInitializer, error) {
		return []admission.PluginInitializer{
			admissioninitializer.New(
				o.CoreInformerFactory,
				coreClient,
				o.ExternalCoreInformerFactory,
				versionedCoreClient,
				o.SeedManagementInformerFactory,
				seedManagementClient,
				o.SettingsInformerFactory,
				o.KubeInformerFactory,
				kubeClient,
				dynamicClient,
				gardenerAPIServerConfig.Authorization.Authorizer,
				// ResourceQuota admission plugin configuration is injected via `ExtraAdmissionInitializers`.
				// Ref implementation of Kube-Apiserver:
				// https://github.com/kubernetes/kubernetes/blob/53b2973440a29e1682df6ba687cebc6764bba44c/pkg/kubeapiserver/admission/config.go#L70
				generic.NewConfiguration(nil, nil),
			),
		}, nil
	}

	gardenerKubeClient, err := kubernetes.NewForConfig(gardenerAPIServerConfig.ClientConfig)
	if err != nil {
		return nil, err
	}
	gardenerDynamicClient, err := dynamic.NewForConfig(gardenerAPIServerConfig.ClientConfig)
	if err != nil {
		return nil, err
	}

	if initializers, err := o.Recommended.ExtraAdmissionInitializers(gardenerAPIServerConfig); err != nil {
		return apiConfig, err
	} else if err := o.Recommended.Admission.ApplyTo(&gardenerAPIServerConfig.Config, gardenerAPIServerConfig.SharedInformerFactory, gardenerKubeClient, gardenerDynamicClient, features.DefaultFeatureGate, initializers...); err != nil {
		return apiConfig, err
	}

	return apiConfig, nil
}

// Run runs gardener-apiserver with the given Options.
func (o *Options) Run(ctx context.Context) error {
	log, err := logger.NewZapLogger(o.ExtraOptions.LogLevel, o.ExtraOptions.LogFormat)
	if err != nil {
		return fmt.Errorf("error instantiating zap logger: %w", err)
	}

	logf.SetLogger(log)
	klog.SetLogger(log)

	log.Info("Starting gardener-apiserver", "version", version.Get())
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

	// Create clientset for the native Kubernetes API group
	// Use remote kubeconfig file (if set) or in-cluster config to create a new Kubernetes client for the native Kubernetes API groups
	kubeAPIServerConfig, err := clientcmd.BuildConfigFromFlags("", o.Recommended.CoreAPI.CoreAPIKubeconfigPath)
	if err != nil {
		return err
	}

	protobufConfig := *kubeAPIServerConfig
	if protobufConfig.ContentType == "" {
		protobufConfig.ContentType = runtime.ContentTypeProtobuf
	}

	// kube client
	kubeClient, err := kubernetes.NewForConfig(&protobufConfig)
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
		o.SeedManagementInformerFactory.Start(context.StopCh)
		o.SettingsInformerFactory.Start(context.StopCh)
		return nil
	}); err != nil {
		return err
	}

	if err := server.GenericAPIServer.AddPostStartHook("bootstrap-garden-cluster", func(context genericapiserver.PostStartHookContext) error {
		if _, err := kubeClient.CoreV1().Namespaces().Get(ctx, gardencorev1beta1.GardenerSeedLeaseNamespace, metav1.GetOptions{}); client.IgnoreNotFound(err) != nil {
			return err
		} else if err == nil {
			// Namespace already exists
			return nil
		}

		_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardencorev1beta1.GardenerSeedLeaseNamespace,
			},
		}, kubernetesclient.DefaultCreateOptions())
		return err
	}); err != nil {
		return err
	}

	if err := server.GenericAPIServer.AddPostStartHook("bootstrap-cluster-identity", func(context genericapiserver.PostStartHookContext) error {
		if clusterIdentity, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, v1beta1constants.ClusterIdentity, metav1.GetOptions{}); client.IgnoreNotFound(err) != nil {
			return err
		} else if err == nil {
			// Set immutable flag to true and origin to gardener-apiserver if cluster-identity config map is not immutable, its origin is empty and the cluster-identity is equal the one set by gardener-apiserver
			if pointer.BoolDeref(clusterIdentity.Immutable, false) {
				return nil
			}
			if clusterIdentity.Data[v1beta1constants.ClusterIdentityOrigin] == "" && clusterIdentity.Data[v1beta1constants.ClusterIdentity] == o.ExtraOptions.ClusterIdentity {
				clusterIdentity.Data[v1beta1constants.ClusterIdentityOrigin] = v1beta1constants.ClusterIdentityOriginGardenerAPIServer
				clusterIdentity.Immutable = pointer.Bool(true)
				if _, err = kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Update(ctx, clusterIdentity, kubernetesclient.DefaultUpdateOptions()); err != nil {
					return err
				}
			}
			return nil
		}

		_, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.ClusterIdentity,
				Namespace: metav1.NamespaceSystem,
			},
			Immutable: pointer.Bool(true),
			Data: map[string]string{
				v1beta1constants.ClusterIdentity:       o.ExtraOptions.ClusterIdentity,
				v1beta1constants.ClusterIdentityOrigin: v1beta1constants.ClusterIdentityOriginGardenerAPIServer,
			},
		}, kubernetesclient.DefaultCreateOptions())
		return err
	}); err != nil {
		return err
	}

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}

// ApplyTo applies the options to the given config.
func (o *Options) ApplyTo(config *apiserver.Config) error {
	gardenerAPIServerConfig := config.GenericConfig

	gardenerVersion := version.Get()
	gardenerAPIServerConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(openapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	gardenerAPIServerConfig.OpenAPIV3Config.Info.Title = "Gardener"
	gardenerAPIServerConfig.OpenAPIV3Config.Info.Version = gardenerVersion.GitVersion
	gardenerAPIServerConfig.Version = &gardenerVersion

	if err := o.ServerRunOptions.ApplyTo(&gardenerAPIServerConfig.Config); err != nil {
		return err
	}
	if err := o.Recommended.SecureServing.ApplyTo(&gardenerAPIServerConfig.SecureServing, &gardenerAPIServerConfig.LoopbackClientConfig); err != nil {
		return err
	}
	if err := o.Recommended.Authentication.ApplyTo(&gardenerAPIServerConfig.Authentication, gardenerAPIServerConfig.SecureServing, gardenerAPIServerConfig.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Recommended.Authorization.ApplyTo(&gardenerAPIServerConfig.Authorization); err != nil {
		return err
	}
	if err := o.Recommended.Audit.ApplyTo(&gardenerAPIServerConfig.Config); err != nil {
		return err
	}
	if err := o.Recommended.Features.ApplyTo(&gardenerAPIServerConfig.Config); err != nil {
		return err
	}
	if err := o.Recommended.CoreAPI.ApplyTo(gardenerAPIServerConfig); err != nil {
		return err
	}
	if err := o.ExtraOptions.ApplyTo(config); err != nil {
		return err
	}

	resourceConfig := serverstorage.NewResourceConfig()
	resourceConfig.EnableVersions(
		seedmanagementv1alpha1.SchemeGroupVersion,
		settingsv1alpha1.SchemeGroupVersion,
		operationsv1alpha1.SchemeGroupVersion,
	)

	mergedResourceConfig, err := resourceconfig.MergeAPIResourceConfigs(resourceConfig, nil, api.Scheme)
	if err != nil {
		return err
	}

	resourceEncodingConfig := serverstorage.NewDefaultResourceEncodingConfig(api.Scheme)
	resourceEncodingConfig.SetResourceEncoding(operations.Resource("bastions"), operationsv1alpha1.SchemeGroupVersion, operations.SchemeGroupVersion)

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

	return o.Recommended.Etcd.ApplyWithStorageFactoryTo(storageFactory, &gardenerAPIServerConfig.Config)
}
