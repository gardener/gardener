// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
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
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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
	timeoutSeconds = w.timeoutSeconds
	rules = []admissionregistrationv1.RuleWithOperations{{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{w.gvr.Group},
			Resources:   []string{w.gvr.Resource},
			APIVersions: []string{w.gvr.Version},
		}},
	}

	opType := admissionregistrationv1.OperationAll
	if w.operationType != nil {
		opType = *w.operationType
	}

	rules[0].Operations = []admissionregistrationv1.OperationType{opType}
	return
}

var _ = Describe("Constraints", func() {
	var (
		clock clock.Clock
		now   = time.Date(2022, 2, 22, 22, 22, 22, 0, time.UTC)
	)

	BeforeEach(func() {
		clock = testclock.NewFakeClock(now)
	})

	Describe("#IsProblematicWebhook", func() {
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
				Entry("namespaceSelector matching name label", webhookTestCase{
					namespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"}},
				}),
				Entry("namespaceSelector matching all gardener labels", webhookTestCase{
					namespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
							"gardener.cloud/purpose":          "kube-system",
							"kubernetes.io/metadata.name":     "kube-system",
						}},
				}),
			}

			kubeSystemNamespaceNotProblematic = []TableEntry{
				Entry("not matching namespaceSelector", webhookTestCase{
					namespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"}},
				}),
				Entry("namespaceSelector excluding name label", webhookTestCase{
					namespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "kubernetes.io/metadata.name",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"kube-system"},
							},
						},
					},
				}),
			}

			commonTests = func(gvr schema.GroupVersionResource, problematic, notProblematic []TableEntry) {
				if gvr.Resource == "leases" || (gvr.Group == "*" && gvr.Resource == "*") {
					problematic = append(problematic,
						Entry("failurePolicy 'Ignore' and timeoutSeconds ok", webhookTestCase{failurePolicy: &failurePolicyIgnore, timeoutSeconds: &timeoutSecondsNotProblematic}))
				} else {
					notProblematic = append(notProblematic,
						Entry("failurePolicy 'Ignore' and timeoutSeconds ok", webhookTestCase{failurePolicy: &failurePolicyIgnore, timeoutSeconds: &timeoutSecondsNotProblematic, objectSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/name": "test",
							}}}),
						Entry("failurePolicy 'Ignore' and timeoutSeconds ok", webhookTestCase{failurePolicy: &failurePolicyIgnore, timeoutSeconds: &timeoutSecondsNotProblematic}))
				}

				DescribeTable(fmt.Sprintf("problematic webhook for %s", gvr.String()),
					func(testCase webhookTestCase) {
						testCase.gvr = gvr
						Expect(IsProblematicWebhook(testCase.build())).To(BeTrue(), "expected webhook to be problematic")
					},
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
					problematic,
				)

				DescribeTable(fmt.Sprintf("not problematic webhook for %s", gvr.String()),
					func(testCase webhookTestCase) {
						testCase.gvr = gvr
						Expect(IsProblematicWebhook(testCase.build())).To(BeFalse(), "expected webhook not to be problematic")
					},
					Entry("operationType 'DELETE'", webhookTestCase{operationType: &operationDelete}),
					notProblematic,
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

			namespacesTestTables = func(gvr schema.GroupVersionResource) {
				var (
					problematic = []TableEntry{
						Entry("namespaceSelector matching purpose", webhookTestCase{
							namespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"gardener.cloud/purpose": "kube-system"}},
						}),
						Entry("objectSelector matching purpose", webhookTestCase{
							objectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"gardener.cloud/purpose": "kube-system"}},
						}),
					}
					notProblematic = []TableEntry{
						Entry("namespaceSelector not matching purpose", webhookTestCase{
							namespaceSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: "gardener.cloud/purpose", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
								},
							},
						}),
						Entry("not matching namespaceSelector", webhookTestCase{
							namespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"}},
						}),
						Entry("objectSelector not matching purpose", webhookTestCase{
							objectSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: "gardener.cloud/purpose", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
								},
							},
						}),
						Entry("not matching objectSelector", webhookTestCase{
							objectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"}},
						}),
						Entry("matching objectSelector, not matching namespaceSelector", webhookTestCase{
							objectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"gardener.cloud/purpose": "kube-system"}},
							namespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"}},
						}),
						Entry("matching namespaceSelector, not matching objectSelector", webhookTestCase{
							objectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"}},
							namespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"gardener.cloud/purpose": "kube-system"}},
						}),
					}
				)

				commonTests(gvr, problematic, notProblematic)
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
		namespacesTestTables(corev1.SchemeGroupVersion.WithResource("namespaces"))
		namespacesTestTables(corev1.SchemeGroupVersion.WithResource("namespaces/status"))

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

		// there are no non-problematic webhooks expected for leases because
		// Gardener considers all leases in all namespaces
		commonTests(coordinationv1.SchemeGroupVersion.WithResource("leases"), kubeSystemNamespaceProblematic, nil)
		withoutSelectorsTables(coordinationv1.SchemeGroupVersion.WithResource("leases"))
		withoutSelectorsTables(coordinationv1beta1.SchemeGroupVersion.WithResource("leases"))

		kubeSystemNamespaceTables(networkingv1.SchemeGroupVersion.WithResource("networkpolicies"))
		kubeSystemNamespaceTables(networkingv1beta1.SchemeGroupVersion.WithResource("networkpolicies"))

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

			Expect(IsProblematicWebhook(wh.build())).To(BeFalse())
		})
	})

	Describe("Constraint", func() {
		var (
			ctx                   = context.Background()
			controlPlaneNamespace = "shoot--foo--bar"
			seedClient            client.Client
			shootClient           client.Client

			constraint *Constraint

			newCASecret = func(validUntilTime time.Time) *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "some-secret-",
						Namespace:    controlPlaneNamespace,
						Labels: map[string]string{
							"managed-by":       "secrets-manager",
							"manager-identity": "gardenlet",
							"persist":          "true",
							"valid-until-time": strconv.FormatInt(validUntilTime.Unix(), 10),
						},
					},
					Data: map[string][]byte{"ca.crt": []byte(""), "ca.key": []byte("")},
				}
			}
		)

		BeforeEach(func() {
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			shoot := &shootpkg.Shoot{
				ControlPlaneNamespace: controlPlaneNamespace,
			}
			shoot.SetInfo(&gardencorev1beta1.Shoot{})

			constraint = NewConstraint(
				logr.Discard(),
				shoot,
				seedClient,
				func() (kubernetes.Interface, bool, error) {
					return fakekubernetes.NewClientSetBuilder().WithClient(shootClient).Build(), true, nil
				},
				clock,
			)
		})

		Describe("#Check", func() {
			var (
				shoot = &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						Constraints: []gardencorev1beta1.Condition{
							{Type: gardencorev1beta1.ShootHibernationPossible},
							{Type: gardencorev1beta1.ShootMaintenancePreconditionsSatisfied},
							{Type: gardencorev1beta1.ShootCRDsWithProblematicConversionWebhooks},
						},
					},
				}

				constraints = NewShootConstraints(testclock.NewFakeClock(time.Time{}), shoot)
			)

			It("should remove the 'CACertificateValiditiesAcceptable' constraint because it's true", func() {
				Expect(constraint.Check(ctx, constraints)).NotTo(ContainCondition(
					OfType(gardencorev1beta1.ShootCACertificateValiditiesAcceptable),
				))
			})

			It("should keep the 'CACertificateValiditiesAcceptable' constraint because it's false (before pardoned)", func() {
				Expect(seedClient.Create(ctx, newCASecret(now))).To(Succeed())

				Expect(constraint.Check(ctx, constraints)).To(ContainCondition(
					OfType(gardencorev1beta1.ShootCACertificateValiditiesAcceptable),
					WithStatus(gardencorev1beta1.ConditionProgressing),
					WithReason("ExpiringCACertificates"),
				))
			})

			It("should not keep the `CRDsWithProblematicConversionWebhooks` condition when it's true", func() {
				Expect(constraint.Check(ctx, constraints)).NotTo(ContainCondition(
					OfType(gardencorev1beta1.ShootCRDsWithProblematicConversionWebhooks),
				))
			})

			It("should keep the `CRDsWithProblematicConversionWebhooks` condition when it's false", func() {
				crd1 := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sample1.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Conversion: &apiextensionsv1.CustomResourceConversion{
							Strategy: "Webhook",
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						StoredVersions: []string{"v1", "v1beta1"},
					},
				}
				crd2 := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sample2.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Conversion: &apiextensionsv1.CustomResourceConversion{
							Strategy: "None",
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						StoredVersions: []string{"v1"},
					},
				}
				Expect(shootClient.Create(ctx, crd1)).To(Succeed())
				Expect(shootClient.Create(ctx, crd2)).To(Succeed())

				Expect(constraint.Check(ctx, constraints)).To(ContainCondition(
					OfType(gardencorev1beta1.ShootCRDsWithProblematicConversionWebhooks),
					WithStatus(gardencorev1beta1.ConditionProgressing),
					WithReason("CRDsWithProblematicConversionWebhooks"),
					WithMessage(fmt.Sprintf("Some CRDs in your cluster have multiple stored versions present and have a conversion webhook configured: %s.", crd1.Name)),
				))
			})
		})

		Describe("#CheckIfCACertificateValiditiesAcceptable", func() {
			var (
				expectTrueCondition = func(status gardencorev1beta1.ConditionStatus, reason, message string, errorCodes []gardencorev1beta1.ErrorCode) {
					Expect(status).To(Equal(gardencorev1beta1.ConditionTrue))
					Expect(reason).To(Equal("NoExpiringCACertificates"))
					Expect(message).To(Equal("All CA certificates are still valid for at least 8760h0m0s."))
					Expect(errorCodes).To(BeNil())
				}
				expectFalseCondition = func(status gardencorev1beta1.ConditionStatus, reason, message string, errorCodes []gardencorev1beta1.ErrorCode, expectedMessage string) {
					Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
					Expect(reason).To(Equal("ExpiringCACertificates"))
					Expect(message).To(Equal("Some CA certificates are expiring in less than 8760h0m0s, you should rotate them: " + expectedMessage))
					Expect(errorCodes).To(BeNil())
				}
			)

			It("should return a 'true' condition when there are no secrets", func() {
				status, reason, message, errorCodes, err := constraint.CheckIfCACertificateValiditiesAcceptable(ctx)
				Expect(err).NotTo(HaveOccurred())
				expectTrueCondition(status, reason, message, errorCodes)
			})

			It("should return a 'true' condition when there are no CA secrets", func() {
				secret := newCASecret(now.Add(time.Second))
				secret.Data = nil
				Expect(seedClient.Create(ctx, secret)).To(Succeed())

				status, reason, message, errorCodes, err := constraint.CheckIfCACertificateValiditiesAcceptable(ctx)
				Expect(err).NotTo(HaveOccurred())
				expectTrueCondition(status, reason, message, errorCodes)
			})

			It("should return a 'true' condition when there are only CA secrets valid long enough", func() {
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*4)))).To(Succeed())
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*3)))).To(Succeed())
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*2)))).To(Succeed())

				status, reason, message, errorCodes, err := constraint.CheckIfCACertificateValiditiesAcceptable(ctx)
				Expect(err).NotTo(HaveOccurred())
				expectTrueCondition(status, reason, message, errorCodes)
			})

			It("should return a 'false' condition when there are CA secrets not valid long enough", func() {
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*4)))).To(Succeed())
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*3)))).To(Succeed())
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*2)))).To(Succeed())
				Expect(seedClient.Create(ctx, newCASecret(now.Add(24*time.Hour*365*1)))).To(Succeed())
				Expect(seedClient.Create(ctx, newCASecret(now))).To(Succeed())

				status, reason, message, errorCodes, err := constraint.CheckIfCACertificateValiditiesAcceptable(ctx)
				Expect(err).NotTo(HaveOccurred())
				expectFalseCondition(status, reason, message, errorCodes, fmt.Sprintf(`"" (expiring at %s)`, now.String()))
			})

			It("should return an error when the valid-until-time label cannot be parsed", func() {
				secret := newCASecret(now)
				secret.Labels["valid-until-time"] = "unparsable"
				Expect(seedClient.Create(ctx, secret)).To(Succeed())

				status, reason, message, errorCodes, err := constraint.CheckIfCACertificateValiditiesAcceptable(ctx)
				Expect(err).To(MatchError(ContainSubstring("could not parse valid-until-time label from secret")))
				Expect(status).To(BeEmpty())
				Expect(reason).To(BeEmpty())
				Expect(message).To(BeEmpty())
				Expect(errorCodes).To(BeNil())
			})
		})
	})

	Describe("ShootConstraints", func() {
		Describe("#NewShootConstraints", func() {
			It("should initialize all constraints", func() {
				constraints := NewShootConstraints(clock, &gardencorev1beta1.Shoot{})

				Expect(constraints.ConvertToSlice()).To(ConsistOf(
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})

			It("should only initialize missing constraints", func() {
				hibernationPossibleConstraint := gardencorev1beta1.Condition{Type: "HibernationPossible"}
				constraints := NewShootConstraints(clock, &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						Constraints: []gardencorev1beta1.Condition{
							hibernationPossibleConstraint,
							{Type: "Foo"},
						},
					},
				})

				Expect(constraints.ConvertToSlice()).To(ConsistOf(
					hibernationPossibleConstraint,
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusAndMsg("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})
		})

		Describe("#ConvertToSlice", func() {
			It("should return the expected conditions", func() {
				constraints := NewShootConstraints(clock, &gardencorev1beta1.Shoot{})

				Expect(constraints.ConvertToSlice()).To(HaveExactElements(
					OfType("HibernationPossible"),
					OfType("MaintenancePreconditionsSatisfied"),
					OfType("CACertificateValiditiesAcceptable"),
					OfType("CRDsWithProblematicConversionWebhooks"),
				))
			})
		})

		Describe("#ConstraintTypes", func() {
			It("should return the expected condition types", func() {
				constraints := NewShootConstraints(clock, &gardencorev1beta1.Shoot{})

				Expect(constraints.ConstraintTypes()).To(HaveExactElements(
					gardencorev1beta1.ConditionType("HibernationPossible"),
					gardencorev1beta1.ConditionType("MaintenancePreconditionsSatisfied"),
					gardencorev1beta1.ConditionType("CACertificateValiditiesAcceptable"),
					gardencorev1beta1.ConditionType("CRDsWithProblematicConversionWebhooks"),
				))
			})
		})
	})
})
