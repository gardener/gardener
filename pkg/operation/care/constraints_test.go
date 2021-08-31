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

package care_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	certificatesv1 "k8s.io/api/certificates/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	coordinationv1 "k8s.io/api/coordination/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacv1alpha1 "k8s.io/api/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	schedulingv1alpha1 "k8s.io/api/scheduling/v1alpha1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"

	"github.com/gardener/gardener/pkg/operation/care"
)

type webhookTestCase struct {
	failurePolicy     *admissionregistrationv1.FailurePolicyType
	operationType     *admissionregistrationv1.OperationType
	gvr               schema.GroupVersionResource
	namespaceSelector *metav1.LabelSelector
	objectSelector    *metav1.LabelSelector
	timeoutSeconds    *int32
}

func (w *webhookTestCase) build() (
	failurePolicy *admissionregistrationv1.FailurePolicyType,
	objSelector *metav1.LabelSelector,
	nsSelector *metav1.LabelSelector,
	rules []admissionregistrationv1.RuleWithOperations,
	timeoutSeconds *int32,
) {
	failurePolicy = w.failurePolicy
	nsSelector = w.namespaceSelector
	objSelector = w.objectSelector
	rules = []admissionregistrationv1.RuleWithOperations{{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{w.gvr.Group},
			Resources:   []string{w.gvr.Resource},
			APIVersions: []string{w.gvr.Version},
		}},
	}
	timeoutSeconds = w.timeoutSeconds

	opType := admissionregistrationv1.OperationAll
	if w.operationType != nil {
		opType = *w.operationType
	}

	rules[0].Operations = []admissionregistrationv1.OperationType{opType}

	return
}

