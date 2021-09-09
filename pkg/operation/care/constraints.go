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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/matchers"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WebhookMaximumTimeoutSecondsNotProblematic is the maximum timeout in seconds a webhooks on critical resources can
// have in order to not be considered as a problematic webhook by the constraints checks. Any webhook on critical
// resources with a larger timeout is considered to be problematic.
const WebhookMaximumTimeoutSecondsNotProblematic = 15

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

// Check checks all given constraints.
func (c *Constraint) Check(
	ctx context.Context,
	constraints []gardencorev1beta1.Condition,
) []gardencorev1beta1.Condition {
	updatedConstraints := c.constraintsChecks(ctx, constraints)
	lastOp := c.shoot.GetInfo().Status.LastOperation
	lastErrors := c.shoot.GetInfo().Status.LastErrors
	return PardonConditions(updatedConstraints, lastOp, lastErrors)
}

func (c *Constraint) constraintsChecks(
	ctx context.Context,
	constraints []gardencorev1beta1.Condition,
) []gardencorev1beta1.Condition {
	if c.shoot.HibernationEnabled || c.shoot.GetInfo().Status.IsHibernated {
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

func getValidatingWebhookConfigurations(ctx context.Context, client client.Client, k8sVersion *semver.Version) ([]admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	if version.ConstraintK8sLessEqual115.Check(k8sVersion) {
		l := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"apiVersion": admissionregistrationv1beta1.SchemeGroupVersion.String(),
				"kind":       "ValidatingWebhookConfigurationList",
			},
		}

		if err := client.List(ctx, l); err != nil && !meta.IsNoMatchError(err) {
			return nil, err
		}

		var webhookConfigs []admissionregistrationv1.ValidatingWebhookConfiguration
		if err := meta.EachListItem(l, func(obj runtime.Object) error {
			u, _ := obj.(*unstructured.Unstructured)
			// Set APIVersion to v1 as the target version. We can transform v1beta1 directly to v1 because both APIs are identical.
			u.SetGroupVersionKind(admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration"))

			webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if err := kubernetes.ShootScheme.Convert(u, webhookConfig, nil); err != nil {
				return err
			}

			webhookConfigs = append(webhookConfigs, *webhookConfig)

			return nil
		}); err != nil {
			return nil, err
		}

		return webhookConfigs, nil
	}

	validatingWebhookConfigs := &admissionregistrationv1.ValidatingWebhookConfigurationList{}
	if err := client.List(ctx, validatingWebhookConfigs); err != nil {
		return nil, err
	}
	return validatingWebhookConfigs.Items, nil
}

func getMutatingWebhookConfigurations(ctx context.Context, client client.Client, k8sVersion *semver.Version) ([]admissionregistrationv1.MutatingWebhookConfiguration, error) {
	if version.ConstraintK8sLessEqual115.Check(k8sVersion) {
		l := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"apiVersion": admissionregistrationv1beta1.SchemeGroupVersion.String(),
				"kind":       "MutatingWebhookConfigurationList",
			},
		}

		if err := client.List(ctx, l); err != nil && !meta.IsNoMatchError(err) {
			return nil, err
		}

		var webhookConfigs []admissionregistrationv1.MutatingWebhookConfiguration
		if err := meta.EachListItem(l, func(obj runtime.Object) error {
			u, _ := obj.(*unstructured.Unstructured)
			// Set APIVersion to v1 as the target version. We can transform v1beta1 directly to v1 because both APIs are identical.
			u.SetGroupVersionKind(admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingWebhookConfiguration"))

			webhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{}
			if err := kubernetes.ShootScheme.Convert(u, webhookConfig, nil); err != nil {
				return err
			}

			webhookConfigs = append(webhookConfigs, *webhookConfig)

			return nil
		}); err != nil {
			return nil, err
		}
		return webhookConfigs, nil
	}

	mutatingWebhookConfigs := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	if err := client.List(ctx, mutatingWebhookConfigs); err != nil {
		return nil, err
	}
	return mutatingWebhookConfigs.Items, nil
}

