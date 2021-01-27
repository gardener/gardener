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

	corerest "github.com/gardener/gardener/pkg/registry/core/rest"
	seedmanagementrest "github.com/gardener/gardener/pkg/registry/seedmanagement/rest"
	settingsrest "github.com/gardener/gardener/pkg/registry/settings/rest"

	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

type ExtraConfig struct {
	// Place you custom config here.
}

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
		s = &GardenerServer{GenericAPIServer: genericServer}

		coreAPIGroupInfo           = (corerest.StorageProvider{}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
		seedManagementAPIGroupInfo = (seedmanagementrest.StorageProvider{}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
		settingsAPIGroupInfo       = (settingsrest.StorageProvider{}).NewRESTStorage(c.GenericConfig.RESTOptionsGetter)
	)

	if err := s.GenericAPIServer.InstallAPIGroups(&coreAPIGroupInfo, &settingsAPIGroupInfo, &seedManagementAPIGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// ExtraOptions is used for providing additional options to the Gardener API Server
type ExtraOptions struct {
	ClusterIdentity string
}

// Validate checks if the required flags are set
func (o *ExtraOptions) Validate() []error {
	allErrors := []error{}
	if len(o.ClusterIdentity) == 0 {
		allErrors = append(allErrors, fmt.Errorf("--cluster-identity must be specified"))
	}

	return allErrors
}

// AddFlags adds flags related to cluster identity to the options
func (o *ExtraOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ClusterIdentity, "cluster-identity", o.ClusterIdentity, "This flag is used for specifying the identity of the Garden cluster")
}
