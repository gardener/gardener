// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"context"
	"fmt"
	"slices"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		if slices.Contains(rule.APIGroups, "") &&
			slices.Contains(rule.Resources, "services") &&
			slices.Contains(rule.Verbs, "*") {
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
