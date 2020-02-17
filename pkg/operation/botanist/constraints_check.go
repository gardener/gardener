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

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission/plugin/webhook"
)

func shootHibernatedConstraint(condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "ConstraintNotChecked", "Shoot cluster has been hibernated.")
}

// ConstraintsChecks conducts the constraints checks on all the given constraints.
func (b *Botanist) ConstraintsChecks(ctx context.Context, initializeShootClients func() error, hibernation gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	hibernationPossible := b.constraintsChecks(ctx, initializeShootClients, hibernation)
	return b.pardonCondition(hibernationPossible)
}

func (b *Botanist) constraintsChecks(ctx context.Context, initializeShootClients func() error, hibernationConstraint gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	if b.Shoot.HibernationEnabled || b.Shoot.Info.Status.IsHibernated {
		return shootHibernatedConstraint(hibernationConstraint)
	}

	if err := initializeShootClients(); err != nil {
		message := fmt.Sprintf("Could not initialize Shoot client for constraints check: %+v", err)
		b.Logger.Error(message)
		hibernationConstraint = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(hibernationConstraint, message)

		return hibernationConstraint
	}

	newHibernationConstraint, err := b.CheckHibernationPossible(ctx, hibernationConstraint)
	hibernationConstraint = newConditionOrError(hibernationConstraint, newHibernationConstraint, err)

	return hibernationConstraint
}

// CheckHibernationPossible checks the Shoot for problematic webhooks which could prevent wakeup after hibernation
func (b *Botanist) CheckHibernationPossible(ctx context.Context, constraint gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	validatingWebhookConfigs := &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}
	if err := b.K8sShootClient.Client().List(ctx, validatingWebhookConfigs); err != nil {
		return nil, fmt.Errorf("could not get ValidatingWebhookConfigurations of Shoot cluster to check if Shoot can be hibernated")
	}

	for _, webhookConfig := range validatingWebhookConfigs.Items {
		for i, w := range webhookConfig.Webhooks {
			uid := fmt.Sprintf("%s/%s/%d", webhookConfig.Name, w.Name, i)
			accessor := webhook.NewValidatingWebhookAccessor(uid, webhookConfig.Name, &w)
			if IsProblematicWebhook(accessor) {
				failurePolicy := "nil"
				if w.FailurePolicy != nil {
					failurePolicy = string(*w.FailurePolicy)
				}

				c := gardencorev1beta1helper.UpdatedCondition(constraint, gardencorev1beta1.ConditionFalse, "ProblematicWebhooks",
					fmt.Sprintf("Shoot cannot be hibernated because of ValidatingWebhookConfiguration %q: webhook %q with failurePolicy %q will probably prevent the Shoot from being woken up again",
						webhookConfig.Name, w.Name, failurePolicy))
				return &c, nil
			}
		}
	}

	mutatingWebhookConfigs := &admissionregistrationv1beta1.MutatingWebhookConfigurationList{}
	if err := b.K8sShootClient.Client().List(ctx, mutatingWebhookConfigs); err != nil {
		return nil, fmt.Errorf("could not get MutatingWebhookConfigurations of Shoot cluster to check if Shoot can be hibernated")
	}

	for _, webhookConfig := range mutatingWebhookConfigs.Items {
		for i, w := range webhookConfig.Webhooks {
			uid := fmt.Sprintf("%s/%s/%d", webhookConfig.Name, w.Name, i)
			accessor := webhook.NewMutatingWebhookAccessor(uid, webhookConfig.Name, &w)
			if IsProblematicWebhook(accessor) {
				failurePolicy := "nil"
				if w.FailurePolicy != nil {
					failurePolicy = string(*w.FailurePolicy)
				}

				c := gardencorev1beta1helper.UpdatedCondition(constraint, gardencorev1beta1.ConditionFalse, "ProblematicWebhooks",
					fmt.Sprintf("Shoot cannot be hibernated because of MutatingWebhookConfiguration %q: webhook %q with failurePolicy %q will probably prevent the Shoot from being woken up again",
						webhookConfig.Name, w.Name, failurePolicy))
				return &c, nil
			}
		}
	}

	c := gardencorev1beta1helper.UpdatedCondition(constraint, gardencorev1beta1.ConditionTrue, "NoProblematicWebhooks", "Shoot can be hibernated.")
	return &c, nil
}

// IsProblematicWebhook checks if a single webhook of the Shoot Cluster is problematic and the Shoot should therefore
// not be hibernated. Problematic webhooks are webhooks with rules for CREATE/UPDATE/* pods or nodes and
// failurePolicy=Fail/nil. If the Shoot contains such a webhook, we can never wake up this shoot cluster again
// as new nodes cannot get created/ready, or our system component pods cannot get created/ready
// (because the webhook's backing pod is not yet running).
func IsProblematicWebhook(w webhook.WebhookAccessor) bool {
	if w.GetFailurePolicy() != nil && *w.GetFailurePolicy() != admissionregistrationv1beta1.Fail {
		// in admissionregistration.k8s.io/v1 FailurePolicy is defaulted to `Fail`
		// see https://github.com/kubernetes/api/blob/release-1.16/admissionregistration/v1/types.go#L195
		// and https://github.com/kubernetes/api/blob/release-1.16/admissionregistration/v1/types.go#L324
		// therefore, webhook with FailurePolicy==nil is also considered problematic
		return false
	}

	for _, rule := range w.GetRules() {
		apiGroups := sets.NewString(rule.APIGroups...)
		resources := sets.NewString(rule.Resources...)

		if apiGroups.Has(corev1.GroupName) && resources.HasAny("pods", "nodes") {
			for _, op := range rule.Operations {
				if op == admissionregistrationv1beta1.Create || op == admissionregistrationv1beta1.Update || op == admissionregistrationv1beta1.OperationAll {
					return true
				}
			}
		}
	}

	return false
}
