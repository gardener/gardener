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

package etcd_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Etcd", func() {
	Describe("#ServiceName", func() {
		It("should return the expected service name", func() {
			Expect(ServiceName(testRole)).To(Equal("etcd-" + testRole + "-client"))
		})
	})

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		etcd Interface
		log  logrus.FieldLogger

		ctx                     = context.TODO()
		fakeErr                 = fmt.Errorf("fake err")
		class                   = ClassNormal
		retainReplicas          = false
		storageCapacity         = "12Gi"
		storageCapacityQuantity = resource.MustParse(storageCapacity)
		defragmentationSchedule = "abcd"

		secretNameCA         = "ca-etcd"
		secretNameServer     = "etcd-server-tls"
		secretNameClient     = "etcd-client-cert"
		secretChecksumCA     = "1234"
		secretChecksumServer = "5678"
		secretChecksumClient = "9012"
		secrets              = Secrets{
			CA:     component.Secret{Name: secretNameCA, Checksum: secretChecksumCA},
			Server: component.Secret{Name: secretNameServer, Checksum: secretChecksumServer},
			Client: component.Secret{Name: secretNameClient, Checksum: secretChecksumClient},
		}

		maintenanceTimeWindow = gardencorev1beta1.MaintenanceTimeWindow{
			Begin: "1234",
			End:   "5678",
		}
		now                     = time.Time{}
		quota                   = resource.MustParse("8Gi")
		garbageCollectionPolicy = druidv1alpha1.GarbageCollectionPolicy(druidv1alpha1.GarbageCollectionPolicyExponential)
		garbageCollectionPeriod = metav1.Duration{Duration: 12 * time.Hour}
		compressionPolicy       = druidv1alpha1.GzipCompression
		compressionSpec         = druidv1alpha1.CompressionSpec{
			Enabled: true,
			Policy:  &compressionPolicy,
		}
		updateModeAuto     = hvpav1alpha1.UpdateModeAuto
		containerPolicyOff = autoscalingv1beta2.ContainerScalingModeOff
		metricsBasic       = druidv1alpha1.Basic
		metricsExtensive   = druidv1alpha1.Extensive

		networkPolicyName = "allow-etcd"
		etcdName          = "etcd-" + testRole
		hvpaName          = "etcd-" + testRole

		protocolTCP       = corev1.ProtocolTCP
		portEtcd          = intstr.FromInt(2379)
		portBackupRestore = intstr.FromInt(8080)

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      networkPolicyName,
				Namespace: testNamespace,
				Annotations: map[string]string{
					"gardener.cloud/description": "Allows Ingress to etcd pods from the Shoot's Kubernetes API Server.",
				},
				Labels: map[string]string{
					"gardener.cloud/role": "controlplane",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"garden.sapcloud.io/role": "controlplane",
						"app":                     "etcd-statefulset",
					},
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"gardener.cloud/role": "controlplane",
										"app":                 "kubernetes",
										"role":                "apiserver",
									},
								},
							},
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"gardener.cloud/role": "monitoring",
										"app":                 "prometheus",
										"role":                "monitoring",
									},
								},
							},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{
								Protocol: &protocolTCP,
								Port:     &portEtcd,
							},
							{
								Protocol: &protocolTCP,
								Port:     &portBackupRestore,
							},
						},
					},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
				},
			},
		}
		etcdObjFor = func(
			class Class,
			replicas int,
			backupConfig *BackupConfig,
			existingDefragmentationSchedule,
			existingBackupSchedule string,
			existingResourcesContainerEtcd *corev1.ResourceRequirements,
			existingResourcesContainerBackupRestore *corev1.ResourceRequirements,
		) *druidv1alpha1.Etcd {

			defragSchedule := defragmentationSchedule
			if existingDefragmentationSchedule != "" {
				defragSchedule = existingDefragmentationSchedule
			}

			resourcesContainerEtcd := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("1G"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2300m"),
					corev1.ResourceMemory: resource.MustParse("6G"),
				},
			}
			if existingResourcesContainerEtcd != nil {
				resourcesContainerEtcd = existingResourcesContainerEtcd
			}

			resourcesContainerBackupRestore := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("23m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("10G"),
				},
			}
			if existingResourcesContainerBackupRestore != nil {
				resourcesContainerBackupRestore = existingResourcesContainerBackupRestore
			}

			obj := &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      etcdName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.String(),
					},
					Labels: map[string]string{
						"gardener.cloud/role": "controlplane",
						"role":                testRole,
					},
				},
				Spec: druidv1alpha1.EtcdSpec{
					Replicas:          replicas,
					PriorityClassName: pointer.String("gardener-shoot-controlplane"),
					Annotations: map[string]string{
						"checksum/secret-etcd-ca":          secretChecksumCA,
						"checksum/secret-etcd-server-cert": secretChecksumServer,
						"checksum/secret-etcd-client-tls":  secretChecksumClient,
					},
					Labels: map[string]string{
						"garden.sapcloud.io/role":          "controlplane",
						"role":                             testRole,
						"app":                              "etcd-statefulset",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.gardener.cloud/to-public-networks":  "allowed",
						"networking.gardener.cloud/to-private-networks": "allowed",
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"garden.sapcloud.io/role": "controlplane",
							"role":                    testRole,
							"app":                     "etcd-statefulset",
						},
					},
					Etcd: druidv1alpha1.EtcdConfig{
						Resources: resourcesContainerEtcd,
						TLS: &druidv1alpha1.TLSConfig{
							TLSCASecretRef: corev1.SecretReference{
								Name:      secretNameCA,
								Namespace: testNamespace,
							},
							ServerTLSSecretRef: corev1.SecretReference{
								Name:      secretNameServer,
								Namespace: testNamespace,
							},
							ClientTLSSecretRef: corev1.SecretReference{
								Name:      secretNameClient,
								Namespace: testNamespace,
							},
						},
						ServerPort:              &PortEtcdServer,
						ClientPort:              &PortEtcdClient,
						Metrics:                 &metricsBasic,
						DefragmentationSchedule: &defragSchedule,
						Quota:                   &quota,
					},
					Backup: druidv1alpha1.BackupSpec{
						Port:                    &PortBackupRestore,
						Resources:               resourcesContainerBackupRestore,
						GarbageCollectionPolicy: &garbageCollectionPolicy,
						GarbageCollectionPeriod: &garbageCollectionPeriod,
						SnapshotCompression:     &compressionSpec,
					},
					StorageCapacity:     &storageCapacityQuantity,
					VolumeClaimTemplate: pointer.String(etcdName),
				},
			}

			if class == ClassImportant {
				obj.Spec.Annotations["cluster-autoscaler.kubernetes.io/safe-to-evict"] = "false"
				obj.Spec.Etcd.Metrics = &metricsExtensive
				obj.Spec.VolumeClaimTemplate = pointer.String(testRole + "-etcd")
			}

			if backupConfig != nil {
				fullSnapshotSchedule := backupConfig.FullSnapshotSchedule
				if existingBackupSchedule != "" {
					fullSnapshotSchedule = existingBackupSchedule
				}

				provider := druidv1alpha1.StorageProvider(backupConfig.Provider)
				deltaSnapshotPeriod := metav1.Duration{Duration: 5 * time.Minute}
				deltaSnapshotMemoryLimit := resource.MustParse("100Mi")

				obj.Spec.Backup.Store = &druidv1alpha1.StoreSpec{
					SecretRef: &corev1.SecretReference{Name: backupConfig.SecretRefName},
					Container: &backupConfig.Container,
					Provider:  &provider,
					Prefix:    backupConfig.Prefix + "/etcd-" + testRole,
				}
				obj.Spec.Backup.FullSnapshotSchedule = &fullSnapshotSchedule
				obj.Spec.Backup.DeltaSnapshotPeriod = &deltaSnapshotPeriod
				obj.Spec.Backup.DeltaSnapshotMemoryLimit = &deltaSnapshotMemoryLimit
			}

			return obj
		}
		hvpaFor = func(class Class, replicas int32, scaleDownUpdateMode string) *hvpav1alpha1.Hvpa {
			obj := &hvpav1alpha1.Hvpa{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hvpaName,
					Namespace: testNamespace,
					Labels: map[string]string{
						"garden.sapcloud.io/role": "controlplane",
						"role":                    testRole,
						"app":                     "etcd-statefulset",
					},
				},
				Spec: hvpav1alpha1.HvpaSpec{
					Replicas: pointer.Int32(1),
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: maintenanceTimeWindow.Begin,
						End:   maintenanceTimeWindow.End,
					},
					Hpa: hvpav1alpha1.HpaSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"role": "etcd-hpa-" + testRole,
							},
						},
						Deploy: false,
						Template: hvpav1alpha1.HpaTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"role": "etcd-hpa-" + testRole,
								},
							},
							Spec: hvpav1alpha1.HpaTemplateSpec{
								MinReplicas: &replicas,
								MaxReplicas: replicas,
								Metrics: []autoscalingv2beta1.MetricSpec{
									{
										Type: autoscalingv2beta1.ResourceMetricSourceType,
										Resource: &autoscalingv2beta1.ResourceMetricSource{
											Name:                     corev1.ResourceCPU,
											TargetAverageUtilization: pointer.Int32(80),
										},
									},
									{
										Type: autoscalingv2beta1.ResourceMetricSourceType,
										Resource: &autoscalingv2beta1.ResourceMetricSource{
											Name:                     corev1.ResourceMemory,
											TargetAverageUtilization: pointer.Int32(80),
										},
									},
								},
							},
						},
					},
					Vpa: hvpav1alpha1.VpaSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"role": "etcd-vpa-" + testRole,
							},
						},
						Deploy: true,
						ScaleUp: hvpav1alpha1.ScaleType{
							UpdatePolicy: hvpav1alpha1.UpdatePolicy{
								UpdateMode: &updateModeAuto,
							},
							StabilizationDuration: pointer.String("5m"),
							MinChange: hvpav1alpha1.ScaleParams{
								CPU: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("1"),
									Percentage: pointer.Int32(80),
								},
								Memory: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("2G"),
									Percentage: pointer.Int32(80),
								},
							},
						},
						ScaleDown: hvpav1alpha1.ScaleType{
							UpdatePolicy: hvpav1alpha1.UpdatePolicy{
								UpdateMode: &scaleDownUpdateMode,
							},
							StabilizationDuration: pointer.String("15m"),
							MinChange: hvpav1alpha1.ScaleParams{
								CPU: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("1"),
									Percentage: pointer.Int32(80),
								},
								Memory: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("2G"),
									Percentage: pointer.Int32(80),
								},
							},
						},
						LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("2"),
								Percentage: pointer.Int32(40),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("5G"),
								Percentage: pointer.Int32(40),
							},
						},
						Template: hvpav1alpha1.VpaTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"role": "etcd-vpa-" + testRole,
								},
							},
							Spec: hvpav1alpha1.VpaTemplateSpec{
								ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
									ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
										{
											ContainerName: "etcd",
											MinAllowed: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("50m"),
												corev1.ResourceMemory: resource.MustParse("200M"),
											},
											MaxAllowed: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("4"),
												corev1.ResourceMemory: resource.MustParse("30G"),
											},
										},
										{
											ContainerName: "backup-restore",
											Mode:          &containerPolicyOff,
										},
									},
								},
							},
						},
					},
					WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{
						{
							VpaWeight:         hvpav1alpha1.VpaOnly,
							StartReplicaCount: replicas,
							LastReplicaCount:  replicas,
						},
					},
					TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       etcdName,
					},
				},
			}

			if class == ClassImportant {
				obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies[0].MinAllowed = corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("700M"),
				}
			}

			return obj
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		log = logger.NewNopLogger()
		etcd = New(c, log, testNamespace, testRole, class, retainReplicas, storageCapacity, &defragmentationSchedule)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			It("should return an error because the CA secret information is not provided", func() {
				Expect(etcd.Deploy(ctx)).To(MatchError(ContainSubstring("missing CA secret information")))
			})

			It("should return an error because the server secret information is not provided", func() {
				etcd.SetSecrets(Secrets{CA: component.Secret{Name: secretNameCA, Checksum: secretChecksumCA}})
				Expect(etcd.Deploy(ctx)).To(MatchError(ContainSubstring("missing server secret information")))
			})

			It("should return an error because the client secret information is not provided", func() {
				etcd.SetSecrets(Secrets{CA: component.Secret{Name: secretNameCA, Checksum: secretChecksumCA}, Server: component.Secret{Name: secretNameServer, Checksum: secretChecksumServer}})
				Expect(etcd.Deploy(ctx)).To(MatchError(ContainSubstring("missing client secret information")))
			})
		})

		Context("secret information available", func() {
			var scaleDownUpdateMode = hvpav1alpha1.UpdateModeMaintenanceWindow
			getSetSecretsAndHVPAConfigFunc := func(updateMode string) func() {
				return func() {
					etcd.SetSecrets(secrets)
					etcd.SetHVPAConfig(&HVPAConfig{
						Enabled:               true,
						MaintenanceTimeWindow: maintenanceTimeWindow,
						ScaleDownUpdateMode:   pointer.String(updateMode),
					})
				}
			}
			setSecretsAndHVPAConfig := getSetSecretsAndHVPAConfigFunc(scaleDownUpdateMode)

			BeforeEach(setSecretsAndHVPAConfig)

			It("should fail because the etcd object retrieval fails", func() {
				c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(fakeErr)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the statefulset object retrieval fails (using the default sts name)", func() {
				c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(fakeErr)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the statefulset object retrieval fails (using the sts name from etcd object)", func() {
				statefulSetName := "sts-name"

				c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
					func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
						(&druidv1alpha1.Etcd{
							Status: druidv1alpha1.EtcdStatus{
								Etcd: druidv1alpha1.CrossVersionObjectReference{
									Name: statefulSetName,
								},
							},
						}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
						return nil
					},
				)
				c.EXPECT().Get(ctx, kutil.Key(testNamespace, statefulSetName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(fakeErr)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the network policy cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Return(fakeErr),
				)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the etcd cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Return(fakeErr),
				)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the hvpa cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Return(fakeErr),
				)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the hvpa cannot be deleted", func() {
				etcd.SetHVPAConfig(&HVPAConfig{})

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()),
					c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})).Return(fakeErr),
				)

				Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy (normal etcd)", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(networkPolicy))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							1,
							nil,
							"",
							"",
							nil,
							nil,
						)))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(hvpaFor(class, 1, scaleDownUpdateMode)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy (normal etcd) and retain replicas (etcd found)", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				var (
					existingReplicas int32 = 245
					retainReplicas         = true
				)

				etcd = New(c, log, testNamespace, testRole, class, retainReplicas, storageCapacity, &defragmentationSchedule)
				setSecretsAndHVPAConfig()

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
						(&druidv1alpha1.Etcd{
							ObjectMeta: metav1.ObjectMeta{
								Name:      etcdName,
								Namespace: testNamespace,
							},
							Spec: druidv1alpha1.EtcdSpec{
								Replicas: int(existingReplicas),
							},
							Status: druidv1alpha1.EtcdStatus{
								Etcd: druidv1alpha1.CrossVersionObjectReference{
									Name: etcdName,
								},
							},
						}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(networkPolicy))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj *druidv1alpha1.Etcd, _ client.Patch, _ ...client.PatchOption) {
						// ignore status when comparing
						obj.Status = druidv1alpha1.EtcdStatus{}

						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							int(existingReplicas),
							nil,
							"",
							"",
							nil,
							nil,
						)))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(hvpaFor(class, existingReplicas, scaleDownUpdateMode)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy (normal etcd) and keep the existing defragmentation schedule", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				existingDefragmentationSchedule := "foobardefragexisting"

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
						(&druidv1alpha1.Etcd{
							ObjectMeta: metav1.ObjectMeta{
								Name:      etcdName,
								Namespace: testNamespace,
							},
							Spec: druidv1alpha1.EtcdSpec{
								Etcd: druidv1alpha1.EtcdConfig{
									DefragmentationSchedule: &existingDefragmentationSchedule,
								},
							},
							Status: druidv1alpha1.EtcdStatus{
								Etcd: druidv1alpha1.CrossVersionObjectReference{
									Name: etcdName,
								},
							},
						}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(networkPolicy))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj *druidv1alpha1.Etcd, _ client.Patch, _ ...client.PatchOption) {
						// ignore status when comparing
						obj.Status = druidv1alpha1.EtcdStatus{}

						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							1,
							nil,
							existingDefragmentationSchedule,
							"",
							nil,
							nil,
						)))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(hvpaFor(class, 1, scaleDownUpdateMode)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy (normal etcd) and keep the existing resource settings to not interfer with HVPA controller", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				var (
					existingResourcesContainerEtcd = corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2G"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("4G"),
						},
					}
					existingResourcesContainerBackupRestore = corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("5"),
							corev1.ResourceMemory: resource.MustParse("6G"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("7"),
							corev1.ResourceMemory: resource.MustParse("8G"),
						},
					}
				)

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).DoAndReturn(func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
						(&appsv1.StatefulSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      etcdName,
								Namespace: testNamespace,
							},
							Spec: appsv1.StatefulSetSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:      "etcd",
												Resources: existingResourcesContainerEtcd,
											},
											{
												Name:      "backup-restore",
												Resources: existingResourcesContainerBackupRestore,
											},
										},
									},
								},
							},
						}).DeepCopyInto(obj.(*appsv1.StatefulSet))
						return nil
					}),

					c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(networkPolicy))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							1,
							nil,
							"",
							"",
							&existingResourcesContainerEtcd,
							&existingResourcesContainerBackupRestore,
						)))
					}),
					c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(hvpaFor(class, 1, scaleDownUpdateMode)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})

			for _, shootPurpose := range []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeEvaluation, gardencorev1beta1.ShootPurposeProduction} {
				var purpose = shootPurpose
				It(fmt.Sprintf("should successfully deploy (important etcd): purpose = %q", purpose), func() {
					oldTimeNow := TimeNow
					defer func() { TimeNow = oldTimeNow }()
					TimeNow = func() time.Time { return now }

					class := ClassImportant

					updateMode := hvpav1alpha1.UpdateModeMaintenanceWindow
					if purpose == gardencorev1beta1.ShootPurposeProduction {
						updateMode = hvpav1alpha1.UpdateModeOff
					}

					etcd = New(c, log, testNamespace, testRole, class, retainReplicas, storageCapacity, &defragmentationSchedule)
					getSetSecretsAndHVPAConfigFunc(updateMode)()

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

						c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(networkPolicy))
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(etcdObjFor(
								class,
								1,
								nil,
								"",
								"",
								nil,
								nil,
							)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(hvpaFor(class, 1, updateMode)))
						}),
					)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				})
			}

			Context("with backup", func() {
				var backupConfig = &BackupConfig{
					Provider:             "prov",
					SecretRefName:        "secret-key",
					Prefix:               "prefix",
					Container:            "bucket",
					FullSnapshotSchedule: "1234",
				}

				BeforeEach(func() {
					etcd.SetBackupConfig(backupConfig)
				})

				It("should successfully deploy (with backup)", func() {
					oldTimeNow := TimeNow
					defer func() { TimeNow = oldTimeNow }()
					TimeNow = func() time.Time { return now }

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

						c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(networkPolicy))
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(etcdObjFor(
								class,
								1,
								backupConfig,
								"",
								"",
								nil,
								nil,
							)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(hvpaFor(class, 1, scaleDownUpdateMode)))
						}),
					)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				})

				It("should successfully deploy (with backup) and keep the existing backup schedule", func() {
					oldTimeNow := TimeNow
					defer func() { TimeNow = oldTimeNow }()
					TimeNow = func() time.Time { return now }

					existingBackupSchedule := "foobarbackupexisting"

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
							(&druidv1alpha1.Etcd{
								ObjectMeta: metav1.ObjectMeta{
									Name:      etcdName,
									Namespace: testNamespace,
								},
								Spec: druidv1alpha1.EtcdSpec{
									Backup: druidv1alpha1.BackupSpec{
										FullSnapshotSchedule: &existingBackupSchedule,
									},
								},
								Status: druidv1alpha1.EtcdStatus{
									Etcd: druidv1alpha1.CrossVersionObjectReference{
										Name: "",
									},
								},
							}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
							return nil
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),

						c.EXPECT().Get(ctx, kutil.Key(testNamespace, networkPolicyName), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(networkPolicy))
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, etcdName), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(etcdObjFor(
								class,
								1,
								backupConfig,
								"",
								existingBackupSchedule,
								nil,
								nil,
							)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(testNamespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(hvpaFor(class, 1, scaleDownUpdateMode)))
						}),
					)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should properly delete all expected objects", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: networkPolicyName, Namespace: testNamespace}}),
			)

			Expect(etcd.Destroy(ctx)).To(Succeed())
		})

		It("should fail when the hvpa deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the etcd deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the network policy deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: networkPolicyName, Namespace: testNamespace}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})
	})

	Describe("#ServiceDNSNames", func() {
		It("should return the expected DNS names", func() {
			Expect(etcd.ServiceDNSNames()).To(ConsistOf(
				"etcd-"+testRole+"-local",
				"etcd-"+testRole+"-client",
				"etcd-"+testRole+"-client."+testNamespace,
				"etcd-"+testRole+"-client."+testNamespace+".svc",
				"etcd-"+testRole+"-client."+testNamespace+".svc.cluster.local",
			))
		})
	})

	Describe("#Snapshot", func() {
		It("should return an error when the backup config is nil", func() {
			Expect(etcd.Snapshot(ctx, nil)).To(MatchError(ContainSubstring("no backup is configured")))
		})

		Context("w/ backup configuration", func() {
			var (
				podExecutor *mockkubernetes.MockPodExecutor
				podName     = "some-etcd-pod"
				selector    = labels.SelectorFromSet(map[string]string{
					"app":  "etcd-statefulset",
					"role": testRole,
				})
			)

			BeforeEach(func() {
				etcd.SetBackupConfig(&BackupConfig{})
				podExecutor = mockkubernetes.NewMockPodExecutor(ctrl)
			})

			It("should successfully execute the full snapshot command", func() {
				podList := &corev1.PodList{
					Items: []corev1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: podName,
							},
						},
					},
				}

				c.EXPECT().List(
					ctx,
					gomock.AssignableToTypeOf(&corev1.PodList{}),
					client.InNamespace(testNamespace),
					client.MatchingLabelsSelector{Selector: selector},
				).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						podList.DeepCopyInto(list.(*corev1.PodList))
						return nil
					},
				)

				podExecutor.EXPECT().Execute(
					testNamespace,
					podName,
					"backup-restore",
					"/bin/sh",
					"curl -k https://etcd-"+testRole+"-local:8080/snapshot/full",
				)

				Expect(etcd.Snapshot(ctx, podExecutor)).To(Succeed())
			})

			It("should return an error when the pod listing fails", func() {
				c.EXPECT().List(
					ctx,
					gomock.AssignableToTypeOf(&corev1.PodList{}),
					client.InNamespace(testNamespace),
					client.MatchingLabelsSelector{Selector: selector},
				).Return(fakeErr)

				Expect(etcd.Snapshot(ctx, podExecutor)).To(MatchError(fakeErr))
			})

			It("should return an error when the pod list is empty", func() {
				podList := &corev1.PodList{}

				c.EXPECT().List(
					ctx,
					gomock.AssignableToTypeOf(&corev1.PodList{}),
					client.InNamespace(testNamespace),
					client.MatchingLabelsSelector{Selector: selector},
				).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						podList.DeepCopyInto(list.(*corev1.PodList))
						return nil
					},
				)

				Expect(etcd.Snapshot(ctx, podExecutor)).To(MatchError(ContainSubstring("didn't find any pods")))
			})

			It("should return an error when the pod list is too large", func() {
				podList := &corev1.PodList{
					Items: []corev1.Pod{
						{},
						{},
					},
				}

				c.EXPECT().List(
					ctx,
					gomock.AssignableToTypeOf(&corev1.PodList{}),
					client.InNamespace(testNamespace),
					client.MatchingLabelsSelector{Selector: selector},
				).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						podList.DeepCopyInto(list.(*corev1.PodList))
						return nil
					},
				)

				Expect(etcd.Snapshot(ctx, podExecutor)).To(MatchError(ContainSubstring("multiple ETCD Pods found")))
			})

			It("should return an error when the execution command fails", func() {
				podList := &corev1.PodList{
					Items: []corev1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: podName,
							},
						},
					},
				}

				c.EXPECT().List(
					ctx,
					gomock.AssignableToTypeOf(&corev1.PodList{}),
					client.InNamespace(testNamespace),
					client.MatchingLabelsSelector{Selector: selector},
				).DoAndReturn(
					func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
						podList.DeepCopyInto(obj.(*corev1.PodList))
						return nil
					},
				)

				podExecutor.EXPECT().Execute(
					testNamespace,
					podName,
					"backup-restore",
					"/bin/sh",
					"curl -k https://etcd-"+testRole+"-local:8080/snapshot/full",
				).Return(nil, fakeErr)

				Expect(etcd.Snapshot(ctx, podExecutor)).To(MatchError(fakeErr))
			})
		})
	})
})
