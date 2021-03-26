// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package clusteridentity

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type shootClusterIdentity struct {
	clusterIdentity       *string
	gardenClusterIdentity string
	seedNamespace         string
	namespace             string
	name                  string
	uid                   string
	gardenClient          client.Client
	seedClient            client.Client
	logger                *logrus.Entry
}

// New creates new instance of Deployer for Shoot cluster identity
func New(
	clusterIdentity *string,
	gardenClusterIdentity string,
	name, namespace, seedNamespace, uid string,
	gardenClient, seedClient client.Client,
	logger *logrus.Entry,
) component.Deployer {
	return &shootClusterIdentity{
		clusterIdentity:       clusterIdentity,
		gardenClusterIdentity: gardenClusterIdentity,
		name:                  name,
		namespace:             namespace,
		uid:                   uid,
		seedNamespace:         seedNamespace,
		gardenClient:          gardenClient,
		seedClient:            seedClient,
		logger:                logger,
	}
}

func (c *shootClusterIdentity) Deploy(ctx context.Context) error {
	if c.clusterIdentity == nil {
		clusterIdentity := fmt.Sprintf("%s-%s-%s", c.seedNamespace, c.uid, c.gardenClusterIdentity)
		shoot := &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.name,
				Namespace: c.namespace,
			},
		}

		patch := []byte(fmt.Sprintf(`{"status": {"clusterIdentity": "%s"}}`, clusterIdentity))
		if err := c.gardenClient.Status().Patch(ctx, shoot, client.RawPatch(types.StrategicMergePatchType, patch)); err != nil {
			return err
		}

		if err := c.gardenClient.Get(ctx, kutil.Key(c.namespace, c.name), shoot); err != nil {
			return err
		}

		return extensions.SyncClusterResourceToSeed(ctx, c.seedClient, c.seedNamespace, shoot, nil, nil)
	}
	return nil
}

// Destroy returns nil
func (c *shootClusterIdentity) Destroy(ctx context.Context) error {
	return nil
}
