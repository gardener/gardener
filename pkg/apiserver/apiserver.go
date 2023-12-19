// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeinformers "k8s.io/client-go/informers"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/logger"
	corerest "github.com/gardener/gardener/pkg/registry/core/rest"
	operationsrest "github.com/gardener/gardener/pkg/registry/operations/rest"
	seedmanagementrest "github.com/gardener/gardener/pkg/registry/seedmanagement/rest"
	settingsrest "github.com/gardener/gardener/pkg/registry/settings/rest"
)

// ExtraConfig contains non-generic Gardener API server configuration.
type ExtraConfig struct {
	AdminKubeconfigMaxExpiration  time.Duration
	ViewerKubeconfigMaxExpiration time.Duration
	CredentialsRotationInterval   time.Duration
}

// Config contains Gardener API server configuration.
type Config struct {
	GenericConfig       *genericapiserver.RecommendedConfig
	ExtraConfig         ExtraConfig
	KubeInformerFactory kubeinformers.SharedInformerFactory
	CoreInformerFactory gardencoreinformers.SharedInformerFactory
}

// GardenerServer contains state for a Gardener API server.
type GardenerServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig

	kubeInformerFactory kubeinformers.SharedInformerFactory
	coreInformerFactory gardencoreinformers.SharedInformerFactory
}

// CompletedConfig contains completed Gardener API server configuration.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
		cfg.KubeInformerFactory,
		cfg.CoreInformerFactory,
	}

	return CompletedConfig{&c}
}

// New returns a new instance of GardenerServer from the given config.
func (c completedConfig) New() (*GardenerServer, error) {
	genericServer, err := c.GenericConfig.New("gardener-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	var (
		s                = &GardenerServer{GenericAPIServer: genericServer}
		coreAPIGroupInfo = (corerest.StorageProvider{
			AdminKubeconfigMaxExpiration:  c.ExtraConfig.AdminKubeconfigMaxExpiration,
			ViewerKubeconfigMaxExpiration: c.ExtraConfig.ViewerKubeconfigMaxExpiration,
			CredentialsRotationInterval:   c.ExtraConfig.CredentialsRotationInterval,
			KubeInformerFactory:           c.kubeInformerFactory,
			CoreInformerFactory:           c.coreInformerFactory,
		}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
		seedManagementAPIGroupInfo = (seedmanagementrest.StorageProvider{}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
		settingsAPIGroupInfo       = (settingsrest.StorageProvider{}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
		operationsAPIGroupInfo     = (operationsrest.StorageProvider{}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
	)

	if err := s.GenericAPIServer.InstallAPIGroups(&coreAPIGroupInfo, &settingsAPIGroupInfo, &seedManagementAPIGroupInfo, &operationsAPIGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// ExtraOptions is used for providing additional options to the Gardener API Server
type ExtraOptions struct {
	ClusterIdentity               string
	AdminKubeconfigMaxExpiration  time.Duration
	ViewerKubeconfigMaxExpiration time.Duration
	CredentialsRotationInterval   time.Duration

	LogLevel  string
	LogFormat string
}

// Validate checks if the required flags are set
func (o *ExtraOptions) Validate() []error {
	allErrors := []error{}
	if len(o.ClusterIdentity) == 0 {
		allErrors = append(allErrors, fmt.Errorf("--cluster-identity must be specified"))
	}

	if o.AdminKubeconfigMaxExpiration < time.Hour ||
		o.AdminKubeconfigMaxExpiration > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, fmt.Errorf("--shoot-admin-kubeconfig-max-expiration must be between 1 hour and 2^32 seconds"))
	}

	if o.ViewerKubeconfigMaxExpiration < time.Hour ||
		o.ViewerKubeconfigMaxExpiration > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, fmt.Errorf("--shoot-viewer-kubeconfig-max-expiration must be between 1 hour and 2^32 seconds"))
	}

	if o.CredentialsRotationInterval < 24*time.Hour ||
		o.CredentialsRotationInterval > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, fmt.Errorf("--shoot-credentials-rotation-interval must be between 24 hours and 2^32 seconds"))
	}

	if !sets.New(logger.AllLogLevels...).Has(o.LogLevel) {
		allErrors = append(allErrors, fmt.Errorf("invalid --log-level: %s", o.LogLevel))
	}

	if !sets.New(logger.AllLogFormats...).Has(o.LogFormat) {
		allErrors = append(allErrors, fmt.Errorf("invalid --log-format: %s", o.LogFormat))
	}

	return allErrors
}

// AddFlags adds flags related to cluster identity to the options
func (o *ExtraOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ClusterIdentity, "cluster-identity", o.ClusterIdentity, "This flag is used for specifying the identity of the Garden cluster")
	fs.DurationVar(&o.AdminKubeconfigMaxExpiration, "shoot-admin-kubeconfig-max-expiration", time.Hour*24, "The maximum validity duration of a credential requested to a Shoot by an AdminKubeconfigRequest. If an otherwise valid AdminKubeconfigRequest with a validity duration larger than this value is requested, a credential will be issued with a validity duration of this value.")
	fs.DurationVar(&o.ViewerKubeconfigMaxExpiration, "shoot-viewer-kubeconfig-max-expiration", time.Hour*24, "The maximum validity duration of a credential requested to a Shoot by an ViewerKubeconfigRequest. If an otherwise valid ViewerKubeconfigRequest with a validity duration larger than this value is requested, a credential will be issued with a validity duration of this value.")
	fs.DurationVar(&o.CredentialsRotationInterval, "shoot-credentials-rotation-interval", time.Hour*24*90, "The duration after the initial shoot creation or the last credentials rotation when a client warning for the next credentials rotation is issued.")

	fs.StringVar(&o.LogLevel, "log-level", "info", "The level/severity for the logs. Must be one of [info,debug,error]")
	fs.StringVar(&o.LogFormat, "log-format", "json", "The format for the logs. Must be one of [json,text]")
}

// ApplyTo applies the extra options to the API Server config.
func (o *ExtraOptions) ApplyTo(c *Config) error {
	c.ExtraConfig.AdminKubeconfigMaxExpiration = o.AdminKubeconfigMaxExpiration
	c.ExtraConfig.ViewerKubeconfigMaxExpiration = o.ViewerKubeconfigMaxExpiration
	c.ExtraConfig.CredentialsRotationInterval = o.CredentialsRotationInterval

	return nil
}
