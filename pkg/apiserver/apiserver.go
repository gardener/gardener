// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/util/keyutil"

	corerest "github.com/gardener/gardener/pkg/apiserver/registry/core/rest"
	operationsrest "github.com/gardener/gardener/pkg/apiserver/registry/operations/rest"
	securityrest "github.com/gardener/gardener/pkg/apiserver/registry/security/rest"
	seedmanagementrest "github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/rest"
	settingsrest "github.com/gardener/gardener/pkg/apiserver/registry/settings/rest"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// ExtraConfig contains non-generic Gardener API server configuration.
type ExtraConfig struct {
	AdminKubeconfigMaxExpiration       time.Duration
	ViewerKubeconfigMaxExpiration      time.Duration
	CredentialsRotationInterval        time.Duration
	WorkloadIdentityTokenIssuer        string
	WorkloadIdentityTokenMinExpiration time.Duration
	WorkloadIdentityTokenMaxExpiration time.Duration
	WorkloadIdentitySigningKey         any
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

	var tokenIssuer workloadidentity.TokenIssuer
	if c.ExtraConfig.WorkloadIdentitySigningKey != nil {
		tokenIssuer, err = workloadidentity.NewTokenIssuer(
			c.ExtraConfig.WorkloadIdentitySigningKey,
			c.ExtraConfig.WorkloadIdentityTokenIssuer,
			int64(c.ExtraConfig.WorkloadIdentityTokenMinExpiration.Seconds()),
			int64(c.ExtraConfig.WorkloadIdentityTokenMaxExpiration.Seconds()),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create WorkloadIdentity token issuer: %w", err)
		}
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
		securityAPIGroupInfo       = (securityrest.StorageProvider{
			TokenIssuer:         tokenIssuer,
			CoreInformerFactory: c.coreInformerFactory,
		}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
	)

	if err := s.GenericAPIServer.InstallAPIGroups(&coreAPIGroupInfo, &settingsAPIGroupInfo, &seedManagementAPIGroupInfo, &operationsAPIGroupInfo, &securityAPIGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// ExtraOptions is used for providing additional options to the Gardener API Server
type ExtraOptions struct {
	ClusterIdentity                    string
	AdminKubeconfigMaxExpiration       time.Duration
	ViewerKubeconfigMaxExpiration      time.Duration
	CredentialsRotationInterval        time.Duration
	WorkloadIdentityTokenIssuer        string
	WorkloadIdentityTokenMinExpiration time.Duration
	WorkloadIdentityTokenMaxExpiration time.Duration
	WorkloadIdentitySigningKeyFile     string

	LogLevel  string
	LogFormat string
}

// Validate checks if the required flags are set
func (o *ExtraOptions) Validate() []error {
	allErrors := []error{}
	if len(o.ClusterIdentity) == 0 {
		allErrors = append(allErrors, errors.New("--cluster-identity must be specified"))
	}

	if o.AdminKubeconfigMaxExpiration < time.Hour ||
		o.AdminKubeconfigMaxExpiration > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, errors.New("--shoot-admin-kubeconfig-max-expiration must be between 1 hour and 2^32 seconds"))
	}

	if o.ViewerKubeconfigMaxExpiration < time.Hour ||
		o.ViewerKubeconfigMaxExpiration > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, errors.New("--shoot-viewer-kubeconfig-max-expiration must be between 1 hour and 2^32 seconds"))
	}

	if o.CredentialsRotationInterval < 24*time.Hour ||
		o.CredentialsRotationInterval > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, errors.New("--shoot-credentials-rotation-interval must be between 24 hours and 2^32 seconds"))
	}

	if len(o.WorkloadIdentityTokenIssuer) != 0 {
		if _, err := url.Parse(o.WorkloadIdentityTokenIssuer); err != nil {
			allErrors = append(allErrors, fmt.Errorf("--workload-identity-token-issuer is not a valid URL, err: %w", err))
		}
	}
	if o.WorkloadIdentityTokenMinExpiration < 10*time.Minute ||
		o.WorkloadIdentityTokenMinExpiration > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, errors.New("--workload-identity-token-min-expiration must be between 10 minutes and 2^32 seconds"))
	}

	if o.WorkloadIdentityTokenMaxExpiration < 10*time.Minute ||
		o.WorkloadIdentityTokenMaxExpiration > time.Duration(1<<32)*time.Second {
		allErrors = append(allErrors, errors.New("--workload-identity-token-max-expiration must be between 10 minutes and 2^32 seconds"))
	}

	if len(o.WorkloadIdentitySigningKeyFile) != 0 {
		if _, err := keyutil.PrivateKeyFromFile(o.WorkloadIdentitySigningKeyFile); err != nil {
			allErrors = append(allErrors, fmt.Errorf("--workload-identity-signing-key-file does not contain valid key, err: %w", err))
		}
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
	fs.StringVar(&o.WorkloadIdentityTokenIssuer, "workload-identity-token-issuer", o.WorkloadIdentityTokenIssuer, "The issuer identifier of the workload identity tokens set in the 'iss' claim. If set, it must be a valid URL")
	fs.DurationVar(&o.WorkloadIdentityTokenMinExpiration, "workload-identity-token-min-expiration", time.Hour, "The minimum validity duration of a workload identity token. If an otherwise valid TokenRequest with a validity duration less than this value is requested, a token will be issued with a validity duration of this value.")
	fs.DurationVar(&o.WorkloadIdentityTokenMaxExpiration, "workload-identity-token-max-expiration", time.Hour*48, "The maximum validity duration of a workload identity token. If an otherwise valid TokenRequest with a validity duration greater than this value is requested, a token will be issued with a validity duration of this value.")
	fs.StringVar(&o.WorkloadIdentitySigningKeyFile, "workload-identity-signing-key-file", o.WorkloadIdentitySigningKeyFile, "Path to the file that contains the current private key of the workload identity token issuer. The issuer will sign issued ID tokens with this private key.")

	fs.StringVar(&o.LogLevel, "log-level", "info", "The level/severity for the logs. Must be one of [info,debug,error]")
	fs.StringVar(&o.LogFormat, "log-format", "json", "The format for the logs. Must be one of [json,text]")
}

// ApplyTo applies the extra options to the API Server config.
func (o *ExtraOptions) ApplyTo(c *Config) error {
	c.ExtraConfig.AdminKubeconfigMaxExpiration = o.AdminKubeconfigMaxExpiration
	c.ExtraConfig.ViewerKubeconfigMaxExpiration = o.ViewerKubeconfigMaxExpiration
	c.ExtraConfig.CredentialsRotationInterval = o.CredentialsRotationInterval
	c.ExtraConfig.WorkloadIdentityTokenIssuer = o.WorkloadIdentityTokenIssuer
	c.ExtraConfig.WorkloadIdentityTokenMinExpiration = o.WorkloadIdentityTokenMinExpiration
	c.ExtraConfig.WorkloadIdentityTokenMaxExpiration = o.WorkloadIdentityTokenMaxExpiration

	if len(o.WorkloadIdentitySigningKeyFile) != 0 {
		signingKey, err := keyutil.PrivateKeyFromFile(o.WorkloadIdentitySigningKeyFile)
		if err != nil {
			return fmt.Errorf("failed to get WorkloadIdentity signing key from file %q: %w", o.WorkloadIdentitySigningKeyFile, err)
		}
		c.ExtraConfig.WorkloadIdentitySigningKey = signingKey
	}

	return nil
}
