// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("scale", func() {
	var (
		ctrl          *gomock.Controller
		runtimeClient *mockclient.MockClient
		sw            *mockclient.MockSubResourceClient
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
				Replicas: ptr.To(int32(2)),
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
		sw = mockclient.NewMockSubResourceClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("ScaleStatefulSet", func() {
		It("sets scale to 2", func() {
			runtimeClient.EXPECT().SubResource("scale").Return(sw)
			sw.EXPECT().Patch(context.TODO(), statefullSet, getPatch(2))
			Expect(ScaleStatefulSet(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale succeeds")
		})
	})

	Context("ScaleDeployment", func() {
		It("sets scale to 2", func() {
			runtimeClient.EXPECT().SubResource("scale").Return(sw)
			sw.EXPECT().Patch(context.TODO(), deployment, getPatch(2))
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
							Replicas: ptr.To(int32(2)),
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
			runtimeClient.EXPECT().SubResource("scale").Return(sw)
			sw.EXPECT().Patch(context.TODO(), statefullSet, getPatch(2))
			runtimeClient.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, deploy *appsv1.StatefulSet, _ ...client.GetOption) error {
					*deploy = statefullSetWith2Replicas
					return nil
				})
			Expect(ScaleStatefulSetAndWaitUntilScaled(context.TODO(), runtimeClient, key, 2)).To(Succeed(), "scale done")
		})
	})
})

func getPatch(replicas int) client.Patch {
	return client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)))
}
