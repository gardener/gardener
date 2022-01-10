// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

// GenerateEncryptionConfiguration generates a missing etcd encryption configuration for the Gardener API server
func (o *operation) GenerateEncryptionConfiguration(_ context.Context) error {
	if o.imports.GardenerAPIServer.ComponentConfiguration.Encryption == nil {
		config, err := generateEncryptionConfiguration()
		if err != nil {
			return fmt.Errorf("failed to generate encryption configuration for Gardener API server: %w", err)
		}
		o.log.Infof("Successfully generated new etcd encryption configuration for the Gardener API Server")
		o.imports.GardenerAPIServer.ComponentConfiguration.Encryption = config
	}

	return nil
}

func generateEncryptionConfiguration() (*apiserverconfigv1.EncryptionConfiguration, error) {
	config := &etcdencryption.EncryptionConfig{}
	if err := config.AddNewEncryptionKey(); err != nil {
		return nil, err
	}

	encryptionConfiguration := etcdencryption.NewEncryptionConfiguration(config)

	// add virtual garden specific resources
	// this is a safe slice access, as we know that there must be one resource configuration
	encryptionConfiguration.Resources[0].Resources = append(encryptionConfiguration.Resources[0].Resources,
		fmt.Sprintf("controllerdeployments.%s", gardencorev1beta1.GroupName),
		fmt.Sprintf("controllerregistrations.%s", gardencorev1beta1.GroupName),
		fmt.Sprintf("shootstates.%s", gardencorev1beta1.GroupName),
	)
	return encryptionConfiguration, nil
}
