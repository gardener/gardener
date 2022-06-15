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

package care

import (
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation"
	webhookmatchers "github.com/gardener/gardener/pkg/operation/botanist/matchers"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/sirupsen/logrus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WebhookRemediation contains required information for shoot webhook remediation.
type WebhookRemediation struct {
	logger                 logrus.FieldLogger
	initializeShootClients ShootClientInit
	shoot                  *gardencorev1beta1.Shoot
	seedNamespace          string
}

// NewWebhookRemediation creates a new instance for webhook remediation.
func NewWebhookRemediation(op *operation.Operation, shootClientInit ShootClientInit) *WebhookRemediation {
	return &WebhookRemediation{
		logger:                 op.Logger,
		initializeShootClients: shootClientInit,
		shoot:                  op.Shoot.GetInfo(),
		seedNamespace:          op.Shoot.SeedNamespace,
	}
}

// Remediate mutates shoot webhooks not following the best practices documented by Kubernetes.
func (r *WebhookRemediation) Remediate(ctx context.Context) error {
	shootClient, apiServerRunning, err := r.initializeShootClients()
	if err != nil {
		return err
	}
	if !apiServerRunning {
		return nil
	}

	var (
		logger = r.logger.WithField("shoot", client.ObjectKeyFromObject(r.shoot))
		fns    []flow.TaskFn

		notExcluded   = utils.MustNewRequirement(v1beta1constants.LabelExcludeWebhookFromRemediation, selection.NotIn, "true")
		labelSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(notExcluded)}
	)

	validatingWebhookConfigs := &admissionregistrationv1.ValidatingWebhookConfigurationList{}
	if err := shootClient.Client().List(ctx, validatingWebhookConfigs, labelSelector); err != nil {
		return fmt.Errorf("could not get ValidatingWebhookConfigurations of Shoot cluster to remediate problematic webhooks: %w", err)
	}

	for _, config := range validatingWebhookConfigs.Items {
		if strings.Contains(config.Annotations[resourcesv1alpha1.OriginAnnotation], r.seedNamespace) {
			continue
		}

		var (
			webhookConfig = config.DeepCopy()
			mustPatch     bool
			patch         = client.StrategicMergeFrom(webhookConfig.DeepCopy())
			remediations  []string
		)

		for i, w := range webhookConfig.Webhooks {
			remediate := newRemediator(logger, "ValidatingWebhookConfiguration", webhookConfig.Name, w.Name, &remediations)

			if mustRemediateTimeoutSeconds(w.TimeoutSeconds) {
				mustPatch = true
				webhookConfig.Webhooks[i].TimeoutSeconds = remediate.timeoutSeconds()
			}

			if w.FailurePolicy != nil && *w.FailurePolicy == admissionregistrationv1.Ignore {
				return nil
			}

			matchers := getMatchingRules(w.Rules, w.ObjectSelector, w.NamespaceSelector)

			if mustRemediateFailurePolicy(matchers) {
				mustPatch = true
				webhookConfig.Webhooks[i].FailurePolicy = remediate.failurePolicy()
			}

			if mustRemediateSelectors(matchers) {
				mustPatch = true
				objectSelector, namespaceSelector := remediate.selectors(matchers)
				webhookConfig.Webhooks[i].ObjectSelector = extendSelector(webhookConfig.Webhooks[i].ObjectSelector, objectSelector...)
				webhookConfig.Webhooks[i].NamespaceSelector = extendSelector(webhookConfig.Webhooks[i].NamespaceSelector, namespaceSelector...)
			}
		}

		if mustPatch {
			fns = append(fns, newPatchFunc(shootClient.Client(), webhookConfig, patch, remediations))
		}
	}

	mutatingWebhookConfigs := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	if err := shootClient.Client().List(ctx, mutatingWebhookConfigs, labelSelector); err != nil {
		return fmt.Errorf("could not get MutatingWebhookConfigurations of Shoot cluster to remediate problematic webhooks: %w", err)
	}

	for _, config := range mutatingWebhookConfigs.Items {
		if strings.Contains(config.Annotations[resourcesv1alpha1.OriginAnnotation], r.seedNamespace) {
			continue
		}

		var (
			webhookConfig = config.DeepCopy()
			mustPatch     bool
			patch         = client.StrategicMergeFrom(webhookConfig.DeepCopy())
			remediations  []string
		)

		for i, w := range webhookConfig.Webhooks {
			remediate := newRemediator(logger, "MutatingWebhookConfiguration", webhookConfig.Name, w.Name, &remediations)

			if mustRemediateTimeoutSeconds(w.TimeoutSeconds) {
				mustPatch = true
				webhookConfig.Webhooks[i].TimeoutSeconds = remediate.timeoutSeconds()
			}

			if w.FailurePolicy != nil && *w.FailurePolicy == admissionregistrationv1.Ignore {
				return nil
			}

			matchers := getMatchingRules(w.Rules, w.ObjectSelector, w.NamespaceSelector)

			if mustRemediateFailurePolicy(matchers) {
				mustPatch = true
				webhookConfig.Webhooks[i].FailurePolicy = remediate.failurePolicy()
			}

			if mustRemediateSelectors(matchers) {
				mustPatch = true
				objectSelector, namespaceSelector := remediate.selectors(matchers)
				webhookConfig.Webhooks[i].ObjectSelector = extendSelector(webhookConfig.Webhooks[i].ObjectSelector, objectSelector...)
				webhookConfig.Webhooks[i].NamespaceSelector = extendSelector(webhookConfig.Webhooks[i].NamespaceSelector, namespaceSelector...)
			}
		}

		if mustPatch {
			fns = append(fns, newPatchFunc(shootClient.Client(), webhookConfig, patch, remediations))
		}
	}

	return flow.Parallel(fns...)(ctx)
}

