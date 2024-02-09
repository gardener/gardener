// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vali_test

import (
	"context"
	"fmt"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/test"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	namespace                          = "shoot--foo--bar"
	managedResourceName                = "vali"
	managedResourceSecretName          = "managedresource-vali"
	managedResourceNameTarget          = "vali-target"
	managedResourceSecretNameTarget    = "managedresource-vali-target"
	valiName                           = "vali"
	valiConfigMapName                  = "vali-config-bc8a885d"
	telegrafConfigMapName              = "telegraf-config-b4c38756"
	maintenanceBegin                   = "210000-0000"
	maintenanceEnd                     = "223000-0000"
	valiImage                          = "vali:0.0.1"
	curatorImage                       = "curator:0.0.1"
	alpineImage                        = "alpine:0.0.1"
	initLargeDirImage                  = "tune2fs:0.0.1"
	telegrafImage                      = "telegraf-iptables:0.0.1"
	kubeRBACProxyImage                 = "kube-rbac-proxy:0.0.1"
	priorityClassName                  = "foo-bar"
	valiHost                           = "vali.foo.bar"
	valitailShootAccessSecretName      = "shoot-access-valitail"
	kubeRBacProxyShootAccessSecretName = "shoot-access-kube-rbac-proxy"
)

var _ = Describe("Vali", func() {
	var (
		ctx = context.TODO()
	)

	Describe("#Deploy", func() {
		var (
			c                           client.Client
			managedResource             *resourcesv1alpha1.ManagedResource
			managedResourceSecret       *corev1.Secret
			managedResourceTarget       *resourcesv1alpha1.ManagedResource
			managedResourceSecretTarget *corev1.Secret

			fakeSecretManager secretsmanager.Interface
			storage           = resource.MustParse("60Gi")
		)

		BeforeEach(func() {
			var err error
			c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeSecretManager = fakesecretsmanager.New(c, namespace)

			Expect(err).ToNot(HaveOccurred())

			By("Create secrets managed outside of this package for which secretsmanager.Get() will be called")
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
		})

		JustBeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
				},
			}
			managedResourceSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceSecretName,
					Namespace: namespace,
				},
			}

			managedResourceTarget = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceNameTarget,
					Namespace: namespace,
				},
			}
			managedResourceSecretTarget = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceSecretNameTarget,
					Namespace: namespace,
				},
			}
		})

		It("should successfully deploy all resources for shoot", func() {
			valiDeployer := New(
				c,
				namespace,
				fakeSecretManager,
				Values{
					Replicas:                1,
					Storage:                 &storage,
					ShootNodeLoggingEnabled: true,
					HVPAEnabled:             true,
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: maintenanceBegin,
						End:   maintenanceEnd,
					},
					ValiImage:          valiImage,
					CuratorImage:       curatorImage,
					InitLargeDirImage:  initLargeDirImage,
					TelegrafImage:      telegrafImage,
					KubeRBACProxyImage: kubeRBACProxyImage,
					PriorityClassName:  priorityClassName,
					ClusterType:        "shoot",
					IngressHost:        valiHost,
				},
			)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(valiDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Name: valitailShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: kubeRBacProxyShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(6))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__"+telegrafConfigMapName+".yaml"])).To(Equal(test.Serialize(getTelegrafConfigMap())))
			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__"+valiConfigMapName+".yaml"])).To(Equal(test.Serialize(getValiConfigMap())))
			Expect(string(managedResourceSecret.Data["hvpa__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getHVPA(true))))
			Expect(string(managedResourceSecret.Data["ingress__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getIngress())))
			Expect(string(managedResourceSecret.Data["service__shoot--foo--bar__logging.yaml"])).To(Equal(test.Serialize(getService(true, "shoot"))))
			Expect(string(managedResourceSecret.Data["statefulset__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getStatefulSet(true))))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResourceTarget)).To(Succeed())
			expectedTargetMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceNameTarget,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceTarget.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedTargetMr))
			Expect(managedResourceTarget).To(DeepEqual(expectedTargetMr))

			managedResourceSecretTarget.Name = managedResourceTarget.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretTarget), managedResourceSecretTarget)).To(Succeed())
			Expect(managedResourceSecretTarget.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecretTarget.Data).To(HaveLen(3))
			Expect(managedResourceSecretTarget.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecretTarget.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(string(managedResourceSecretTarget.Data["clusterrolebinding____gardener.cloud_logging_kube-rbac-proxy.yaml"])).To(Equal(test.Serialize(getKubeRBACProxyClusterRoleBinding())))
			Expect(string(managedResourceSecretTarget.Data["clusterrole____gardener.cloud_logging_valitail.yaml"])).To(Equal(test.Serialize(getValitailClusterRole())))
			Expect(string(managedResourceSecretTarget.Data["clusterrolebinding____gardener.cloud_logging_valitail.yaml"])).To(Equal(test.Serialize(getValitailClusterRoleBinding())))
		})

		It("should successfully deploy all resources for seed", func() {
			valiDeployer := New(
				c,
				namespace,
				fakeSecretManager,
				Values{
					Replicas:    1,
					Storage:     &storage,
					HVPAEnabled: true,
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: maintenanceBegin,
						End:   maintenanceEnd,
					},
					ValiImage:         valiImage,
					CuratorImage:      curatorImage,
					InitLargeDirImage: initLargeDirImage,
					PriorityClassName: priorityClassName,
					ClusterType:       "seed",
				},
			)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: namespace}})).To(Succeed())
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: namespace}})).To(Succeed())
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretNameTarget, Namespace: namespace}})).To(Succeed())
			Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: namespace}})).To(Succeed())

			Expect(valiDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Name: valitailShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: kubeRBacProxyShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: managedResourceSecretNameTarget, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: managedResourceNameTarget, Namespace: namespace}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(4))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__"+valiConfigMapName+".yaml"])).To(Equal(test.Serialize(getValiConfigMap())))
			Expect(string(managedResourceSecret.Data["hvpa__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getHVPA(false))))
			Expect(string(managedResourceSecret.Data["service__shoot--foo--bar__logging.yaml"])).To(Equal(test.Serialize(getService(false, "seed"))))
			Expect(string(managedResourceSecret.Data["statefulset__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getStatefulSet(false))))
		})

		It("should successfully deploy all resources for seed without HVPA", func() {
			valiDeployer := New(
				c,
				namespace,
				fakeSecretManager,
				Values{
					Replicas:          1,
					Storage:           &storage,
					ValiImage:         valiImage,
					CuratorImage:      curatorImage,
					InitLargeDirImage: initLargeDirImage,
					PriorityClassName: priorityClassName,
					ClusterType:       "seed",
				},
			)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(valiDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(3))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__"+valiConfigMapName+".yaml"])).To(Equal(test.Serialize(getValiConfigMap())))
			Expect(string(managedResourceSecret.Data["service__shoot--foo--bar__logging.yaml"])).To(Equal(test.Serialize(getService(false, "seed"))))
			Expect(string(managedResourceSecret.Data["statefulset__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getStatefulSet(false))))
		})
	})

	Describe("#ResizeOrDeleteValiDataVolumeIfStorageNotTheSame", func() {
		const (
			valiPVCName         = "vali-vali-0"
			valiStatefulSetName = "vali"
			gardenNamespace     = "garden"
		)

		var (
			ctrl              *gomock.Controller
			runtimeClient     *mockclient.MockClient
			sw                *mockclient.MockSubResourceClient
			ctx               = context.TODO()
			valiPVCObjectMeta = metav1.ObjectMeta{
				Name:      valiPVCName,
				Namespace: gardenNamespace,
			}
			valiPVC = &corev1.PersistentVolumeClaim{
				ObjectMeta: valiPVCObjectMeta,
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							"storage": resource.MustParse("100Gi"),
						},
					},
				},
			}
			patch       = client.MergeFrom(valiPVC.DeepCopy())
			statefulset = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      valiStatefulSetName,
					Namespace: gardenNamespace,
				},
			}
			scaledToZeroValiStatefulset = appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:       valiStatefulSetName,
					Namespace:  gardenNamespace,
					Generation: 2,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(0)),
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 2,
					Replicas:           0,
					AvailableReplicas:  0,
				},
			}
			zeroReplicaRawPatch     = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":0}}`))
			errNotFound             = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
			errForbidden            = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonForbidden}}
			new200GiStorageQuantity = resource.MustParse("200Gi")
			new100GiStorageQuantity = resource.MustParse("100Gi")
			new80GiStorageQuantity  = resource.MustParse("80Gi")
			valiPVCKey              = kubernetesutils.Key("garden", "vali-vali-0")
			valiStatefulSetKey      = kubernetesutils.Key("garden", "vali")
			//nolint:unparam
			funcGetValiPVC = func(_ context.Context, _ types.NamespacedName, pvc *corev1.PersistentVolumeClaim, _ ...client.GetOption) error {
				*pvc = *valiPVC
				return nil
			}
			//nolint:unparam
			funcGetScaledToZeroValiStatefulset = func(_ context.Context, _ types.NamespacedName, sts *appsv1.StatefulSet, _ ...client.GetOption) error {
				*sts = scaledToZeroValiStatefulset
				return nil
			}
			funcPatchTo200GiStorage = func(_ context.Context, pvc *corev1.PersistentVolumeClaim, _ client.Patch, _ ...interface{}) error {
				if pvc.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("200Gi")) != 0 {
					return fmt.Errorf("expect 200Gi found %v", *pvc.Spec.Resources.Requests.Storage())
				}
				return nil
			}
			objectOfTypePVC       = gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})
			objectOfTypeSTS       = gomock.AssignableToTypeOf(&appsv1.StatefulSet{})
			objectOfTypeMR        = gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})
			objectOfTypeSecret    = gomock.AssignableToTypeOf(&corev1.Secret{})
			skipedManagedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: gardenNamespace,
					Annotations: map[string]string{
						"resources.gardener.cloud/ignore": "true",
					},
				},
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: gardenNamespace,
				},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			runtimeClient = mockclient.NewMockClient(ctrl)
			sw = mockclient.NewMockSubResourceClient(ctrl)
			runtimeClient.EXPECT().SubResource("scale").Return(sw).AnyTimes()
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should patch garden/vali's PVC when new size is greater than the current one", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new200GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroValiStatefulset),
				// Path Vali PVC
				runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("should delete garden/vali's PVC when new size is less than the current one", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroValiStatefulset),
				// Delete the Vali PVC
				runtimeClient.EXPECT().Delete(ctx, valiPVC),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("shouldn't do anything when garden/vali's PVC is missing", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC).Return(errNotFound),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("shouldn't do anything when garden/vali's PVC storage is the same as the new one", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new100GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("should proceed with the garden/vali's PVC resizing when Vali StatefulSet is missing", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new200GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch).Return(errNotFound),
				// Path Vali PVC
				runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("should not fail with patching garden/vali's PVC when the PVC itself was deleted during function execution", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new200GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroValiStatefulset),
				// Path Vali PVC
				runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage).Return(errNotFound),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("should not fail with deleting garden/vali's PVC when the PVC itself was deleted during function execution", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroValiStatefulset),
				// Delete the Vali PVC
				runtimeClient.EXPECT().Delete(ctx, valiPVC).Return(errNotFound),
				// Remove Ignore annotation form the managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				// Delete target managed resource
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceNameTarget), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				runtimeClient.EXPECT().Delete(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceNameTarget, Namespace: gardenNamespace}}),
				// Delete shoot access secrets
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: valitailShootAccessSecretName, Namespace: gardenNamespace}}),
				runtimeClient.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kubeRBacProxyShootAccessSecretName, Namespace: gardenNamespace}}),
				// Create Managed resource
				runtimeClient.EXPECT().Get(ctx, gomock.AssignableToTypeOf(types.NamespacedName{}), objectOfTypeSecret),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeSecret),
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Update(ctx, objectOfTypeMR),
			)
			Expect(valiDeployer.Deploy(ctx)).To(Succeed())
		})

		It("should not neglect errors when getting garden/vali's PVC", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})

		It("should not neglect errors when patching garden/vali's StatefulSet", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new200GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})

		It("should not neglect errors when getting garden/vali's StatefulSet", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new200GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})

		It("should not neglect errors when patching garden/vali's PVC", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new200GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroValiStatefulset),
				// Path Vali PVC
				runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage).Return(errNotFound).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})

		It("should not neglect errors when deleting garden/vali's PVC", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).
					Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(skipedManagedResource))
					}),
				// Scale Vali StatefulSet to zero
				sw.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch),
				runtimeClient.EXPECT().Get(gomock.Any(), valiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroValiStatefulset),
				// Delete the Vali PVC
				runtimeClient.EXPECT().Delete(ctx, valiPVC).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})

		It("should not neglect errors when cannot get Vali ManagedResource", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})

		It("should not neglect errors when cannot patch Vali ManagedResource", func() {
			valiDeployer := New(runtimeClient, gardenNamespace, nil, Values{Storage: &new80GiStorageQuantity})
			gomock.InOrder(
				runtimeClient.EXPECT().Get(ctx, valiPVCKey, objectOfTypePVC).DoAndReturn(funcGetValiPVC),
				// Annotate the Vali MamangedResource with Ignore annotation
				runtimeClient.EXPECT().Get(ctx, kubernetesutils.Key(gardenNamespace, managedResourceName), objectOfTypeMR),
				runtimeClient.EXPECT().Patch(ctx, objectOfTypeMR, gomock.Any()).Return(errForbidden),
			)
			Expect(valiDeployer.Deploy(ctx)).ToNot(Succeed())
		})
	})
})

