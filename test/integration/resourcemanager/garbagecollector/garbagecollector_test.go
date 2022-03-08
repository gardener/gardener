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

package garbagecollector_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Garbage collector tests", func() {
	var (
		testLabels                = map[string]string{"test-data": "true"}
		garbageCollectableObjects []client.Object
	)

	BeforeEach(func() {
		garbageCollectableObjects = make([]client.Object, 0, 14)

		for i := 0; i < cap(garbageCollectableObjects)/2; i++ {
			garbageCollectableObjects = append(garbageCollectableObjects,
				&corev1.Secret{ObjectMeta: objectMeta(fmt.Sprintf("secret%d", i), testLabels)},
				&corev1.ConfigMap{ObjectMeta: objectMeta(fmt.Sprintf("configmap%d", i), testLabels)},
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
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    "secret0",
						"reference.resources.gardener.cloud/configmap-foo": "configmap6",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Template: podTemplateSpec(corev1.RestartPolicyAlways),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},

			&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    "secret1",
						"reference.resources.gardener.cloud/configmap-foo": "configmap5",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Template: podTemplateSpec(corev1.RestartPolicyAlways),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},

			&appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    "secret2",
						"reference.resources.gardener.cloud/configmap-foo": "configmap4",
					},
				},
				Spec: appsv1.DaemonSetSpec{
					Template: podTemplateSpec(corev1.RestartPolicyAlways),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},

			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    "secret3",
						"reference.resources.gardener.cloud/configmap-foo": "configmap3",
					},
				},
				Spec: batchv1.JobSpec{
					Template: podTemplateSpec(corev1.RestartPolicyNever),
				},
			},

			&batchv1beta1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    "secret4",
						"reference.resources.gardener.cloud/configmap-foo": "configmap2",
					},
				},
				Spec: batchv1beta1.CronJobSpec{
					Schedule: "5 4 * * *",
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: podTemplateSpec(corev1.RestartPolicyNever),
						},
					},
				},
			},

			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: testNamespace.Name,
					Annotations: map[string]string{
						"reference.resources.gardener.cloud/secret-foo":    "secret5",
						"reference.resources.gardener.cloud/configmap-foo": "configmap1",
					},
				},
				Spec: podTemplateSpec(corev1.RestartPolicyAlways).Spec,
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
		}).Should(onlyContainObject(withName("secret6")))

		Eventually(func(g Gomega) []corev1.ConfigMap {
			configMapList := &corev1.ConfigMapList{}
			g.Expect(testClient.List(ctx, configMapList, client.InNamespace(testNamespace.Name), client.MatchingLabels(testLabels))).To(Succeed())
			return configMapList.Items
		}).Should(onlyContainObject(withName("configmap0")))
	})
})

func onlyContainObject(matchers ...gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return ContainElements(And(matchers...))
}

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
