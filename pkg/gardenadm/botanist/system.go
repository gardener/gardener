// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// DeployPriorityClassCritical deploys the gardener-system-critical PriorityClass needed for running
// gardener-resource-manager. This is usually deployed to seed clusters via the gardenlet helm chart. When bootstrapping
// an autonomous shoot cluster with `gardenadm bootstrap` there is no gardenlet. Hence, we need to deploy the
// PriorityClass for gardener-resource-manager manually, as it is not taken over by the seedsystem component (it uses a
// ManagedResource for deploying the seed PriorityClasses).
// Using system-cluster-critical for gardener-resource-manager could also be good enough, but it's very simple to deploy
// the gardener-system-critical PriorityClass, so we can also choose the cleaner way.
func (b *AutonomousBotanist) DeployPriorityClassCritical(ctx context.Context) error {
	priorityClass := &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.PriorityClassNameSeedSystemCritical}}
	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, b.SeedClientSet.Client(), priorityClass, func() error {
		priorityClass.Value = int32(999998950)
		priorityClass.GlobalDefault = false
		priorityClass.Description = "This class is used to ensure that the gardener-resource-manager has a high priority and is not preempted in favor of other pods."
		return nil
	})
	return err
}
