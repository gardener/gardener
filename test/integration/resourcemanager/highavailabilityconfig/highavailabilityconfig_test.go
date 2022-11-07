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

package highavailabilityconfig_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("HighAvailabilityConfig tests", func() {
	var (
		namespace   *corev1.Namespace
		deployment  *appsv1.Deployment
		statefulSet *appsv1.StatefulSet

		labels = map[string]string{"foo": "bar"}
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Replicas: pointer.Int32(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "foo-container",
							Image: "foo",
						}},
					},
				},
			},
		}

		statefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Replicas: pointer.Int32(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "foo-container",
							Image: "foo",
						}},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Namespace")
		Expect(testClient.Create(ctx, namespace)).To(Succeed())
		log.Info("Created Namespace", "namespaceName", namespace.Name)

		DeferCleanup(func() {
			By("Delete Namespace")
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())
			log.Info("Deleted Namespace", "namespaceName", namespace.Name)
		})
	})

	tests := func(
		getObj func() client.Object,
		getReplicas func() *int32,
		setReplicas func(*int32),
		getPodSpec func() corev1.PodSpec,
		setPodSpec func(func(*corev1.PodSpec)),
	) {
		Context("when namespace is not labeled with consider=true", func() {
			It("should not mutate anything", func() {
				Expect(getReplicas()).To(PointTo(Equal(int32(1))))
				Expect(getPodSpec().Affinity).To(BeNil())
				Expect(getPodSpec().TopologySpreadConstraints).To(BeEmpty())
			})
		})

		Context("when namespace is labeled with consider=true", func() {
			BeforeEach(func() {
				metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			})
		})
	}

	Context("for deployments", func() {
		JustBeforeEach(func() {
			By("Create Deployment")
			Expect(testClient.Create(ctx, deployment)).To(Succeed())
		})

		tests(
			func() client.Object { return deployment },
			func() *int32 { return deployment.Spec.Replicas },
			func(replicas *int32) { deployment.Spec.Replicas = replicas },
			func() corev1.PodSpec { return deployment.Spec.Template.Spec },
			func(mutate func(spec *corev1.PodSpec)) { mutate(&deployment.Spec.Template.Spec) },
		)
	})

	Context("for statefulsets", func() {
		JustBeforeEach(func() {
			By("Create StatefulSet")
			Expect(testClient.Create(ctx, statefulSet)).To(Succeed())
		})

		tests(
			func() client.Object { return statefulSet },
			func() *int32 { return statefulSet.Spec.Replicas },
			func(replicas *int32) { statefulSet.Spec.Replicas = replicas },
			func() corev1.PodSpec { return statefulSet.Spec.Template.Spec },
			func(mutate func(spec *corev1.PodSpec)) { mutate(&statefulSet.Spec.Template.Spec) },
		)
	})
})
