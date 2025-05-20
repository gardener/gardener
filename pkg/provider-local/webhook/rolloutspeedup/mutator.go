// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rolloutspeedup

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mutator struct {
	client client.Client
}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	deployment, ok := newObj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("expected deployment, got %T", newObj)
	}

	// 3 and 5 are the magic numbers 🪄
	if deployment.Spec.MinReadySeconds > 5 {
		deployment.Spec.MinReadySeconds = 5
	}
	deployment.Spec.Template.Spec.TerminationGracePeriodSeconds = ptr.To[int64](3)

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.ReadinessProbe != nil {
			deployment.Spec.Template.Spec.Containers[i].ReadinessProbe.InitialDelaySeconds = 3
			deployment.Spec.Template.Spec.Containers[i].ReadinessProbe.PeriodSeconds = 5
		}
	}

	return nil
}