// CheckForProblematicWebhooks checks the Shoot for problematic webhooks which could prevent shoot worker nodes from
// joining the cluster.
func (c *Constraint) CheckForProblematicWebhooks(ctx context.Context) (gardencorev1beta1.ConditionStatus, string, string, error) {
	validatingWebhookConfigs, err := getValidatingWebhookConfigurations(ctx, c.shootClient, c.shoot.KubernetesVersion)
	if err != nil {
		return "", "", "", fmt.Errorf("could not get ValidatingWebhookConfigurations of Shoot cluster to check for problematic webhooks: %w", err)
	}

	for _, webhookConfig := range validatingWebhookConfigs {
		for _, w := range webhookConfig.Webhooks {
			if IsProblematicWebhook(w.FailurePolicy, w.ObjectSelector, w.NamespaceSelector, w.Rules, w.TimeoutSeconds) {
				return gardencorev1beta1.ConditionFalse,
					"ProblematicWebhooks",
					buildProblematicWebhookMessage("ValidatingWebhookConfiguration", webhookConfig.Name, w.Name, w.FailurePolicy, w.TimeoutSeconds),
					nil
			}
		}
	}

	mutatingWebhookConfigs, err := getMutatingWebhookConfigurations(ctx, c.shootClient, c.shoot.KubernetesVersion)
	if err != nil {
		return "", "", "", fmt.Errorf("could not get MutatingWebhookConfigurations of Shoot cluster to check for problematic webhooks: %w", err)
	}

	for _, webhookConfig := range mutatingWebhookConfigs {
		for _, w := range webhookConfig.Webhooks {
			if IsProblematicWebhook(w.FailurePolicy, w.ObjectSelector, w.NamespaceSelector, w.Rules, w.TimeoutSeconds) {
				return gardencorev1beta1.ConditionFalse,
					"ProblematicWebhooks",
					buildProblematicWebhookMessage("MutatingWebhookConfiguration", webhookConfig.Name, w.Name, w.FailurePolicy, w.TimeoutSeconds),
					nil
			}
		}
	}

	return gardencorev1beta1.ConditionTrue,
		"NoProblematicWebhooks",
		"All webhooks are properly configured.",
		nil
}

func buildProblematicWebhookMessage(
	kind string,
	configName string,
	webhookName string,
	failurePolicy *admissionregistrationv1.FailurePolicyType,
	timeoutSeconds *int32,
) string {
	failurePolicyString := "nil"
	if failurePolicy != nil {
		failurePolicyString = string(*failurePolicy)
	}
	timeoutString := "and unset timeoutSeconds"
	if timeoutSeconds != nil {
		timeoutString = fmt.Sprintf("and %ds timeout", *timeoutSeconds)
	}
	return fmt.Sprintf("%s %q is problematic: webhook %q with failurePolicy %q %s might prevent worker nodes from properly joining the shoot cluster",
		kind, configName, webhookName, failurePolicyString, timeoutString)
}

// IsProblematicWebhook checks if a single webhook of the Shoot Cluster is problematic. Problematic webhooks are
// webhooks with rules for CREATE/UPDATE/* pods or nodes and failurePolicy=Fail/nil. If the Shoot contains such a
// webhook, we can never wake up this shoot cluster again as new nodes cannot get created/ready, or our system
// component pods cannot get created/ready (because the webhook's backing pod is not yet running).
func IsProblematicWebhook(
	failurePolicy *admissionregistrationv1.FailurePolicyType,
	objSelector *metav1.LabelSelector,
	nsSelector *metav1.LabelSelector,
	rules []admissionregistrationv1.RuleWithOperations,
	timeoutSeconds *int32,
) bool {
	if failurePolicy != nil && *failurePolicy != admissionregistrationv1.Fail {
		// in admissionregistration.k8s.io/v1 FailurePolicy is defaulted to `Fail`
		// see https://github.com/kubernetes/api/blob/release-1.16/admissionregistration/v1/types.go#L195
		// and https://github.com/kubernetes/api/blob/release-1.16/admissionregistration/v1/types.go#L324
		// therefore, webhook with FailurePolicy==nil is also considered problematic
		if timeoutSeconds != nil && *timeoutSeconds <= WebhookMaximumTimeoutSecondsNotProblematic {
			// most control-plane API calls are made with a client-side timeout of 30s, so if a webhook has
			// timeoutSeconds==30 the overall request might still fail although failurePolicy==Ignore, as there
			// is overhead in communication with the API server and possible other webhooks.
			// in admissionregistration/v1 timeoutSeconds is defaulted to 10 while in v1beta1 it's defaulted to 30.
			// be restrictive here and mark all webhooks without a timeout set or timeouts > 15s as problematic to
			// avoid ops effort. It's clearly documented that users should specify low timeouts, see
			// https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#timeouts
			return false
		}
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
