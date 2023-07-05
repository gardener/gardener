// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewPlutono returns a deployer for the plutono.
func NewPlutono(
	c client.Client,
	namespace string,
	imageVector imagevector.ImageVector,
	secretsManager secretsmanager.Interface,
	authSecretName string,
	clusterType component.ClusterType,
	ingressHost string,
	includeIstioDashboards, isGardenletManagesMCMbool, isNodeLocalDNSEnabled, isWorkerless, isVPNHighAvailabilityEnabled bool,
	replicas int32,
	wildcardCertName *string,
	vpaEnabled bool,
) (plutono.Interface, error) {
	plutonoImage, err := imageVector.FindImage(images.ImageNamePlutono)
	if err != nil {
		return nil, err
	}

	return plutono.New(
		c,
		namespace,
		secretsManager,
		plutono.Values{
			AuthSecretName:               authSecretName,
			ClusterType:                  clusterType,
			Image:                        plutonoImage.String(),
			IngressHost:                  ingressHost,
			IncludeIstioDashboards:       includeIstioDashboards,
			IsGardenletManagesMCM:        isGardenletManagesMCMbool,
			IsNodeLocalDNSEnabled:        isNodeLocalDNSEnabled,
			IsWorkerless:                 isWorkerless,
			IsVPNHighAvailabilityEnabled: isVPNHighAvailabilityEnabled,
			Replicas:                     replicas,
			WildcardCertName:             wildcardCertName,
			VPAEnabled:                   vpaEnabled,
		},
	), nil
}