func getMatchingRules(
	rules []admissionregistrationv1.RuleWithOperations,
	objectSelector, namespaceSelector *metav1.LabelSelector,
) []webhookmatchers.WebhookConstraintMatcher {
	var matchers []webhookmatchers.WebhookConstraintMatcher
	for _, rule := range rules {
		for _, matcher := range webhookmatchers.WebhookConstraintMatchers {
			if matcher.Match(rule, objectSelector, namespaceSelector) {
				matchers = append(matchers, matcher)
			}
		}
	}
	return matchers
}

func mustRemediateTimeoutSeconds(timeoutSeconds *int32) bool {
	return timeoutSeconds == nil || *timeoutSeconds > WebhookMaximumTimeoutSecondsNotProblematic
}

func mustRemediateSelectors(matchers []webhookmatchers.WebhookConstraintMatcher) bool {
	return len(matchers) > 0
}

func mustRemediateFailurePolicy(matchers []webhookmatchers.WebhookConstraintMatcher) bool {
	for _, matcher := range matchers {
		return matcher.NamespaceLabels == nil && matcher.ObjectLabels == nil
	}
	return false
}

type remediator struct {
	logger            logrus.FieldLogger
	webhookConfigKind string
	webhookConfigName string
	webhookName       string
	remediations      *[]string
}

func newRemediator(logger logrus.FieldLogger, webhookConfigKind, webhookConfigName, webhookName string, remediations *[]string) remediator {
	return remediator{
		logger: logger.WithFields(logrus.Fields{
			"kind":    webhookConfigKind,
			"name":    webhookConfigName,
			"webhook": webhookName,
		}),
		webhookConfigKind: webhookConfigKind,
		webhookConfigName: webhookConfigName,
		webhookName:       webhookName,
		remediations:      remediations,
	}
}

func (r *remediator) timeoutSeconds() *int32 {
	var timeoutSeconds int32 = WebhookMaximumTimeoutSecondsNotProblematic
	r.reportf("timeoutSeconds", "set to %d", timeoutSeconds)
	return pointer.Int32(timeoutSeconds)
}

func (r *remediator) selectors(matchers []webhookmatchers.WebhookConstraintMatcher) (objectSelector, namespaceSelector []metav1.LabelSelectorRequirement) {
	for _, matcher := range matchers {
		for k, v := range matcher.ObjectLabels {
			objectSelector = append(objectSelector, newNotInLabelSelectorRequirement(k, v))
		}
		for k, v := range matcher.NamespaceLabels {
			namespaceSelector = append(namespaceSelector, newNotInLabelSelectorRequirement(k, v))
		}
	}

	objectSelector = removeDuplicateRequirements(objectSelector)
	namespaceSelector = removeDuplicateRequirements(namespaceSelector)

	if len(objectSelector) > 0 {
		r.reportf("objectSelector", "extended with %s", objectSelector)
	}
	if len(namespaceSelector) > 0 {
		r.reportf("namespaceSelector", "extended with %s", namespaceSelector)
	}

	return
}

func (r *remediator) failurePolicy() *admissionregistrationv1.FailurePolicyType {
	ignore := admissionregistrationv1.Ignore
	r.reportf("failurePolicy", "set to %s", ignore)
	return &ignore
}

func (r *remediator) reportf(fieldName string, messageFmt string, args ...interface{}) {
	r.logger.Infof("Remediating %s", fieldName)
	*r.remediations = append(*r.remediations, fmt.Sprintf("%s of webhook %q was %s", fieldName, r.webhookName, fmt.Sprintf(messageFmt, args...)))
}

func newPatchFunc(shootClient client.Client, webhookConfig client.Object, patch client.Patch, remediations []string) func(context.Context) error {
	annotations := webhookConfig.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}

	annotations[v1beta1constants.GardenerWarning] = "ATTENTION: This webhook configuration has been modified by " +
		"Gardener since it does not follow the best practices recommended by Kubernetes " +
		"(https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings) " +
		"and might interfere with the cluster operations. Please make sure to follow these recommendations to prevent " +
		"future interventions. When you are done, please remove this annotation. See also " +
		"https://github.com/gardener/gardener/blob/master/docs/usage/shoot_status.md#constraints for further information.\n" +
		"The following modifications have been made:\n" +
		strings.Join(addHyphenPrefix(remediations), "\n")
	webhookConfig.SetAnnotations(annotations)

	return func(ctx context.Context) error {
		return shootClient.Patch(ctx, webhookConfig, patch)
	}
}

func addHyphenPrefix(list []string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		out = append(out, "- "+v)
	}
	return out
}

func newNotInLabelSelectorRequirement(key, value string) metav1.LabelSelectorRequirement {
	return metav1.LabelSelectorRequirement{
		Key:      key,
		Operator: metav1.LabelSelectorOpNotIn,
		Values:   []string{value},
	}
}

func removeDuplicateRequirements(requirements []metav1.LabelSelectorRequirement) []metav1.LabelSelectorRequirement {
	var (
		keyValues   = sets.NewString()
		keyValuesID = func(requirement metav1.LabelSelectorRequirement) string {
			return requirement.Key + strings.Join(requirement.Values, "")
		}

		out []metav1.LabelSelectorRequirement
	)

	for _, requirement := range requirements {
		id := keyValuesID(requirement)

		if keyValues.Has(id) {
			continue
		}

		out = append(out, requirement)
		keyValues.Insert(id)
	}

	return out
}

func extendSelector(selector *metav1.LabelSelector, requirements ...metav1.LabelSelectorRequirement) *metav1.LabelSelector {
	if selector == nil {
		selector = &metav1.LabelSelector{}
	}

	selector.MatchExpressions = append(selector.MatchExpressions, requirements...)
	return selector
}
