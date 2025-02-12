// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garbagecollector_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Garbage collector tests", func() {
	var (
		resourceName string

		testLabels                = map[string]string{"test-data": "true"}
		garbageCollectableObjects []client.Object
	)

	BeforeEach(func() {
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		garbageCollectableObjects = make([]client.Object, 0, 18)

		for i := 0; i < cap(garbageCollectableObjects)/2; i++ {
			garbageCollectableObjects = append(garbageCollectableObjects,
				&corev1.Secret{ObjectMeta: objectMeta(fmt.Sprintf("%s-secret%d", resourceName, i), testLabels)},
				&corev1.ConfigMap{ObjectMeta: objectMeta(fmt.Sprintf("%s-configmap%d", resourceName, i), testLabels)},
			)
		}
	})

	It("should garbage collect all resources because they are not referenced", func() {
		for _, obj := range garbageCollectableObjects {
			Expect(testClient.Create(ctx, obj)).To(Succeed())
		}

		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels(testLabels))).To(Succeed())
			return secretList.Items
		}).Should(BeEmpty())

		Eventually(func(g Gomega) []corev1.ConfigMap {
			configMapList := &corev1.ConfigMapList{}
			g.Expect(testClient.List(ctx, configMapList, client.InNamespace(testNamespace.Name), client.MatchingLabels(testLabels))).To(Succeed())
			return configMapList.Items
		}).Should(BeEmpty())
	})

	It("should only garbage collect unreferenced resources", func() {
		referencingResources := []client.Object{
			&monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret0",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap8",
					},
				},
				Spec: monitoringv1.PrometheusSpec{
					CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
						RemoteWrite: []monitoringv1.RemoteWriteSpec{
							{
								URL: "example.com",
								BasicAuth: &monitoringv1.BasicAuth{
									Username: corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: resourceName + "-secret0"},
									},
								},
							},
						},
					},
				},
			},

			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret1",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap7",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Template: podTemplateSpec(corev1.RestartPolicyAlways),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},

			&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret2",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap6",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Template: podTemplateSpec(corev1.RestartPolicyAlways),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},

			&appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret3",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap5",
					},
				},
				Spec: appsv1.DaemonSetSpec{
					Template: podTemplateSpec(corev1.RestartPolicyAlways),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},

			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret4",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap4",
					},
				},
				Spec: batchv1.JobSpec{
					Template: podTemplateSpec(corev1.RestartPolicyNever),
				},
			},

			&batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret5",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap3",
					},
				},
				Spec: batchv1.CronJobSpec{
					Schedule: "5 4 * * *",
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: podTemplateSpec(corev1.RestartPolicyNever),
						},
					},
				},
			},

			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret6",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap2",
					},
				},
				Spec: podTemplateSpec(corev1.RestartPolicyAlways).Spec,
			},

			&resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    resourceName + "-secret7",
						"reference.resources.gardener.cloud/configmap-foo": resourceName + "-configmap1",
					},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{Name: resourceName + "-secret7"}},
				},
			},
		}

		for _, obj := range referencingResources {
			Expect(testClient.Create(ctx, obj)).To(Succeed())
		}

		for _, obj := range garbageCollectableObjects {
			Expect(testClient.Create(ctx, obj)).To(Succeed())
		}

		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels(testLabels))).To(Succeed())
			return secretList.Items
		}).Should(And(
			ContainElements(
				withName(resourceName+"-secret0"),
				withName(resourceName+"-secret1"),
				withName(resourceName+"-secret2"),
				withName(resourceName+"-secret3"),
				withName(resourceName+"-secret4"),
				withName(resourceName+"-secret5"),
				withName(resourceName+"-secret6"),
				withName(resourceName+"-secret7"),
			),
			Not(ContainElement(withName(resourceName+"-secret8"))),
		))

		Eventually(func(g Gomega) []corev1.ConfigMap {
			configMapList := &corev1.ConfigMapList{}
			g.Expect(testClient.List(ctx, configMapList, client.InNamespace(testNamespace.Name), client.MatchingLabels(testLabels))).To(Succeed())
			return configMapList.Items
		}).Should(And(
			Not(ContainElement(withName(resourceName+"-configmap0"))),
			ContainElements(
				withName(resourceName+"-configmap1"),
				withName(resourceName+"-configmap2"),
				withName(resourceName+"-configmap3"),
				withName(resourceName+"-configmap4"),
				withName(resourceName+"-configmap5"),
				withName(resourceName+"-configmap6"),
				withName(resourceName+"-configmap7"),
				withName(resourceName+"-configmap8"),
			),
		))
	})
})

func withName(name string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Name": Equal(name),
		}),
	})
}

func objectMeta(name string, testLabels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: testNamespace.Name,
		Labels: utils.MergeStringMaps(testLabels, map[string]string{
			"resources.gardener.cloud/garbage-collectable-reference": "true",
		}),
	}
}

func podTemplateSpec(restartPolicy corev1.RestartPolicy) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"foo": "bar"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "foo",
				Image: "bar",
			}},
			RestartPolicy: restartPolicy,
		},
	}
}
