// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/worker"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("Worker", func() {
	var (
		ctrl *gomock.Controller
		c    client.Client

		mockNow   *mocktime.MockNow
		now       time.Time
		metav1Now metav1.Time

		ctx = context.TODO()
		log = logr.Discard()

		name                         = "test"
		namespace                    = "testnamespace"
		extensionType                = "some-type"
		region                       = "local"
		sshPublicKey                 = []byte("very-public")
		kubernetesVersion            = semver.MustParse("1.31.1")
		workerKubernetesVersion      = "1.27.6"
		infrastructureProviderStatus = &runtime.RawExtension{Raw: []byte(`{"baz":"foo"}`)}

		worker1Name                           = "worker1"
		worker1Minimum                  int32 = 1
		worker1Maximum                  int32 = 2
		worker1Priority                       = ptr.To(int32(0))
		worker1MaxSurge                       = intstr.FromInt32(3)
		worker1MaxUnavailable                 = intstr.FromInt32(4)
		worker1Labels                         = map[string]string{"foo": "bar"}
		worker1Annotations                    = map[string]string{"bar": "foo"}
		worker1Taints                         = []corev1.Taint{{}}
		worker1MachineType                    = "worker1machinetype"
		worker1MachineImageName               = "worker1machineimage"
		worker1MachineImageVersion            = "worker1machineimagev1"
		worker1MCMSettings                    = &gardencorev1beta1.MachineControllerManagerSettings{}
		worker1UserDataKeyName                = "user-data-key-name-w1"
		worker1UserDataSecretName             = "user-data-secret-name-w1"
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
		worker1Arch                           = ptr.To("amd64")

		worker2Name                      = "worker2"
		worker2Minimum             int32 = 5
		worker2Maximum             int32 = 6
		worker2Priority                  = ptr.To(int32(10))
		worker2MaxSurge                  = intstr.FromInt32(7)
		worker2MaxUnavailable            = intstr.FromInt32(8)
		worker2MachineType               = "worker2machinetype"
		worker2MachineImageName          = "worker2machineimage"
		worker2MachineImageVersion       = "worker2machineimagev1"
		worker2UserDataKeyName           = "user-data-key-name-w2"
		worker2UserDataSecretName        = "user-data-secret-name-w2"
		worker2Arch                      = ptr.To("arm64")

		machineTypes = []gardencorev1beta1.MachineType{
			{
				Name:   worker1MachineType,
				CPU:    resource.MustParse("4"),
				GPU:    resource.MustParse("1"),
				Memory: resource.MustParse("256Gi"),
			},
			{
				Name:   worker2MachineType,
				CPU:    resource.MustParse("16"),
				GPU:    resource.MustParse("0"),
				Memory: resource.MustParse("512Gi"),
			},
		}

		workerPool1NodeTemplate = &extensionsv1alpha1.NodeTemplate{
			Capacity: corev1.ResourceList{
				"cpu":    machineTypes[0].CPU,
				"gpu":    machineTypes[0].GPU,
				"memory": machineTypes[0].Memory,
			},
		}

		workerPool2NodeTemplate = &extensionsv1alpha1.NodeTemplate{
			Capacity: corev1.ResourceList{
				"cpu":    machineTypes[1].CPU,
				"gpu":    machineTypes[1].GPU,
				"memory": machineTypes[1].Memory,
			},
		}

		w, empty *extensionsv1alpha1.Worker
		wSpec    extensionsv1alpha1.WorkerSpec

		defaultDepWaiter worker.Interface
		values           *worker.Values

		emptyAutoscalerOptions = &extensionsv1alpha1.ClusterAutoscalerOptions{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()
		metav1Now = metav1.NewTime(now)

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		values = &worker.Values{
			Name:                         name,
			Namespace:                    namespace,
			Type:                         extensionType,
			Region:                       region,
			KubernetesVersion:            kubernetesVersion,
			MachineTypes:                 machineTypes,
			SSHPublicKey:                 sshPublicKey,
			InfrastructureProviderStatus: infrastructureProviderStatus,
			WorkerPoolNameToOperatingSystemConfigsMap: map[string]*operatingsystemconfig.OperatingSystemConfigs{
				worker1Name: {
					Init: operatingsystemconfig.Data{
						GardenerNodeAgentSecretName: worker1UserDataKeyName,
						SecretName:                  &worker1UserDataSecretName,
					},
				},
				worker2Name: {
					Init: operatingsystemconfig.Data{
						GardenerNodeAgentSecretName: worker2UserDataKeyName,
						SecretName:                  &worker2UserDataSecretName,
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
					Priority:       worker1Priority,
					Annotations:    worker1Annotations,
					Labels:         worker1Labels,
					Taints:         worker1Taints,
					Machine: gardencorev1beta1.Machine{
						Type: worker1MachineType,
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    worker1MachineImageName,
							Version: &worker1MachineImageVersion,
						},
						Architecture: worker1Arch,
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
					ClusterAutoscaler:                &gardencorev1beta1.ClusterAutoscalerOptions{},
				},
				{
					Name:           worker2Name,
					Minimum:        worker2Minimum,
					Maximum:        worker2Maximum,
					MaxSurge:       &worker2MaxSurge,
					MaxUnavailable: &worker2MaxUnavailable,
					Priority:       worker2Priority,
					Machine: gardencorev1beta1.Machine{
						Type: worker2MachineType,
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    worker2MachineImageName,
							Version: &worker2MachineImageVersion,
						},
						Architecture: worker2Arch,
					},
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Version: &workerKubernetesVersion,
					},
					ClusterAutoscaler: &gardencorev1beta1.ClusterAutoscalerOptions{},
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
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
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
					Priority:       worker1Priority,
					Annotations:    worker1Annotations,
					Labels: utils.MergeStringMaps(worker1Labels, map[string]string{
						"node.kubernetes.io/role":                                                   "node",
						"kubernetes.io/arch":                                                        *worker1Arch,
						"worker.gardener.cloud/pool":                                                worker1Name,
						"worker.garden.sapcloud.io/group":                                           worker1Name,
						"worker.gardener.cloud/cri-name":                                            string(worker1CRIName),
						"worker.gardener.cloud/gardener-node-agent-secret-name":                     worker1UserDataKeyName,
						"containerruntime.worker.gardener.cloud/" + worker1CRIContainerRuntime1Type: "true",
						"networking.gardener.cloud/node-local-dns-enabled":                          "false",
					}),
					Taints:      worker1Taints,
					MachineType: worker1MachineType,
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    worker1MachineImageName,
						Version: worker1MachineImageVersion,
					},
					ProviderConfig:    worker1ProviderConfig,
					UserDataSecretRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: worker1UserDataSecretName}, Key: "cloud_config"},
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
					KubernetesVersion:                ptr.To(kubernetesVersion.String()),
					Zones:                            []string{worker1Zone1, worker1Zone2},
					MachineControllerManagerSettings: worker1MCMSettings,
					NodeTemplate:                     workerPool1NodeTemplate,
					Architecture:                     worker1Arch,
					ClusterAutoscaler:                emptyAutoscalerOptions,
				},
				{
					Name:           worker2Name,
					Minimum:        worker2Minimum,
					Maximum:        worker2Maximum,
					MaxSurge:       worker2MaxSurge,
					MaxUnavailable: worker2MaxUnavailable,
					Priority:       worker2Priority,
					Labels: map[string]string{
						"node.kubernetes.io/role":                               "node",
						"kubernetes.io/arch":                                    *worker2Arch,
						"worker.gardener.cloud/system-components":               "true",
						"worker.gardener.cloud/pool":                            worker2Name,
						"worker.garden.sapcloud.io/group":                       worker2Name,
						"worker.gardener.cloud/gardener-node-agent-secret-name": worker2UserDataKeyName,
						"networking.gardener.cloud/node-local-dns-enabled":      "false",
					},
					MachineType: worker2MachineType,
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    worker2MachineImageName,
						Version: worker2MachineImageVersion,
					},
					KubernetesVersion: &workerKubernetesVersion,
					UserDataSecretRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: worker2UserDataSecretName}, Key: "cloud_config"},
					NodeTemplate:      workerPool2NodeTemplate,
					Architecture:      worker2Arch,
					ClusterAutoscaler: emptyAutoscalerOptions,
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "1",
				},
				Spec: wSpec,
			}))
		})
		It("should initialize nodeTemplate when it exists for pool in worker resource, but absent in cloudProfile", func() {
			defer test.WithVars(&worker.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			newValues := *values
			newValues.Workers = []gardencorev1beta1.Worker{
				values.Workers[1],
			}
			newValues.MachineTypes = []gardencorev1beta1.MachineType{}

			expectedWorkerSpec := wSpec.DeepCopy()
			expectedWorkerSpec.Pools = []extensionsv1alpha1.WorkerPool{
				wSpec.Pools[1],
			}

			existingWorker := w.DeepCopy()
			existingWorker.Spec.Pools = []extensionsv1alpha1.WorkerPool{
				wSpec.Pools[1],
			}

			Expect(c.Create(ctx, existingWorker)).To(Succeed(), "creating worker succeeds")

			defaultDepWaiter = worker.New(log, c, &newValues, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.Worker{}

			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj).To(DeepEqual(&extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "2",
				},
				Spec: *expectedWorkerSpec,
			}))
		})

		It("should initialize nodeTemplate from cloudProfile, when machineType updated for worker pool", func() {
			defer test.WithVars(&worker.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			newValues := *values
			newValues.Workers = []gardencorev1beta1.Worker{
				values.Workers[1],
			}
			newValues.MachineTypes = machineTypes

			expectedWorkerSpec := wSpec.DeepCopy()
			expectedWorkerSpec.Pools = []extensionsv1alpha1.WorkerPool{
				wSpec.Pools[1],
			}

			existingWorker := w.DeepCopy()
			existingWorker.Spec.Pools = []extensionsv1alpha1.WorkerPool{
				wSpec.Pools[1],
			}
			existingWorker.Spec.Pools[0].MachineType = worker1MachineType

			Expect(c.Create(ctx, existingWorker)).To(Succeed(), "creating worker succeeds")

			defaultDepWaiter = worker.New(log, c, &newValues, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.Worker{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)).To(Succeed())

			Expect(obj).To(DeepEqual(&extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "2",
				},
				Spec: *expectedWorkerSpec,
			}))
		})

		It("should successfully deploy the Worker resource with cluster autoscaker options when present", func() {
			defer test.WithVars(&worker.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			newValues := *values
			newValues.Workers[0].ClusterAutoscaler = &gardencorev1beta1.ClusterAutoscalerOptions{
				ScaleDownUtilizationThreshold:    ptr.To(0.5),
				ScaleDownGpuUtilizationThreshold: ptr.To(0.7),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 1 * time.Minute}),
				ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
				MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
			}
			newValues.Workers[1].ClusterAutoscaler = &gardencorev1beta1.ClusterAutoscalerOptions{
				ScaleDownGpuUtilizationThreshold: ptr.To(0.8),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 4 * time.Minute}),
				MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
			}

			expectedWorkerSpec := wSpec.DeepCopy()
			expectedWorkerSpec.Pools[0].ClusterAutoscaler = &extensionsv1alpha1.ClusterAutoscalerOptions{
				ScaleDownUtilizationThreshold:    ptr.To("0.5"),
				ScaleDownGpuUtilizationThreshold: ptr.To("0.7"),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 1 * time.Minute}),
				ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
				MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
			}
			expectedWorkerSpec.Pools[1].ClusterAutoscaler = &extensionsv1alpha1.ClusterAutoscalerOptions{
				ScaleDownGpuUtilizationThreshold: ptr.To("0.8"),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 4 * time.Minute}),
				MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
			}

			existingWorker := w.DeepCopy()
			existingWorker.Spec.Pools = expectedWorkerSpec.Pools

			Expect(c.Create(ctx, existingWorker)).To(Succeed(), "creating worker succeeds")

			defaultDepWaiter = worker.New(log, c, &newValues, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.Worker{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)).To(Succeed())

			Expect(obj).To(DeepEqual(&extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "2",
				},
				Spec: *expectedWorkerSpec,
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

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(w.DeepCopy())
			w.Status.LastError = nil
			// remove operation annotation, add old timestamp annotation
			w.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			w.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, w, patch)).To(Succeed(), "patching worker succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "worker indicates error")
		})

		It("should return no error when it's ready", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(w.DeepCopy())
			w.Status.LastError = nil
			// remove operation annotation, add up-to-date timestamp annotation
			w.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			w.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
			}
			Expect(c.Patch(ctx, w, patch)).To(Succeed(), "patching worker succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "worker is ready")
		})
	})

	Describe("#WaitUntilWorkerStatusMachineDeploymentsUpdated", func() {
		It("should return error when no resources are found", func() {
			Expect(defaultDepWaiter.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)).To(HaveOccurred())
		})

		It("should return error when status.machineDeploymentsLastUpdateTime remains nil", func() {
			obj := w.DeepCopy()
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating worker succeeds")

			Expect(defaultDepWaiter.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)).To(HaveOccurred(), "worker status is not updated")
		})

		It("should return error when status.machineDeploymentsLastUpdateTime is not updated", func() {
			obj := w.DeepCopy()
			obj.Status.MachineDeploymentsLastUpdateTime = &metav1Now
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating worker succeeds")

			// this will populate the machineDeploymentsLastUpdateTime in the worker struct
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			Expect(defaultDepWaiter.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)).To(HaveOccurred(), "worker status is not updated")
		})

		It("should return no error when status.machineDeploymentsLastUpdateTime is added for the first time", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(w.DeepCopy())
			// remove operation annotation, add up-to-date timestamp annotation
			w.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			// update the MachineDeploymentsLastUpdateTime in the worker status
			w.Status.MachineDeploymentsLastUpdateTime = &metav1Now
			Expect(c.Patch(ctx, w, patch)).To(Succeed(), "patching worker succeeds")

			By("WaitUntilWorkerStatusMachineDeploymentsUpdated")
			Expect(defaultDepWaiter.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)).To(Succeed(), "worker status is updated with latest machine deployments")
		})

		It("should return no error when status.machineDeploymentsLastUpdateTime is updated", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			obj := w.DeepCopy()
			obj.Status.MachineDeploymentsLastUpdateTime = &metav1Now
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating worker succeeds")

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(w.DeepCopy())
			// remove operation annotation, add up-to-date timestamp annotation
			w.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			// update the MachineDeploymentsLastUpdateTime in the worker status
			lastUpdateTime := metav1.NewTime(metav1Now.Add(1 * time.Second))
			w.Status.MachineDeploymentsLastUpdateTime = &lastUpdateTime
			Expect(c.Patch(ctx, w, patch)).To(Succeed(), "patching worker succeeds")

			By("WaitUntilWorkerStatusMachineDeploymentsUpdated")
			Expect(defaultDepWaiter.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)).To(Succeed(), "worker status is updated with latest machine deployments")
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
				&gardenerutils.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			fakeErr := fmt.Errorf("some random error")
			obj := w.DeepCopy()
			obj.Annotations = map[string]string{
				"confirmation.gardener.cloud/deletion": "true",
				"gardener.cloud/timestamp":             now.UTC().Format(time.RFC3339Nano),
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
		var shootState *gardencorev1beta1.ShootState

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Name:  &w.Name,
							Kind:  extensionsv1alpha1.WorkerResource,
							State: &runtime.RawExtension{Raw: []byte(`{"some":"state"}`)},
						},
					},
				},
			}
		})

		It("should properly restore the worker state if it exists", func() {
			defer test.WithVars(
				&worker.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("workers"), name)).AnyTimes()

			// deploy with wait-for-state annotation
			obj := w.DeepCopy()
			obj.Spec = wSpec
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().Format(time.RFC3339Nano))
			obj.TypeMeta = metav1.TypeMeta{}
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(obj)).
				DoAndReturn(func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(obj))
					return nil
				})

			// restore state
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = &runtime.RawExtension{Raw: []byte(`{"some":"state"}`)}
			test.EXPECTStatusPatch(ctx, mockStatusWriter, expectedWithState, obj, types.MergePatchType)

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
