// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/matchers"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func shootHibernatedConstraint(condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "ConstraintNotChecked", "Shoot cluster has been hibernated.")
}

func shootControlPlaneNotRunningConstraint(condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, "ConstraintNotChecked", "Shoot control plane is not running at the moment.")
}

// ConstraintsChecks conducts the constraints checks on all the given constraints.
func (b *Botanist) ConstraintsChecks(
	ctx context.Context,
	initializeShootClients func() (bool, error),
	hibernationPossibleConstraint gardencorev1beta1.Condition,
	maintenancePreconditionsSatisfiedConstraint gardencorev1beta1.Condition,
) (
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
) {
	hibernationPossible, maintenancePreconditionsSatisfied := b.constraintsChecks(ctx, initializeShootClients, hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint)
	lastOp := b.Shoot.Info.Status.LastOperation
	lastErrors := b.Shoot.Info.Status.LastErrors
	return PardonCondition(hibernationPossible, lastOp, lastErrors),
		PardonCondition(maintenancePreconditionsSatisfied, lastOp, lastErrors)
}

func (b *Botanist) constraintsChecks(
	ctx context.Context,
	initializeShootClients func() (bool, error),
	hibernationPossibleConstraint gardencorev1beta1.Condition,
	maintenancePreconditionsSatisfiedConstraint gardencorev1beta1.Condition,
) (
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
) {
	if b.Shoot.HibernationEnabled || b.Shoot.Info.Status.IsHibernated {
		return shootHibernatedConstraint(hibernationPossibleConstraint), shootHibernatedConstraint(maintenancePreconditionsSatisfiedConstraint)
	}

	apiServerRunning, err := initializeShootClients()
	if err != nil {
		message := fmt.Sprintf("Could not initialize Shoot client for constraints check: %+v", err)
		b.Logger.Error(message)
		hibernationPossibleConstraint = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(hibernationPossibleConstraint, message)
		maintenancePreconditionsSatisfiedConstraint = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(maintenancePreconditionsSatisfiedConstraint, message)

		return hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint
	}

	if !apiServerRunning {
		// don't check constraints if API server has already been deleted or has not been created yet
		return shootControlPlaneNotRunningConstraint(hibernationPossibleConstraint),
			shootControlPlaneNotRunningConstraint(maintenancePreconditionsSatisfiedConstraint)
	}

	var newHibernationConstraint, newMaintenancePreconditionsSatisfiedConstraint *gardencorev1beta1.Condition

	status, reason, message, err := b.CheckForProblematicWebhooks(ctx)
	if err == nil {
		updatedHibernationCondition := gardencorev1beta1helper.UpdatedCondition(hibernationPossibleConstraint, status, reason, message)
		newHibernationConstraint = &updatedHibernationCondition

		updatedMaintenanceCondition := gardencorev1beta1helper.UpdatedCondition(maintenancePreconditionsSatisfiedConstraint, status, reason, message)
		newMaintenancePreconditionsSatisfiedConstraint = &updatedMaintenanceCondition
	}

	hibernationPossibleConstraint = newConditionOrError(hibernationPossibleConstraint, newHibernationConstraint, err)
	maintenancePreconditionsSatisfiedConstraint = newConditionOrError(maintenancePreconditionsSatisfiedConstraint, newMaintenancePreconditionsSatisfiedConstraint, err)

	return hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint
}

// CheckForProblematicWebhooks checks the Shoot for problematic webhooks which could prevent shoot worker nodes from
// joining the cluster.
func (b *Botanist) CheckForProblematicWebhooks(ctx context.Context) (gardencorev1beta1.ConditionStatus, string, string, error) {
	validatingWebhookConfigs := &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}
	if err := b.K8sShootClient.Client().List(ctx, validatingWebhookConfigs); err != nil {
		return "", "", "", fmt.Errorf("could not get ValidatingWebhookConfigurations of Shoot cluster to check for problematic webhooks")
	}

	for _, webhookConfig := range validatingWebhookConfigs.Items {
		for _, w := range webhookConfig.Webhooks {
			if IsProblematicWebhook(w.FailurePolicy, w.ObjectSelector, w.NamespaceSelector, w.Rules) {
				failurePolicy := "nil"
				if w.FailurePolicy != nil {
					failurePolicy = string(*w.FailurePolicy)
				}

				return gardencorev1beta1.ConditionFalse,
					"ProblematicWebhooks",
					fmt.Sprintf("ValidatingWebhookConfiguration %q is problematic: webhook %q with failurePolicy %q might prevent worker nodes from properly joining the shoot cluster",
						webhookConfig.Name, w.Name, failurePolicy),
					nil
			}
		}
	}

	mutatingWebhookConfigs := &admissionregistrationv1beta1.MutatingWebhookConfigurationList{}
	if err := b.K8sShootClient.Client().List(ctx, mutatingWebhookConfigs); err != nil {
		return "", "", "", fmt.Errorf("could not get MutatingWebhookConfigurations of Shoot cluster to check for problematic webhooks")
	}

	for _, webhookConfig := range mutatingWebhookConfigs.Items {
		for _, w := range webhookConfig.Webhooks {
			if IsProblematicWebhook(w.FailurePolicy, w.ObjectSelector, w.NamespaceSelector, w.Rules) {
				failurePolicy := "nil"
				if w.FailurePolicy != nil {
					failurePolicy = string(*w.FailurePolicy)
				}

				return gardencorev1beta1.ConditionFalse,
					"ProblematicWebhooks",
					fmt.Sprintf("MutatingWebhookConfiguration %q is problematic: webhook %q with failurePolicy %q might prevent worker nodes from properly joining the shoot cluster",
						webhookConfig.Name, w.Name, failurePolicy),
					nil
			}
		}
	}

	return gardencorev1beta1.ConditionTrue,
		"NoProblematicWebhooks",
		"All webhooks are properly configured.",
		nil
}

// IsProblematicWebhook checks if a single webhook of the Shoot Cluster is problematic. Problematic webhooks are
// webhooks with rules for CREATE/UPDATE/* pods or nodes and failurePolicy=Fail/nil. If the Shoot contains such a
// webhook, we can never wake up this shoot cluster again  as new nodes cannot get created/ready, or our system
// component pods cannot get created/ready (because the webhook's backing pod is not yet running).
func IsProblematicWebhook(
	failurePolicy *admissionregistrationv1beta1.FailurePolicyType,
	objSelector *metav1.LabelSelector,
	nsSelector *metav1.LabelSelector,
	rules []admissionregistrationv1beta1.RuleWithOperations,
) bool {
	if failurePolicy != nil && *failurePolicy != admissionregistrationv1beta1.Fail {
		// in admissionregistration.k8s.io/v1 FailurePolicy is defaulted to `Fail`
		// see https://github.com/kubernetes/api/blob/release-1.16/admissionregistration/v1/types.go#L195
		// and https://github.com/kubernetes/api/blob/release-1.16/admissionregistration/v1/types.go#L324
		// therefore, webhook with FailurePolicy==nil is also considered problematic
		return false
	}

	for _, rule := range rules {
		for _, matcher := range matchers.WebhookConstraintMatchers {
			if matcher.Match(rule, objSelector, nsSelector) {
				return true
			}
		}
	}

	return false
}
