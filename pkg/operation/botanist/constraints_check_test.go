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

package botanist_test

import (
	"github.com/gardener/gardener/pkg/operation/botanist"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission/plugin/webhook"
)

var _ = Describe("constraints checks", func() {
	Context("HibernationPossible", func() {
		type webhookTestCase struct {
			failurePolicy *admissionregistrationv1beta1.FailurePolicyType
			operationType admissionregistrationv1beta1.OperationType
			groupResource schema.GroupResource
		}

		var (
			failurePolicyIgnore = admissionregistrationv1beta1.Ignore
			failurePolicyFail   = admissionregistrationv1beta1.Fail

			operationCreate = admissionregistrationv1beta1.Create
			operationUpdate = admissionregistrationv1beta1.Update
			operationAll    = admissionregistrationv1beta1.OperationAll
			operationDelete = admissionregistrationv1beta1.Delete

			groupResourcePods  = corev1.Resource("pods")
			groupResourceNodes = corev1.Resource("nodes")
			groupResourceOther = corev1.Resource("other")

			problematicWebhookTestCase = webhookTestCase{
				failurePolicy: &failurePolicyFail,
				operationType: operationCreate,
				groupResource: groupResourcePods,
			}
		)

		DescribeTable("#IsProblematicWebhook",
			func(testCase webhookTestCase, expected bool) {
				var (
					w = admissionregistrationv1beta1.MutatingWebhook{
						Name:          "foo-webhook",
						FailurePolicy: testCase.failurePolicy,
						Rules: []admissionregistrationv1beta1.RuleWithOperations{
							{
								Operations: []admissionregistrationv1beta1.OperationType{testCase.operationType},
								Rule: admissionregistrationv1beta1.Rule{
									APIGroups: []string{testCase.groupResource.Group},
									Resources: []string{testCase.groupResource.Resource},
								},
							},
						},
					}
					accessor = webhook.NewMutatingWebhookAccessor("test-uid", "test-cfg", &w)
				)

				isProblematic := botanist.IsProblematicWebhook(accessor)
				Expect(isProblematic).To(Equal(expected))
			},
			Entry("Problematic Webhook for CREATE pods",
				problematicWebhookTestCase,
				true,
			),
			Entry("Problematic Webhook with failurePolicy nil for CREATE pods",
				webhookTestCase{
					failurePolicy: nil,
					operationType: problematicWebhookTestCase.operationType,
					groupResource: problematicWebhookTestCase.groupResource,
				},
				true,
			),
			Entry("Problematic Webhook for UPDATE pods",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: operationUpdate,
					groupResource: problematicWebhookTestCase.groupResource,
				},
				true,
			),
			Entry("Problematic Webhook for * pods",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: operationAll,
					groupResource: problematicWebhookTestCase.groupResource,
				},
				true,
			),
			Entry("Problematic Webhook for CREATE nodes",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: problematicWebhookTestCase.operationType,
					groupResource: groupResourceNodes,
				},
				true,
			),
			Entry("Problematic Webhook for UPDATE nodes",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: operationUpdate,
					groupResource: groupResourceNodes,
				},
				true,
			),
			Entry("Problematic Webhook for * nodes",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: operationAll,
					groupResource: groupResourceNodes,
				},
				true,
			),
			Entry("Unproblematic Webhook because of failurePolicy 'Ignore'",
				webhookTestCase{
					failurePolicy: &failurePolicyIgnore,
					operationType: problematicWebhookTestCase.operationType,
					groupResource: problematicWebhookTestCase.groupResource,
				},
				false,
			),
			Entry("Unproblematic Webhook because of operationType 'DELETE'",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: operationDelete,
					groupResource: problematicWebhookTestCase.groupResource,
				},
				false,
			),
			Entry("Unproblematic Webhook because of resource 'other'",
				webhookTestCase{
					failurePolicy: problematicWebhookTestCase.failurePolicy,
					operationType: problematicWebhookTestCase.operationType,
					groupResource: groupResourceOther,
				},
				false,
			),
		)
	})
})
