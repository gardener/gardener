// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/operation"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/sirupsen/logrus"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/matchers"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func shootHibernatedConstraints(conditions ...gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	hibernationConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		hibernationConditions = append(hibernationConditions, gardencorev1beta1helper.UpdatedCondition(cond, gardencorev1beta1.ConditionTrue, "ConstraintNotChecked", "Shoot cluster has been hibernated."))
	}
	return hibernationConditions
}

func shootControlPlaneNotRunningConstraints(conditions ...gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	constraints := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		constraints = append(constraints, gardencorev1beta1helper.UpdatedCondition(cond, gardencorev1beta1.ConditionFalse, "ConstraintNotChecked", "Shoot control plane is not running at the moment."))
	}
	return constraints
}

// Constraint contains required information for shoot constraint checks.
type Constraint struct {
	shoot *shoot.Shoot

	initializeShootClients ShootClientInit
	shootClient            client.Client

	logger logrus.FieldLogger
}

// NewConstraint returns a new constraint instance.
func NewConstraint(op *operation.Operation, shootClientInit ShootClientInit) *Constraint {
	return &Constraint{
		shoot:                  op.Shoot,
		initializeShootClients: shootClientInit,
		logger:                 op.Logger,
	}
}

// ConstraintsChecks conducts the constraints checks on all the given constraints.
func (c *Constraint) ConstraintsChecks(
	ctx context.Context,
	constraints []gardencorev1beta1.Condition,
) []gardencorev1beta1.Condition {
	updatedConstrataints := c.constraintsChecks(ctx, constraints)
	lastOp := c.shoot.Info.Status.LastOperation
	lastErrors := c.shoot.Info.Status.LastErrors
	return PardonConditions(updatedConstrataints, lastOp, lastErrors)
}

func (c *Constraint) constraintsChecks(
	ctx context.Context,
	constraints []gardencorev1beta1.Condition,
) []gardencorev1beta1.Condition {
	if c.shoot.HibernationEnabled || c.shoot.Info.Status.IsHibernated {
		return shootHibernatedConstraints(constraints...)
	}

	var hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint gardencorev1beta1.Condition
	for _, cons := range constraints {
		switch cons.Type {
		case gardencorev1beta1.ShootHibernationPossible:
			hibernationPossibleConstraint = cons
		case gardencorev1beta1.ShootMaintenancePreconditionsSatisfied:
			maintenancePreconditionsSatisfiedConstraint = cons
		}
	}

	client, apiServerRunning, err := c.initializeShootClients()
	if err != nil {
		message := fmt.Sprintf("Could not initialize Shoot client for constraints check: %+v", err)
		c.logger.Error(message)
		hibernationPossibleConstraint = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(hibernationPossibleConstraint, message)
		maintenancePreconditionsSatisfiedConstraint = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(maintenancePreconditionsSatisfiedConstraint, message)

		return []gardencorev1beta1.Condition{hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint}
	}

	if !apiServerRunning {
		// don't check constraints if API server has already been deleted or has not been created yet
		return shootControlPlaneNotRunningConstraints(hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint)
	}

	c.shootClient = client.Client()

	var newHibernationConstraint, newMaintenancePreconditionsSatisfiedConstraint *gardencorev1beta1.Condition

	status, reason, message, err := c.CheckForProblematicWebhooks(ctx)
	if err == nil {
		updatedHibernationCondition := gardencorev1beta1helper.UpdatedCondition(hibernationPossibleConstraint, status, reason, message)
		newHibernationConstraint = &updatedHibernationCondition

		updatedMaintenanceCondition := gardencorev1beta1helper.UpdatedCondition(maintenancePreconditionsSatisfiedConstraint, status, reason, message)
		newMaintenancePreconditionsSatisfiedConstraint = &updatedMaintenanceCondition
	}

	hibernationPossibleConstraint = NewConditionOrError(hibernationPossibleConstraint, newHibernationConstraint, err)
	maintenancePreconditionsSatisfiedConstraint = NewConditionOrError(maintenancePreconditionsSatisfiedConstraint, newMaintenancePreconditionsSatisfiedConstraint, err)

	return []gardencorev1beta1.Condition{hibernationPossibleConstraint, maintenancePreconditionsSatisfiedConstraint}
}

// CheckForProblematicWebhooks checks the Shoot for problematic webhooks which could prevent shoot worker nodes from
// joining the cluster.
func (c *Constraint) CheckForProblematicWebhooks(ctx context.Context) (gardencorev1beta1.ConditionStatus, string, string, error) {
	validatingWebhookConfigs := &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}
	if err := c.shootClient.List(ctx, validatingWebhookConfigs); err != nil {
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
	if err := c.shootClient.List(ctx, mutatingWebhookConfigs); err != nil {
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