var _ = Describe("#IsProblematicWebhook", func() {
	var (
		failurePolicyIgnore = admissionregistrationv1.Ignore
		failurePolicyFail   = admissionregistrationv1.Fail

		timeoutSecondsNotProblematic int32 = 15
		timeoutSecondsProblematic    int32 = 16

		operationCreate = admissionregistrationv1.Create
		operationUpdate = admissionregistrationv1.Update
		operationAll    = admissionregistrationv1.OperationAll
		operationDelete = admissionregistrationv1.Delete

		kubeSystemNamespaceProblematic = []TableEntry{
			Entry("namespaceSelector matching no-cleanup", webhookTestCase{
				namespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"}},
			}),
			Entry("namespaceSelector matching purpose", webhookTestCase{
				namespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"gardener.cloud/purpose": "kube-system"}},
			}),
			Entry("namespaceSelector matching all gardener labels", webhookTestCase{
				namespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"shoot.gardener.cloud/no-cleanup": "true",
						"gardener.cloud/purpose":          "kube-system",
					}},
			}),
		}

		kubeSystemNamespaceNotProblematic = []TableEntry{
			Entry("not matching namespaceSelector", webhookTestCase{
				namespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"}},
			}),
		}

		commonTests = func(gvr schema.GroupVersionResource, problematic, notProblematic []TableEntry) {
			DescribeTable(fmt.Sprintf("problematic webhook for %s", gvr.String()),
				func(testCase webhookTestCase) {
					testCase.gvr = gvr
					Expect(care.IsProblematicWebhook(testCase.build())).To(BeTrue(), "expected webhook to be problematic")
				},
				append([]TableEntry{
					Entry("CREATE", webhookTestCase{
						failurePolicy:  &failurePolicyFail,
						timeoutSeconds: &timeoutSecondsProblematic,
						operationType:  &operationCreate,
					}),
					Entry("CREATE with nil failurePolicy and nil timeoutSeconds", webhookTestCase{operationType: &operationCreate}),
					Entry("CREATE with nil failurePolicy and timeoutSeconds too high",
						webhookTestCase{operationType: &operationCreate, timeoutSeconds: &timeoutSecondsProblematic}),
					Entry("CREATE with failurePolicy 'Ignore' and nil timeoutSeconds",
						webhookTestCase{failurePolicy: &failurePolicyIgnore, operationType: &operationCreate}),
					Entry("CREATE with failurePolicy 'Ignore' and timeoutSeconds too high",
						webhookTestCase{failurePolicy: &failurePolicyIgnore, operationType: &operationCreate, timeoutSeconds: &timeoutSecondsProblematic}),
					Entry("CREATE with failurePolicy 'Fail' and nil timeoutSeconds",
						webhookTestCase{failurePolicy: &failurePolicyFail, operationType: &operationCreate}),
					Entry("CREATE with failurePolicy 'Fail' and timeoutSeconds ok",
						webhookTestCase{failurePolicy: &failurePolicyFail, operationType: &operationCreate, timeoutSeconds: &timeoutSecondsNotProblematic}),
					Entry("UPDATE", webhookTestCase{operationType: &operationUpdate}),
					Entry("*", webhookTestCase{operationType: &operationAll}),
				}, problematic...)...,
			)

			DescribeTable(fmt.Sprintf("not problematic webhook for %s", gvr.String()),
				func(testCase webhookTestCase) {
					testCase.gvr = gvr
					Expect(care.IsProblematicWebhook(testCase.build())).To(BeFalse(), "expected webhook not to be problematic")
				},
				append([]TableEntry{
					Entry("failurePolicy 'Ignore' and timeoutSeconds ok", webhookTestCase{failurePolicy: &failurePolicyIgnore, timeoutSeconds: &timeoutSecondsNotProblematic}),
					Entry("operationType 'DELETE'", webhookTestCase{operationType: &operationDelete}),
				}, notProblematic...)...,
			)
		}

		podsTestTables = func(gvr schema.GroupVersionResource) {
			commonTests(gvr, append(kubeSystemNamespaceProblematic,
				Entry("objectSelector matching no-cleanup", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"}},
				}),
				Entry("objectSelector matching origin", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"origin": "gardener"}},
				}),
				Entry("objectSelector matching all gardener labels", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
							"origin":                          "gardener",
						}},
				}),
				Entry("objectSelector and namespaceSelector matching all gardener labels", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
							"origin":                          "gardener",
						}},
					namespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
							"gardener.cloud/purpose":          "kube-system",
						}},
				}),
			), append(kubeSystemNamespaceNotProblematic,
				Entry("matching objectSelector, not matching namespaceSelector", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"origin":                          "gardener",
							"shoot.gardener.cloud/no-cleanup": "true",
						}},
					namespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"}},
				}),
				Entry("not matching objectSelector", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"}},
				}),
				Entry("matching namespaceSelector, not matching objectSelector", webhookTestCase{
					objectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"}},
					namespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
							"gardener.cloud/purpose":          "kube-system",
						}},
				}),
			))
		}

		kubeSystemNamespaceTables = func(gvr schema.GroupVersionResource) {
			commonTests(gvr, kubeSystemNamespaceProblematic, kubeSystemNamespaceNotProblematic)
		}

		withoutSelectorsTables = func(gvr schema.GroupVersionResource) {
			commonTests(gvr, []TableEntry{}, []TableEntry{})
		}
	)

	podsTestTables(corev1.SchemeGroupVersion.WithResource("pods"))
	podsTestTables(corev1.SchemeGroupVersion.WithResource("pods/status"))
	kubeSystemNamespaceTables(corev1.SchemeGroupVersion.WithResource("configmaps"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("endpoints"))
	kubeSystemNamespaceTables(corev1.SchemeGroupVersion.WithResource("secrets"))
	kubeSystemNamespaceTables(corev1.SchemeGroupVersion.WithResource("serviceaccounts"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("services"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("services/status"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("nodes"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("nodes/status"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("namespaces"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("namespaces/status"))
	withoutSelectorsTables(corev1.SchemeGroupVersion.WithResource("namespaces/finalize"))

	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("controllerrevisions"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("daemonsets"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("daemonsets/status"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("deployments"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("deployments/scale"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("replicasets"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("replicasets/status"))
	kubeSystemNamespaceTables(appsv1.SchemeGroupVersion.WithResource("replicasets/scale"))

	// don't remove this version if deprecated / removed
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("controllerrevisions"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("daemonsets"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("daemonsets/status"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("deployments"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("deployments/scale"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("replicasets"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("replicasets/status"))
	kubeSystemNamespaceTables(appsv1beta1.SchemeGroupVersion.WithResource("replicasets/scale"))

	// don't remove this version if deprecated / removed
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("controllerrevisions"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("daemonsets"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("daemonsets/status"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("deployments"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("deployments/scale"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("replicasets"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("replicasets/status"))
	kubeSystemNamespaceTables(appsv1beta2.SchemeGroupVersion.WithResource("replicasets/scale"))

	// don't remove this version if deprecated / removed
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("controllerrevisions"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("daemonsets"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("daemonsets/status"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("deployments"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("deployments/scale"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets/status"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("replicasets/scale"))
	kubeSystemNamespaceTables(extensionsv1beta1.SchemeGroupVersion.WithResource("networkpolicies"))
	withoutSelectorsTables(extensionsv1beta1.SchemeGroupVersion.WithResource("podsecuritypolicies"))

	withoutSelectorsTables(coordinationv1.SchemeGroupVersion.WithResource("leases"))
	withoutSelectorsTables(coordinationv1beta1.SchemeGroupVersion.WithResource("leases"))

	kubeSystemNamespaceTables(networkingv1.SchemeGroupVersion.WithResource("networkpolicies"))
	kubeSystemNamespaceTables(networkingv1beta1.SchemeGroupVersion.WithResource("networkpolicies"))

	withoutSelectorsTables(policyv1beta1.SchemeGroupVersion.WithResource("podsecuritypolicies"))

	withoutSelectorsTables(rbacv1.SchemeGroupVersion.WithResource("clusterroles"))
	withoutSelectorsTables(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"))
	kubeSystemNamespaceTables(rbacv1.SchemeGroupVersion.WithResource("roles"))
	kubeSystemNamespaceTables(rbacv1.SchemeGroupVersion.WithResource("rolebindings"))

	withoutSelectorsTables(rbacv1alpha1.SchemeGroupVersion.WithResource("clusterroles"))
	withoutSelectorsTables(rbacv1alpha1.SchemeGroupVersion.WithResource("clusterrolebindings"))
	kubeSystemNamespaceTables(rbacv1alpha1.SchemeGroupVersion.WithResource("roles"))
	kubeSystemNamespaceTables(rbacv1alpha1.SchemeGroupVersion.WithResource("rolebindings"))

	withoutSelectorsTables(rbacv1beta1.SchemeGroupVersion.WithResource("clusterroles"))
	withoutSelectorsTables(rbacv1beta1.SchemeGroupVersion.WithResource("clusterrolebindings"))
	kubeSystemNamespaceTables(rbacv1beta1.SchemeGroupVersion.WithResource("roles"))
	kubeSystemNamespaceTables(rbacv1beta1.SchemeGroupVersion.WithResource("rolebindings"))

	withoutSelectorsTables(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"))
	withoutSelectorsTables(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions/status"))

	withoutSelectorsTables(apiextensionsv1beta1.SchemeGroupVersion.WithResource("customresourcedefinitions"))
	withoutSelectorsTables(apiextensionsv1beta1.SchemeGroupVersion.WithResource("customresourcedefinitions/status"))

	withoutSelectorsTables(apiregistrationv1.SchemeGroupVersion.WithResource("apiservices"))
	withoutSelectorsTables(apiregistrationv1.SchemeGroupVersion.WithResource("apiservices/status"))

	withoutSelectorsTables(apiregistrationv1beta1.SchemeGroupVersion.WithResource("apiservices"))
	withoutSelectorsTables(apiregistrationv1beta1.SchemeGroupVersion.WithResource("apiservices/status"))

	withoutSelectorsTables(certificatesv1.SchemeGroupVersion.WithResource("certificatesigningrequests"))
	withoutSelectorsTables(certificatesv1.SchemeGroupVersion.WithResource("certificatesigningrequests/status"))
	withoutSelectorsTables(certificatesv1.SchemeGroupVersion.WithResource("certificatesigningrequests/approval"))

	withoutSelectorsTables(certificatesv1beta1.SchemeGroupVersion.WithResource("certificatesigningrequests"))
	withoutSelectorsTables(certificatesv1beta1.SchemeGroupVersion.WithResource("certificatesigningrequests/status"))
	withoutSelectorsTables(certificatesv1beta1.SchemeGroupVersion.WithResource("certificatesigningrequests/approval"))

	withoutSelectorsTables(schedulingv1.SchemeGroupVersion.WithResource("priorityclasses"))
	withoutSelectorsTables(schedulingv1alpha1.SchemeGroupVersion.WithResource("priorityclasses"))
	withoutSelectorsTables(schedulingv1beta1.SchemeGroupVersion.WithResource("priorityclasses"))

	withoutSelectorsTables(schema.GroupVersionResource{
		Group:    "*",
		Version:  "*",
		Resource: "*",
	})
	withoutSelectorsTables(schema.GroupVersionResource{
		Group:    "apps",
		Version:  "*",
		Resource: "*",
	})
	withoutSelectorsTables(schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "*",
	})

	It("should not block another resource", func() {
		wh := webhookTestCase{
			failurePolicy: &failurePolicyFail,
			gvr:           schema.GroupVersionResource{Group: "foo", Resource: "bar", Version: "baz"},
			operationType: &operationCreate,
		}

		Expect(care.IsProblematicWebhook(wh.build())).To(BeFalse())
	})
})
