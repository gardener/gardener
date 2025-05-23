// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
)

// WaitForExtensionCondition waits for the extension to contain the condition type, status and reason
func WaitForExtensionCondition(ctx context.Context, log logr.Logger, seedClient client.Client, groupVersionKind schema.GroupVersionKind, namespacedName types.NamespacedName, conditionType gardencorev1beta1.ConditionType, conditionStatus gardencorev1beta1.ConditionStatus, conditionReason string) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		rawExtension := unstructured.Unstructured{}
		rawExtension.SetGroupVersionKind(groupVersionKind)

		log = log.WithValues(
			"objectKey", namespacedName,
			"gvk", groupVersionKind,
		)

		if err := seedClient.Get(ctx, namespacedName, &rawExtension); err != nil {
			log.Error(err, "Unable to retrieve extension from seed")
			return retry.MinorError(fmt.Errorf("unable to retrieve extension from seed (ns: %s, name: %s, kind %s)", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind))
		}

		acc, err := extensions.Accessor(rawExtension.DeepCopyObject())
		if err != nil {
			return retry.MinorError(err)
		}

		for _, condition := range acc.GetExtensionStatus().GetConditions() {
			log.Info("Extension has condition", "condition", condition)
			if condition.Type == conditionType && condition.Status == conditionStatus && condition.Reason == conditionReason {
				log.Info("Found expected conditions")
				return retry.Ok()
			}
		}
		log.Info("Extension does not yet contain expected condition", "expectedType", conditionType, "expectedStatus", conditionStatus, "expectedReason", conditionReason)
		return retry.MinorError(fmt.Errorf("extension (ns: %s, name: %s, kind %s) does not yet contain expected condition. EXPECTED: (conditionType: %s, conditionStatus: %s, conditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, conditionType, conditionStatus, conditionReason))
	})
}

// ScaleGardenerResourceManager scales the gardener-resource-manager to the desired replicas
func ScaleGardenerResourceManager(ctx context.Context, namespace string, client client.Client, desiredReplicas *int32) (*int32, error) {
	return framework.ScaleDeployment(ctx, client, desiredReplicas, v1beta1constants.DeploymentNameGardenerResourceManager, namespace)
}
