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

package worker_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/worker"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Worker", func() {
	var (
		ctrl *gomock.Controller
		c    client.Client

		mockNow *mocktime.MockNow
		now     time.Time

		ctx = context.TODO()
		log = logger.NewNopLogger()

		name                         = "test"
		namespace                    = "testnamespace"
		extensionType                = "some-type"
		region                       = "local"
		sshPublicKey                 = []byte("very-public")
		kubernetesVersion            = semver.MustParse("1.15.5")
		workerKubernetesVersion      = "1.16.6"
		infrastructureProviderStatus = &runtime.RawExtension{Raw: []byte(`{"baz":"foo"}`)}

		worker1Name                           = "worker1"
		worker1Minimum                  int32 = 1
		worker1Maximum                  int32 = 2
		worker1MaxSurge                       = intstr.FromInt(3)
		worker1MaxUnavailable                 = intstr.FromInt(4)
		worker1Labels                         = map[string]string{"foo": "bar"}
		worker1Annotations                    = map[string]string{"bar": "foo"}
		worker1Taints                         = []corev1.Taint{{}}
		worker1MachineType                    = "worker1machinetype"
		worker1MachineImageName               = "worker1machineimage"
		worker1MachineImageVersion            = "worker1machineimagev1"
		worker1MCMSettings                    = &gardencorev1beta1.MachineControllerManagerSettings{}
		worker1UserData                       = []byte("bootstrap-me")
		worker1VolumeName                     = "worker1volumename"
		worker1VolumeType                     = "worker1volumetype"
		worker1VolumeSize                     = "42Gi"
		worker1VolumeEncrypted                = true
		worker1DataVolume1Name                = "worker1datavolume1name"
		worker1DataVolume1Type                = "worker1datavolume1type"
		worker1DataVolume1Size                = "43Gi"
		worker1DataVolume1Encrypted           = false
		worker1KubeletDataVolumeName          = "worker1kubeletdatavol"
		worker1CRIName                        = gardencorev1beta1.CRIName("cri")
		worker1CRIContainerRuntime1Type       = "cruntime"
		worker1ProviderConfig                 = &runtime.RawExtension{Raw: []byte(`{"bar":"baz"}`)}
		worker1Zone1                          = "worker1zone1"
		worker1Zone2                          = "worker1zone1"

		worker2Name                      = "worker2"
		worker2Minimum             int32 = 5
		worker2Maximum             int32 = 6
		worker2MaxSurge                  = intstr.FromInt(7)
		worker2MaxUnavailable            = intstr.FromInt(8)
		worker2MachineType               = "worker2machinetype"
		worker2MachineImageName          = "worker2machineimage"
		worker2MachineImageVersion       = "worker2machineimagev1"
		worker2UserData                  = []byte("bootstrap-me-now")

		w, empty *extensionsv1alpha1.Worker
		wSpec    extensionsv1alpha1.WorkerSpec

		defaultDepWaiter worker.Interface
		values           *worker.Values
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		values = &worker.Values{
			Name:                         name,
			Namespace:                    namespace,
			Type:                         extensionType,
			Region:                       region,
			KubernetesVersion:            kubernetesVersion,
			SSHPublicKey:                 sshPublicKey,
			InfrastructureProviderStatus: infrastructureProviderStatus,
			WorkerNameToOperatingSystemConfigsMap: map[string]*operatingsystemconfig.OperatingSystemConfigs{
				worker1Name: {
					Downloader: operatingsystemconfig.Data{
						Content: string(worker1UserData),
					},
				},
				worker2Name: {
					Downloader: operatingsystemconfig.Data{
						Content: string(worker2UserData),
					},
				},
			},
			Workers: []gardencorev1beta1.Worker{
				{
					Name:           worker1Name,
					Minimum:        worker1Minimum,
					Maximum:        worker1Maximum,
					MaxSurge:       &worker1MaxSurge,
					MaxUnavailable: &worker1MaxUnavailable,
					Annotations:    worker1Annotations,
					Labels:         worker1Labels,
					Taints:         worker1Taints,
					Machine: gardencorev1beta1.Machine{
						Type: worker1MachineType,
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    worker1MachineImageName,
							Version: &worker1MachineImageVersion,
						},
					},
					Volume: &gardencorev1beta1.Volume{
						Name:       &worker1VolumeName,
						Type:       &worker1VolumeType,
						VolumeSize: worker1VolumeSize,
						Encrypted:  &worker1VolumeEncrypted,
					},
					DataVolumes: []gardencorev1beta1.DataVolume{
						{
							Name:       worker1DataVolume1Name,
							Type:       &worker1DataVolume1Type,
							VolumeSize: worker1DataVolume1Size,
							Encrypted:  &worker1DataVolume1Encrypted,
						},
					},
					KubeletDataVolumeName: &worker1KubeletDataVolumeName,
					SystemComponents:      &gardencorev1beta1.WorkerSystemComponents{Allow: false},
					CRI: &gardencorev1beta1.CRI{
						Name:              worker1CRIName,
						ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: worker1CRIContainerRuntime1Type}},
					},
					ProviderConfig:                   worker1ProviderConfig,
					MachineControllerManagerSettings: worker1MCMSettings,
					Zones:                            []string{worker1Zone1, worker1Zone2},
				},
				{
					Name:           worker2Name,
					Minimum:        worker2Minimum,
					Maximum:        worker2Maximum,
					MaxSurge:       &worker2MaxSurge,
					MaxUnavailable: &worker2MaxUnavailable,
					Machine: gardencorev1beta1.Machine{
						Type: worker2MachineType,
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    worker2MachineImageName,
							Version: &worker2MachineImageVersion,
						},
					},
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Version: &workerKubernetesVersion,
					},
				},
			},
		}

		empty = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		w = empty.DeepCopy()
		w.SetAnnotations(map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			v1beta1constants.GardenerTimestamp: now.UTC().String(),
		})

		wSpec = extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: extensionType,
			},
			Region: region,
			SecretRef: corev1.SecretReference{
				Name:      "cloudprovider",
				Namespace: namespace,
			},
			SSHPublicKey:                 sshPublicKey,
			InfrastructureProviderStatus: infrastructureProviderStatus,
			Pools: []extensionsv1alpha1.WorkerPool{
				{
					Name:           worker1Name,
					Minimum:        worker1Minimum,
					Maximum:        worker1Maximum,
					MaxSurge:       worker1MaxSurge,
					MaxUnavailable: worker1MaxUnavailable,
					Annotations:    worker1Annotations,
					Labels: utils.MergeStringMaps(worker1Labels, map[string]string{
						"node.kubernetes.io/role":         "node",
						"worker.gardener.cloud/pool":      worker1Name,
						"worker.garden.sapcloud.io/group": worker1Name,
						"worker.gardener.cloud/cri-name":  string(worker1CRIName),
						"containerruntime.worker.gardener.cloud/" + worker1CRIContainerRuntime1Type: "true",
					}),
					Taints:      worker1Taints,
					MachineType: worker1MachineType,
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    worker1MachineImageName,
						Version: worker1MachineImageVersion,
					},
					ProviderConfig: worker1ProviderConfig,
					UserData:       worker1UserData,
					Volume: &extensionsv1alpha1.Volume{
						Name:      &worker1VolumeName,
						Type:      &worker1VolumeType,
						Size:      worker1VolumeSize,
						Encrypted: &worker1VolumeEncrypted,
					},
					DataVolumes: []extensionsv1alpha1.DataVolume{
						{
							Name:      worker1DataVolume1Name,
							Type:      &worker1DataVolume1Type,
							Size:      worker1DataVolume1Size,
							Encrypted: &worker1DataVolume1Encrypted,
						},
					},
					KubeletDataVolumeName:            &worker1KubeletDataVolumeName,
					KubernetesVersion:                pointer.String(kubernetesVersion.String()),
					Zones:                            []string{worker1Zone1, worker1Zone2},
					MachineControllerManagerSettings: worker1MCMSettings,
				},
				{
					Name:           worker2Name,
					Minimum:        worker2Minimum,
					Maximum:        worker2Maximum,
					MaxSurge:       worker2MaxSurge,
					MaxUnavailable: worker2MaxUnavailable,
					Labels: map[string]string{
						"node.kubernetes.io/role":                 "node",
						"worker.gardener.cloud/system-components": "true",
						"worker.gardener.cloud/pool":              worker2Name,
						"worker.garden.sapcloud.io/group":         worker2Name,
					},
					MachineType: worker2MachineType,
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    worker2MachineImageName,
						Version: worker2MachineImageVersion,
					},
					KubernetesVersion: &workerKubernetesVersion,
					UserData:          worker2UserData,
				},
			},
		}

		defaultDepWaiter = worker.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should successfully deploy the Worker resource", func() {
			defer test.WithVars(&worker.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			defaultDepWaiter = worker.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.Worker{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj).To(DeepEqual(&extensionsv1alpha1.Worker{
				TypeMeta: metav1.TypeMeta{
					APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
					Kind:       extensionsv1alpha1.WorkerResource,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().String(),
					},
					ResourceVersion: "1",
				},
				Spec: wSpec,
			}))
		})
	})

	Describe("#Wait", func() {
		It("should return error when no resources are found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when resource is not ready", func() {
			obj := w.DeepCopy()
			obj.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating worker succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred(), "worker indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("patch object")
			patch := client.MergeFrom(w.DeepCopy())
			w.Status.LastError = nil
			// remove operation annotation, add old timestamp annotation
			w.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().String(),
			}
			w.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, w, patch)).To(Succeed(), "patching worker succeeds")

			By("wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "worker indicates error")
		})

		It("should return no error when it's ready", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("patch object")
			patch := client.MergeFrom(w.DeepCopy())
			w.Status.LastError = nil
			// remove operation annotation, add up-to-date timestamp annotation
			w.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}
			w.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, w, patch)).To(Succeed(), "patching worker succeeds")

			By("wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "worker is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			Expect(c.Create(ctx, w.DeepCopy())).To(Succeed(), "adding pre-existing worker succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error if not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gutil.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			fakeErr := fmt.Errorf("some random error")
			obj := w.DeepCopy()
			obj.Annotations = map[string]string{
				"confirmation.gardener.cloud/deletion": "true",
				"gardener.cloud/timestamp":             now.UTC().String(),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.Any())
			mc.EXPECT().Delete(ctx, obj).Return(fakeErr)

			err := worker.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Destroy(ctx)
			Expect(err).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when resources are removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources with deletionTimestamp still exist", func() {
			timeNow := metav1.Now()
			obj := w.DeepCopy()
			obj.DeletionTimestamp = &timeNow
			Expect(c.Create(ctx, obj)).To(Succeed())

			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		var (
			state      = &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}
			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: []gardencorev1alpha1.ExtensionResourceState{
						{
							Name:  &name,
							Kind:  extensionsv1alpha1.WorkerResource,
							State: state,
						},
					},
				},
			}
		)

		It("should properly restore the worker state if it exists", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Status().Return(mc)

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("workers"), name))

			// deploy with wait-for-state annotation
			obj := w.DeepCopy()
			obj.Spec = wSpec
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().String())
			obj.TypeMeta = metav1.TypeMeta{}
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(obj)).
				DoAndReturn(func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(obj))
					return nil
				})

			// restore state
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			test.EXPECTPatch(ctx, mc, expectedWithState, obj, types.MergePatchType)

			// annotate with restore annotation
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations["gardener.cloud/operation"] = "restore"
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			Expect(worker.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources", func() {
			Expect(c.Create(ctx, w.DeepCopy())).To(Succeed(), "creating worker succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			result := &extensionsv1alpha1.Worker{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, result)).To(Succeed())
			Expect(result.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "migrate"))
		})

		It("should not return error if resource does not exist", func() {
			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrate", func() {
		It("should not return error when resource is missing", func() {
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			obj := w.DeepCopy()
			obj.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, obj)).To(Succeed(), "creating worker succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should not return error if resource gets migrated successfully", func() {
			obj := w.DeepCopy()
			obj.Status.LastError = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, obj)).To(Succeed(), "creating worker succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "worker is ready, should not return an error")
		})
	})
})
