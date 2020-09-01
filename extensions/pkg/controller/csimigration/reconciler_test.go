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

package csimigration

import (
	"context"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("reconciler", func() {
	Describe("#reconcile", func() {
		var (
			ctrl *gomock.Controller

			ctx    = context.TODO()
			logger = log.Log.WithName("test")
			c      *mockclient.MockClient

			cluster *extensionsv1alpha1.Cluster
			shoot   *gardencorev1beta1.Shoot

			clusterName                   = "cluster"
			csiMigrationKubernetesVersion = "1.18"
			storageClassName              = "foo"
			storageClassProvisioner       = "bar"

			emptyPatch                      = client.ConstantPatch(types.StrategicMergePatchType, []byte("{}"))
			kubeAPIServerDeployment         = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: clusterName}}
			kubeControllerManagerDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeControllerManager, Namespace: clusterName}}
			kubeSchedulerDeployment         = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: clusterName}}

			reconciler = &reconciler{
				logger:                              logger,
				ctx:                                 ctx,
				csiMigrationKubernetesVersion:       csiMigrationKubernetesVersion,
				storageClassNameToLegacyProvisioner: map[string]string{storageClassName: storageClassProvisioner},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			reconciler.client = c

			cluster = &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}}
			shoot = shootWithVersion("1.18.0")
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should behave as expected when the needs-complete annotation is already set", func() {
			cluster.Annotations = map[string]string{
				AnnotationKeyNeedsComplete: "true",
			}

			c.EXPECT().Patch(ctx, kubeAPIServerDeployment, emptyPatch)
			c.EXPECT().Patch(ctx, kubeControllerManagerDeployment, emptyPatch)
			c.EXPECT().Patch(ctx, kubeSchedulerDeployment, emptyPatch)

			c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
				cluster, ok := obj.(*extensionsv1alpha1.Cluster)
				Expect(ok).To(BeTrue())
				Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyNeedsComplete, "true"))
				Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyControllerFinished, "true"))
				return nil
			})

			_, err := reconciler.reconcile(ctx, cluster, shoot)
			Expect(err).To(Succeed())
		})

		It("should behave as expected for new clusters", func() {
			c.EXPECT().Get(ctx, kutil.Key(cluster.Name, shoot.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{})).Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("controlplane"), shoot.Name))

			c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
				cluster, ok := obj.(*extensionsv1alpha1.Cluster)
				Expect(ok).To(BeTrue())
				Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyNeedsComplete, "true"))
				Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyControllerFinished, "true"))
				return nil
			})

			_, err := reconciler.reconcile(ctx, cluster, shoot)
			Expect(err).To(Succeed())
		})

		It("should do nothing if the minimum shoot version is not reached", func() {
			shoot.Spec.Kubernetes.Version = "1.17.1"

			c.EXPECT().Get(ctx, kutil.Key(cluster.Name, shoot.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}))

			_, err := reconciler.reconcile(ctx, cluster, shoot)
			Expect(err).To(Succeed())
		})

		It("should do nothing if the shoot is hibernated", func() {
			shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: pointer.BoolPtr(true)}

			c.EXPECT().Get(ctx, kutil.Key(cluster.Name, shoot.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}))

			_, err := reconciler.reconcile(ctx, cluster, shoot)
			Expect(err).To(Succeed())
		})

		Context("CSI migration", func() {
			var shootClient *mockclient.MockClient

			BeforeEach(func() {
				shootClient = mockclient.NewMockClient(ctrl)
			})

			It("should requeue if not all nodes are updated", func() {
				c.EXPECT().Get(ctx, kutil.Key(cluster.Name, shoot.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}))

				oldNewClientForShoot := NewClientForShoot
				defer func() { NewClientForShoot = oldNewClientForShoot }()
				NewClientForShoot = func(_ context.Context, _ client.Client, _ string, _ client.Options) (*rest.Config, client.Client, error) {
					return nil, shootClient, nil
				}

				shootClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
					obj := &corev1.NodeList{
						Items: []corev1.Node{
							{
								Status: corev1.NodeStatus{
									NodeInfo: corev1.NodeSystemInfo{
										KubeletVersion: "1.17.0",
									},
								},
							},
						},
					}
					obj.DeepCopyInto(list.(*corev1.NodeList))
					return nil
				})

				result, err := reconciler.reconcile(ctx, cluster, shoot)
				Expect(result.RequeueAfter).To(Equal(RequeueAfter))
				Expect(err).To(Succeed())
			})

			It("should perform the migration steps correctly if all nodes are updated", func() {
				c.EXPECT().Get(ctx, kutil.Key(cluster.Name, shoot.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}))

				oldNewClientForShoot := NewClientForShoot
				defer func() { NewClientForShoot = oldNewClientForShoot }()
				NewClientForShoot = func(_ context.Context, _ client.Client, _ string, _ client.Options) (*rest.Config, client.Client, error) {
					return nil, shootClient, nil
				}

				shootClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
					obj := &corev1.NodeList{
						Items: []corev1.Node{
							{
								Status: corev1.NodeStatus{
									NodeInfo: corev1.NodeSystemInfo{
										KubeletVersion: csiMigrationKubernetesVersion,
									},
								},
							},
						},
					}
					obj.DeepCopyInto(list.(*corev1.NodeList))
					return nil
				})

				storageClass := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: storageClassName,
					},
					Provisioner: storageClassProvisioner,
				}
				shootClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&storagev1.StorageClassList{})).DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
					obj := &storagev1.StorageClassList{
						Items: []storagev1.StorageClass{*storageClass},
					}
					obj.DeepCopyInto(list.(*storagev1.StorageClassList))
					return nil
				})
				shootClient.EXPECT().Delete(ctx, storageClass)

				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					cluster, ok := obj.(*extensionsv1alpha1.Cluster)
					Expect(ok).To(BeTrue())
					Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyNeedsComplete, "true"))
					Expect(cluster.Annotations).NotTo(HaveKeyWithValue(AnnotationKeyControllerFinished, "true"))
					return nil
				})

				c.EXPECT().Patch(ctx, kubeAPIServerDeployment, emptyPatch)
				c.EXPECT().Patch(ctx, kubeControllerManagerDeployment, emptyPatch)
				c.EXPECT().Patch(ctx, kubeSchedulerDeployment, emptyPatch)

				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).DoAndReturn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					cluster, ok := obj.(*extensionsv1alpha1.Cluster)
					Expect(ok).To(BeTrue())
					Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyNeedsComplete, "true"))
					Expect(cluster.Annotations).To(HaveKeyWithValue(AnnotationKeyControllerFinished, "true"))
					return nil
				})

				_, err := reconciler.reconcile(ctx, cluster, shoot)
				Expect(err).To(Succeed())
			})
		})
	})
})

func shootWithVersion(v string) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: v,
			},
		},
	}
}