func getService(isRBACProxyEnabled bool, clusterType string) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "logging",
			Namespace:   namespace,
			Labels:      getLabels(),
			Annotations: map[string]string{},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       3100,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(3100),
					Name:       "metrics",
				},
			},
			Selector: getLabels(),
		},
	}

	if isRBACProxyEnabled {
		svc.Spec.Ports = append(svc.Spec.Ports, []corev1.ServicePort{
			{
				Port:       8080,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(8080),
				Name:       "external",
			},
			{
				Port:       9273,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(9273),
				Name:       "telegraf",
			},
		}...)
	}

	switch clusterType {
	case "seed":
		if isRBACProxyEnabled {
			svc.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100},{"protocol":"TCP","port":9273}]`
		} else {
			svc.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100}]`
		}
	case "shoot":
		if isRBACProxyEnabled {
			svc.Annotations["networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100},{"protocol":"TCP","port":9273}]`
		} else {
			svc.Annotations["networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100}]`
		}
		svc.Annotations["networking.resources.gardener.cloud/pod-label-selector-namespace-alias"] = "all-shoots"
		svc.Annotations["networking.resources.gardener.cloud/namespace-selectors"] = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
	}

	return svc
}

func getValiConfigMap() *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vali-config",
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{
			"vali.yaml": `auth_enabled: false
ingester:
  chunk_target_size: 1536000
  chunk_idle_period: 3m
  chunk_block_size: 262144
  chunk_retain_period: 3m
  max_transfer_retries: 3
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
    final_sleep: 0s
    min_ready_duration: 1s
limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
schema_config:
  configs:
  - from: 2018-04-15
    store: boltdb
    object_store: filesystem
    schema: v11
    index:
      prefix: index_
      period: 24h
server:
  http_listen_port: 3100
storage_config:
  boltdb:
    directory: /data/vali/index
  filesystem:
    directory: /data/vali/chunks
chunk_store_config:
  max_look_back_period: 360h
table_manager:
  retention_deletes_enabled: true
  retention_period: 360h
`,
			"curator.yaml": `LogLevel: info
DiskPath: /data/vali/chunks
TriggerInterval: 1h
InodeConfig:
  MinFreePercentages: 10
  TargetFreePercentages: 15
  PageSizeForDeletionPercentages: 1
StorageConfig:
  MinFreePercentages: 10
  TargetFreePercentages: 15
  PageSizeForDeletionPercentages: 1
`,
			"vali-init.sh": `#!/bin/bash
set -o errexit

function error() {
    exit_code=$?
    echo "${BASH_COMMAND} failed, exit code $exit_code"
}

trap error ERR

tune2fs -O large_dir $(mount | gawk '{if($3=="/data") {print $1}}')
`,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap
}

