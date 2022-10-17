// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("scale", func() {
	var (
		ctrl          *gomock.Controller
		runtimeClient *mockclient.MockClient
		fakeClient    client.Client
		namespace     *corev1.Namespace
		ctx           = context.TODO()
		key           = client.ObjectKey{Name: "foo", Namespace: "bar"}
		statefullSet  = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		statefullSetWith2Replicas = appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 2,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointer.Int32Ptr(2),
			},
			Status: appsv1.StatefulSetStatus{
				ObservedGeneration: 2,
				Replicas:           2,
				AvailableReplicas:  2,
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		runtimeClient = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("ScaleStatefulSet", func() {
		It("sets scale to 2", func() {
			runtimeClient.EXPECT().Patch(context.TODO(), statefullSet, getPatch(2))
			Expect(ScaleStatefulSet(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale succeeds")
		})
	})

	Context("ScaleDeployment", func() {
		It("sets scale to 2", func() {
			runtimeClient.EXPECT().Patch(context.TODO(), deployment, getPatch(2))
			Expect(ScaleDeployment(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale succeeds")
		})
	})

	Describe("#WaitUntilDeploymentScaledToDesiredReplicas", func() {
		It("should wait until deployment was scaled", func() {
			runtimeClient.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, deploy *appsv1.Deployment, _ ...client.GetOption) error {
					*deploy = appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Generation: 2,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32Ptr(2),
						},
						Status: appsv1.DeploymentStatus{
							ObservedGeneration: 2,
							Replicas:           2,
							AvailableReplicas:  2,
						},
					}
					return nil
				})
			Expect(WaitUntilDeploymentScaledToDesiredReplicas(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale done")
		})
	})

	Describe("#WaitUntilStatefulSetScaledToDesiredReplicas", func() {
		It("should wait until statefulset was scaled", func() {
			runtimeClient.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, deploy *appsv1.StatefulSet, _ ...client.GetOption) error {
					*deploy = statefullSetWith2Replicas
					return nil
				})
			Expect(WaitUntilStatefulSetScaledToDesiredReplicas(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale done")
		})
	})

	Describe("#ScaleStatefulSetAndWaitUntilScaled", func() {
		It("should scale and wait until statefulset was scaled", func() {
			runtimeClient.EXPECT().Patch(context.TODO(), statefullSet, getPatch(2))
			runtimeClient.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, deploy *appsv1.StatefulSet, _ ...client.GetOption) error {
					*deploy = statefullSetWith2Replicas
					return nil
				})
			Expect(ScaleStatefulSetAndWaitUntilScaled(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale done")
		})
	})

	Describe("#WaitUntilNoPodRunningForDeployment", func() {
		var (
			depl *appsv1.Deployment
		)
		BeforeEach(func() {
			depl = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: namespace.Name,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(0),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"1": "2"},
					},
				},
			}
		})

		It("should wait until there are no pods for the deployment", func() {
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, depl)).To(Succeed())
			Expect(WaitUntilNoPodsForDeployment(ctx, fakeClient, client.ObjectKeyFromObject(depl), time.Second*5, time.Minute*1)).To(Succeed(), "no running pods for deployment")
		})

		It("should timeout waiting for no pods for the deployment", func() {
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			depl.Spec.Replicas = pointer.Int32(1)
			Expect(fakeClient.Create(ctx, depl)).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: namespace.Name,
					Labels:    depl.Spec.Selector.MatchLabels,
				},
			})).To(Succeed())
			err := WaitUntilNoPodsForDeployment(ctx, fakeClient, client.ObjectKeyFromObject(depl), time.Second*1, time.Second*2)
			Expect(err).To(MatchError(ContainSubstring("there is still at least one Pod for deployment: foo/bar")))
		})
	})
})

func getPatch(replicas int) client.Patch {
	return client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)))
}
