// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bootstrappers

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SeedConfigChecker checks whether the seed networks in the specification of the provided SeedConfig are correctly
// configured. Note that this only works in case the seed cluster is a shoot cluster (i.e., if it has the `shoot-info`
// ConfigMap in the kube-system namespace).
type SeedConfigChecker struct {
	SeedClient client.Client
	SeedConfig *config.SeedConfig
}

// Start performs the check.
func (s *SeedConfigChecker) Start(ctx context.Context) error {
	if s.SeedConfig == nil {
		return nil
	}

	shootInfo := &corev1.ConfigMap{}
	if err := s.SeedClient.Get(ctx, kubernetes.Key(metav1.NamespaceSystem, constants.ConfigMapNameShootInfo), shootInfo); client.IgnoreNotFound(err) != nil {
		return err
	} else if errors.IsNotFound(err) {
		// Seed cluster does not seem to be managed by Gardener
		return nil
	}

	if podNetwork := shootInfo.Data["podNetwork"]; podNetwork != s.SeedConfig.Spec.Networks.Pods {
		return fmt.Errorf("incorrect pod network specified in seed configuration (cluster=%q vs. config=%q)", podNetwork, s.SeedConfig.Spec.Networks.Pods)
	}

	if serviceNetwork := shootInfo.Data["serviceNetwork"]; serviceNetwork != s.SeedConfig.Spec.Networks.Services {
		return fmt.Errorf("incorrect service network specified in seed configuration (cluster=%q vs. config=%q)", serviceNetwork, s.SeedConfig.Spec.Networks.Services)
	}

	// Be graceful in case the (optional) node network is only available on one side
	if nodeNetwork, exists := shootInfo.Data["nodeNetwork"]; exists &&
		s.SeedConfig.Spec.Networks.Nodes != nil &&
		*s.SeedConfig.Spec.Networks.Nodes != nodeNetwork {
		return fmt.Errorf("incorrect node network specified in seed configuration (cluster=%q vs. config=%q)", nodeNetwork, *s.SeedConfig.Spec.Networks.Nodes)
	}

	return nil
}
