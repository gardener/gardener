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

package apiserver

import (
	"fmt"
	"time"

	corerest "github.com/gardener/gardener/pkg/registry/core/rest"
	operationsrest "github.com/gardener/gardener/pkg/registry/operations/rest"
	seedmanagementrest "github.com/gardener/gardener/pkg/registry/seedmanagement/rest"
	settingsrest "github.com/gardener/gardener/pkg/registry/settings/rest"

	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// ExtraConfig contains non-generic Gardener API server configuration.
type ExtraConfig struct {
	AdminKubeconfigMaxExpiration time.Duration
}

// Config contains Gardener API server configuration.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// GardenerServer contains state for a Gardener API server.
type GardenerServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
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
			AdminKubeconfigMaxExpiration: c.ExtraConfig.AdminKubeconfigMaxExpiration,
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
	ClusterIdentity              string
	AdminKubeconfigMaxExpiration time.Duration
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

	return allErrors
}

// AddFlags adds flags related to cluster identity to the options
func (o *ExtraOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ClusterIdentity, "cluster-identity", o.ClusterIdentity, "This flag is used for specifying the identity of the Garden cluster")
	fs.DurationVar(&o.AdminKubeconfigMaxExpiration, "shoot-admin-kubeconfig-max-expiration", time.Hour*24, "The maximum validity duration of a credential requested to a Shoot by an AdminKubeconfigRequest. If an otherwise valid AdminKubeconfigRequest with a validity duration larger than this value is requested, a credential will be issued with a validity duration of this value.")
}

// ApplyTo applies the extra options to the API Server config.
func (o *ExtraOptions) ApplyTo(c *Config) error {
	c.ExtraConfig.AdminKubeconfigMaxExpiration = o.AdminKubeconfigMaxExpiration

	return nil
}
