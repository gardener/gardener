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

package machinecontrollermanager

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
)

type mutator struct{}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	if newObj.GetName() != "system:machine-controller-manager-runtime" {
		return nil
	}

	clusterRole, ok := newObj.(*rbacv1.ClusterRole)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *rbacv1.ClusterRole", newObj)
	}

	for _, rule := range clusterRole.Rules {
		if utils.ValueExists("", rule.APIGroups) &&
			utils.ValueExists("services", rule.Resources) &&
			utils.ValueExists("*", rule.Verbs) {
			return nil
		}
	}

	clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"services"},
		Verbs:     []string{"*"},
	})
	return nil
}