func getTelegrafConfigMap() *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telegraf-config",
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{
			"telegraf.conf": `[[outputs.prometheus_client]]
## Address to listen on.
listen = ":9273"
metric_version = 2
# Gather packets and bytes throughput from iptables
[[inputs.iptables]]
## iptables require root access on most systems.
## Setting 'use_sudo' to true will make use of sudo to run iptables.
## Users must configure sudo to allow telegraf user to run iptables with no password.
## iptables can be restricted to only list command "iptables -nvL".
use_sudo = true
## defines the table to monitor:
table = "filter"
## defines the chains to monitor.
## NOTE: iptables rules without a comment will not be monitored.
## Read the plugin documentation for more information.
chains = [ "INPUT" ]
`,
			"start.sh": `#/bin/bash

trap 'kill %1; wait' SIGTERM
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT -m comment --comment "valitail"
/usr/bin/telegraf --config /etc/telegraf/telegraf.conf &
wait
`,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap
}

func getHVPA(isRBACProxyEnabled bool) *hvpav1alpha1.Hvpa {
	controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	containerPolicyOff := vpaautoscalingv1.ContainerScalingModeOff

	obj := &hvpav1alpha1.Hvpa{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: hvpav1alpha1.HvpaSpec{
			Replicas: ptr.To(int32(1)),
			MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
				Begin: maintenanceBegin,
				End:   maintenanceEnd,
			},
			Hpa: hvpav1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": valiName + "-hpa",
					},
				},
				Deploy: false,
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"role": valiName + "-hpa",
						},
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: ptr.To(int32(1)),
						MaxReplicas: 1,
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "cpu",
									TargetAverageUtilization: ptr.To(int32(80)),
								},
							},
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "memory",
									TargetAverageUtilization: ptr.To(int32(80)),
								},
							},
						},
					},
				},
			},
			Vpa: hvpav1alpha1.VpaSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": valiName + "vpa",
					},
				},
				Deploy: true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: ptr.To("Auto"),
					},
					StabilizationDuration: ptr.To("5m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("100m"),
							Percentage: ptr.To(int32(80)),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("300M"),
							Percentage: ptr.To(int32(80)),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: ptr.To("MaintenanceWindow"),
					},
					StabilizationDuration: ptr.To("168h"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("200m"),
							Percentage: ptr.To(int32(80)),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("500M"),
							Percentage: ptr.To(int32(80)),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      ptr.To("300m"),
						Percentage: ptr.To(int32(40)),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      ptr.To("1G"),
						Percentage: ptr.To(int32(40)),
					},
				},
				Template: hvpav1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"role": valiName + "vpa",
						},
					},
					Spec: hvpav1alpha1.VpaTemplateSpec{
						ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
							ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
								{
									ContainerName: valiName,
									MinAllowed: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("300M"),
									},
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("800m"),
										corev1.ResourceMemory: resource.MustParse("3Gi"),
									},
									ControlledValues: &controlledValues,
								},
								{
									ContainerName:    "curator",
									Mode:             &containerPolicyOff,
									ControlledValues: &controlledValues,
								},
								{
									ContainerName:    "init-large-dir",
									Mode:             &containerPolicyOff,
									ControlledValues: &controlledValues,
								},
							},
						},
					},
				},
			},
			WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{
				{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: 1,
					LastReplicaCount:  1,
				},
			},
			TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       valiName,
			},
		},
	}

	if isRBACProxyEnabled {
		obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies = append(obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies,
			[]vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName:    "kube-rbac-proxy",
					Mode:             &containerPolicyOff,
					ControlledValues: &controlledValues,
				},
				{
					ContainerName:    "telegraf",
					Mode:             &containerPolicyOff,
					ControlledValues: &controlledValues,
				},
			}...)
	}
	return obj
}

func getIngress() *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx-ingress-gardener"),
			TLS: []networkingv1.IngressTLS{
				{
					SecretName: "vali-tls",
					Hosts:      []string{valiHost},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: valiHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "logging",
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
									Path:     "/vali/api/v1/push",
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getStatefulSet(isRBACProxyEnabled bool) *appsv1.StatefulSet {
	fsGroupChangeOnRootMismatch := corev1.FSGroupChangeOnRootMismatch
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getLabels(),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            priorityClassName,
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:             ptr.To(int64(10001)),
						FSGroupChangePolicy: &fsGroupChangeOnRootMismatch,
					},
					InitContainers: []corev1.Container{
						{
							Name:  "init-large-dir",
							Image: initLargeDirImage,
							Command: []string{
								"bash",
								"-c",
								"/vali-init.sh || true",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged:   ptr.To(true),
								RunAsUser:    ptr.To(int64(0)),
								RunAsNonRoot: ptr.To(false),
								RunAsGroup:   ptr.To(int64(0)),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/data",
									Name:      "vali",
								},
								{
									MountPath: "/vali-init.sh",
									SubPath:   "vali-init.sh",
									Name:      "config",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "vali",
							Image: valiImage,
							Args: []string{
								"-config.file=/etc/vali/vali.yaml",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/vali/vali.yaml",
									SubPath:   "vali.yaml",
								},
								{
									Name:      "vali",
									MountPath: "/data",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: 3100,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("metrics"),
									},
								},
								InitialDelaySeconds: 120,
								FailureThreshold:    5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("metrics"),
									},
								},
								FailureThreshold: 7,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("300Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("3Gi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:              ptr.To(int64(10001)),
								RunAsGroup:             ptr.To(int64(10001)),
								RunAsNonRoot:           ptr.To(true),
								ReadOnlyRootFilesystem: ptr.To(true),
							},
						},
						{
							Name:  "curator",
							Image: curatorImage,
							Args: []string{
								"-config=/etc/vali/curator.yaml",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "curatormetrics",
									ContainerPort: 2718,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/vali/curator.yaml",
									SubPath:   "curator.yaml",
								},
								{
									Name:      "vali",
									MountPath: "/data",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("12Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("700Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:              ptr.To(int64(10001)),
								RunAsGroup:             ptr.To(int64(10001)),
								RunAsNonRoot:           ptr.To(true),
								ReadOnlyRootFilesystem: ptr.To(true),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: valiConfigMapName,
									},
									DefaultMode: ptr.To(int32(0520)),
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "vali",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resource.MustParse("60Gi"),
							},
						},
					},
				},
			},
		},
	}

	if isRBACProxyEnabled {
		sts.Spec.Template.ObjectMeta.Labels["networking.gardener.cloud/to-dns"] = "allowed"
		sts.Spec.Template.ObjectMeta.Labels["networking.resources.gardener.cloud/to-kube-apiserver-tcp-443"] = "allowed"

		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, []corev1.Container{
			{
				Name:  "kube-rbac-proxy",
				Image: kubeRBACProxyImage,
				Args: []string{
					"--insecure-listen-address=0.0.0.0:8080",
					"--upstream=http://127.0.0.1:3100/",
					"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
					"--logtostderr=true",
					"--v=6",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("150Mi"),
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "kube-rbac-proxy",
						ContainerPort: 8080,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "kubeconfig",
						MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
						ReadOnly:  true,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:              ptr.To(int64(65532)),
					RunAsGroup:             ptr.To(int64(65534)),
					RunAsNonRoot:           ptr.To(true),
					ReadOnlyRootFilesystem: ptr.To(true),
				},
			},
			{
				Name:  "telegraf",
				Image: telegrafImage,
				Command: []string{
					"/bin/bash",
					"-c",
					`
trap 'kill %1; wait' SIGTERM
bash /etc/telegraf/start.sh &
wait
`,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("35Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("350Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{
							"NET_ADMIN",
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "telegraf",
						ContainerPort: 9273,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "telegraf-config-volume",
						MountPath: "/etc/telegraf/telegraf.conf",
						SubPath:   "telegraf.conf",
						ReadOnly:  true,
					},
					{
						Name:      "telegraf-config-volume",
						MountPath: "/etc/telegraf/start.sh",
						SubPath:   "start.sh",
						ReadOnly:  true,
					},
				},
			},
		}...)

		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: "telegraf-config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: telegrafConfigMapName,
						},
					},
				},
			},
			{
				Name: "kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: ptr.To(int32(420)),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									Items: []corev1.KeyToPath{
										{
											Key:  "kubeconfig",
											Path: "kubeconfig",
										},
									},
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "generic-token-kubeconfig",
									},
									Optional: ptr.To(false),
								},
							},
							{
								Secret: &corev1.SecretProjection{
									Items: []corev1.KeyToPath{
										{
											Key:  "token",
											Path: "token",
										},
									},
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "shoot-access-kube-rbac-proxy",
									},
									Optional: ptr.To(false),
								},
							},
						},
					},
				},
			},
		}...)
	}

	utilruntime.Must(references.InjectAnnotations(sts))

	return sts
}

func getKubeRBACProxyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:kube-rbac-proxy",
			Labels: map[string]string{"app": "kube-rbac-proxy"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "kube-rbac-proxy",
			Namespace: "kube-system",
		}},
	}
}

func getValitailClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:valitail",
			Labels: map[string]string{"app": "gardener-valitail"},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes",
					"nodes/proxy",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				NonResourceURLs: []string{"/vali/api/v1/push"},
				Verbs:           []string{"create"},
			},
		},
	}
}

func getValitailClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:valitail",
			Labels: map[string]string{"app": "gardener-valitail"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "gardener.cloud:logging:valitail",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "gardener-valitail",
			Namespace: "kube-system",
		}},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		"gardener.cloud/role": "logging",
		"role":                "logging",
		"app":                 "vali",
	}
}
