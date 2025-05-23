// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/component/extensions/infrastructure/mock"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	mockworker "github.com/gardener/gardener/pkg/component/extensions/worker/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Worker", func() {
	var (
		ctrl                  *gomock.Controller
		c                     *mockclient.MockClient
		fakeClient            client.Client
		sm                    secretsmanager.Interface
		worker                *mockworker.MockInterface
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		infrastructure        *mockinfrastructure.MockInterface
		botanist              *Botanist

		ctx                                    = context.TODO()
		namespace                              = "namespace"
		fakeErr                                = errors.New("fake")
		shootState                             = &gardencorev1beta1.ShootState{}
		infrastructureProviderStatus           = &runtime.RawExtension{Raw: []byte("infrastatus")}
		workerNameToOperatingSystemConfigMaps  = map[string]*operatingsystemconfig.OperatingSystemConfigs{"foo": {}}
		operatingSystemConfigSecretListOptions = []client.ListOption{
			client.InNamespace(metav1.NamespaceSystem),
			client.MatchingLabels{"gardener.cloud/role": "operating-system-config"},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair", Namespace: namespace}})).To(Succeed())

		worker = mockworker.NewMockInterface(ctrl)
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)
		infrastructure = mockinfrastructure.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				SecretsManager: sm,
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{
							Infrastructure:        infrastructure,
							OperatingSystemConfig: operatingSystemConfig,
							Worker:                worker,
						},
					},
				},
			},
		}
		botanist.Shoot.SetShootState(shootState)
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "foo"},
					},
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployWorker", func() {
		BeforeEach(func() {
			infrastructure.EXPECT().ProviderStatus().Return(infrastructureProviderStatus)
			operatingSystemConfig.EXPECT().WorkerPoolNameToOperatingSystemConfigsMap().Return(workerNameToOperatingSystemConfigMaps)

			worker.EXPECT().SetSSHPublicKey(gomock.Any())
			worker.EXPECT().SetInfrastructureProviderStatus(infrastructureProviderStatus)
			worker.EXPECT().SetWorkerPoolNameToOperatingSystemConfigsMap(workerNameToOperatingSystemConfigMaps)
		})

		Context("deploy", func() {
			It("should deploy successfully", func() {
				worker.EXPECT().Deploy(ctx)
				Expect(botanist.DeployWorker(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				worker.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployWorker(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				shoot := botanist.Shoot.GetInfo()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
				botanist.Shoot.SetInfo(shoot)
			})

			It("should restore successfully", func() {
				worker.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployWorker(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				worker.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployWorker(ctx)).To(MatchError(fakeErr))
			})
		})
	})

	Describe("#WorkerPoolToNodesMap", func() {
		It("should return an error when the list fails", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).Return(fakeErr)

			workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, c)
			Expect(workerPoolToNodes).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return an empty map when there are no nodes", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{}))

			workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, c)
			Expect(workerPoolToNodes).To(BeEmpty())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a map of nodes per worker pool if the label is present", func() {
			var (
				pool1 = "pool1"
				pool2 = "pool2"
				node1 = corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"worker.gardener.cloud/pool": pool1},
					},
				}
				node2 = corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"worker.gardener.cloud/pool": pool2},
					},
				}
				node3 = corev1.Node{}
			)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
				*list = corev1.NodeList{Items: []corev1.Node{node1, node2, node3}}
				return nil
			})

			workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, c)
			Expect(workerPoolToNodes).To(Equal(map[string][]corev1.Node{
				pool1: {node1},
				pool2: {node2},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#WorkerPoolToOperatingSystemConfigSecretMetaMap", func() {
		It("should return an error when the list fails", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), operatingSystemConfigSecretListOptions).Return(fakeErr)

			workerPoolToCloudConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, c, "operating-system-config")
			Expect(workerPoolToCloudConfigSecretMeta).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return an empty map when there are no secrets", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), operatingSystemConfigSecretListOptions)

			workerPoolToCloudConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, c, "operating-system-config")
			Expect(workerPoolToCloudConfigSecretMeta).To(BeEmpty())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a map of secret checksums per worker pool if the label and the annotation are present", func() {
			var (
				pool1     = "pool1"
				pool2     = "pool1"
				checksum1 = "foo"
				checksum2 = "bar"

				secret1 = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{"worker.gardener.cloud/pool": pool1},
						Annotations: map[string]string{"checksum/data-script": checksum1},
					},
				}
				secret2 = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{"worker.gardener.cloud/pool": pool2},
						Annotations: map[string]string{"checksum/data-script": checksum2},
					},
				}
				secret3WithoutLabel = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{"checksum/data-script": "baz"},
					},
				}
				secret4WithoutAnnotations = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"worker.gardener/cloud": pool2},
					},
				}
			)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), operatingSystemConfigSecretListOptions).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
				*list = corev1.SecretList{Items: []corev1.Secret{secret1, secret2, secret3WithoutLabel, secret4WithoutAnnotations}}
				return nil
			})

			workerPoolToCloudConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, c, "operating-system-config")
			Expect(workerPoolToCloudConfigSecretMeta).To(Equal(map[string]metav1.ObjectMeta{
				pool1: {
					Labels:      map[string]string{"worker.gardener.cloud/pool": pool1},
					Annotations: map[string]string{"checksum/data-script": checksum1},
				},
				pool2: {
					Labels:      map[string]string{"worker.gardener.cloud/pool": pool2},
					Annotations: map[string]string{"checksum/data-script": checksum2},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	DescribeTable("#OperatingSystemConfigUpdatedForAllWorkerPools",
		func(workers []gardencorev1beta1.Worker, workerPoolToNodes map[string][]corev1.Node, workerPoolToCloudConfigSecretMeta map[string]metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
			Expect(OperatingSystemConfigUpdatedForAllWorkerPools(workers, workerPoolToNodes, workerPoolToCloudConfigSecretMeta)).To(matcher)
		},

		Entry("secret meta missing",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			nil,
			nil,
			MatchError(ContainSubstring("missing operating system config secret metadata")),
		),
		Entry("checksum annotation missing",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			map[string][]corev1.Node{"pool1": {{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"worker.gardener.cloud/kubernetes-version":              "1.24.0",
					"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent--c63c0",
				},
			}}}},
			map[string]metav1.ObjectMeta{"pool1": {
				Name:        "gardener-node-agent--c63c0",
				Annotations: map[string]string{"checksum/data-script": "foo"},
			}},
			MatchError(ContainSubstring("hasn't been reported yet")),
		),
		Entry("checksum annotation outdated",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			map[string][]corev1.Node{"pool1": {{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"checksum/cloud-config-data": "outdated"},
				Labels: map[string]string{
					"worker.gardener.cloud/kubernetes-version":              "1.24.0",
					"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent--c63c0",
				},
			}}}},
			map[string]metav1.ObjectMeta{"pool1": {
				Name:        "gardener-node-agent--c63c0",
				Annotations: map[string]string{"checksum/data-script": "foo"},
			}},
			MatchError(ContainSubstring("is outdated")),
		),
		Entry("skip node marked by MCM for termination",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			map[string][]corev1.Node{"pool1": {{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"checksum/cloud-config-data": "outdated"},
					Labels: map[string]string{
						"worker.gardener.cloud/kubernetes-version":              "1.24.0",
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent--c63c0",
					},
				},
				Spec: corev1.NodeSpec{Taints: []corev1.Taint{{Key: "deployment.machine.sapcloud.io/prefer-no-schedule", Effect: corev1.TaintEffectPreferNoSchedule}}},
			}}},
			map[string]metav1.ObjectMeta{"pool1": {
				Name:        "gardener-node-agent--c63c0",
				Annotations: map[string]string{"checksum/data-script": "foo"},
			}},
			BeNil(),
		),
		Entry("skip node whose OSC key does not match secret OSC key",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			map[string][]corev1.Node{"pool1": {{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
				Labels: map[string]string{
					"worker.gardener.cloud/kubernetes-version":              "1.24.0",
					"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent--c63c0",
				},
			}}}},
			map[string]metav1.ObjectMeta{"pool1": {
				Name:        "gardener-node-agent--c63c1",
				Annotations: map[string]string{"checksum/data-script": "foo"},
			}},
			BeNil(),
		),
		Entry("everything up-to-date",
			[]gardencorev1beta1.Worker{{Name: "pool1"}, {Name: "pool2"}},
			map[string][]corev1.Node{
				"pool1": {{ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"checksum/cloud-config-data": "uptodate1"},
					Labels: map[string]string{
						"worker.gardener.cloud/kubernetes-version":              "1.26.0",
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent--c63c0",
					},
				}}},
				"pool2": {{ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"checksum/cloud-config-data": "uptodate2"},
					Labels: map[string]string{
						"worker.gardener.cloud/kubernetes-version":              "1.25.0",
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent--5dcdf",
					},
				}}},
			},
			map[string]metav1.ObjectMeta{
				"pool1": {
					Name:        "gardener-node-agent--c63c0",
					Annotations: map[string]string{"checksum/data-script": "uptodate1"},
				},
				"pool2": {
					Name:        "gardener-node-agent--5dcdf",
					Annotations: map[string]string{"checksum/data-script": "uptodate2"},
				},
			},
			BeNil(),
		),
	)

	Describe("#WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools", func() {
		var (
			seedInterface  *kubernetesmock.MockInterface
			seedClient     *mockclient.MockClient
			shootInterface *kubernetesmock.MockInterface
			shootClient    *mockclient.MockClient

			controlPlaneNamespace = "shoot--foo--bar"
		)

		BeforeEach(func() {
			botanist = &Botanist{Operation: &operation.Operation{
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
				},
			}}

			seedInterface = kubernetesmock.NewMockInterface(ctrl)
			seedClient = mockclient.NewMockClient(ctrl)
			botanist.SeedClientSet = seedInterface

			shootInterface = kubernetesmock.NewMockInterface(ctrl)
			shootClient = mockclient.NewMockClient(ctrl)
			botanist.ShootClientSet = shootInterface
		})

		It("should fail when the operating system config secret was not updated yet", func() {
			DeferCleanup(test.WithVars(
				&IntervalWaitOperatingSystemConfigUpdated, time.Millisecond,
				&GetTimeoutWaitOperatingSystemConfigUpdated, func(*shootpkg.Shoot) time.Duration { return 5 * time.Millisecond },
			))

			gomock.InOrder(
				seedInterface.EXPECT().Client().Return(seedClient),
				seedClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: controlPlaneNamespace, Name: "shoot-gardener-node-agent"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{Generation: 2},
					Status:     resourcesv1alpha1.ManagedResourceStatus{ObservedGeneration: 1},
				})).AnyTimes(),
			)

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx)).To(MatchError(ContainSubstring("the operating system configs for the worker nodes were not populated yet")))
		})

		It("should fail when the operating system config was not updated for all worker pools", func() {
			DeferCleanup(test.WithVars(
				&IntervalWaitOperatingSystemConfigUpdated, time.Millisecond,
				&GetTimeoutWaitOperatingSystemConfigUpdated, func(*shootpkg.Shoot) time.Duration { return time.Millisecond },
			))

			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{Name: "pool1"},
						},
					},
				},
			})

			gomock.InOrder(
				seedInterface.EXPECT().Client().Return(seedClient),
				seedClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: controlPlaneNamespace, Name: "shoot-gardener-node-agent"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})),
				shootInterface.EXPECT().Client().Return(shootClient).AnyTimes(),
				shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					*list = corev1.NodeList{Items: []corev1.Node{{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"worker.gardener.cloud/pool":                            "pool1",
								"worker.gardener.cloud/kubernetes-version":              "1.24.0",
								"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool1-c63c0",
							},
							Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
						},
					}}}
					return nil
				}).AnyTimes(),
				shootInterface.EXPECT().Client().Return(shootClient).AnyTimes(),
				shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), operatingSystemConfigSecretListOptions).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{Items: []corev1.Secret{{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "gardener-node-agent-pool1-c63c0",
							Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1"},
							Annotations: map[string]string{"checksum/data-script": "bar"},
						},
					}}}
					return nil
				}).AnyTimes(),
			)

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx)).To(MatchError(ContainSubstring("is outdated")))
		})

		It("should succeed when the operating system config was updated for all worker pools", func() {
			DeferCleanup(test.WithVars(
				&IntervalWaitOperatingSystemConfigUpdated, time.Millisecond,
				&GetTimeoutWaitOperatingSystemConfigUpdated, func(*shootpkg.Shoot) time.Duration { return time.Millisecond },
			))

			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{Name: "pool1"},
						},
					},
				},
			})

			gomock.InOrder(
				seedInterface.EXPECT().Client().Return(seedClient),
				seedClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: controlPlaneNamespace, Name: "shoot-gardener-node-agent"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})),
				shootInterface.EXPECT().Client().Return(shootClient).AnyTimes(),
				shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					*list = corev1.NodeList{Items: []corev1.Node{{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"worker.gardener.cloud/pool":                            "pool1",
								"worker.gardener.cloud/kubernetes-version":              "1.26.0",
								"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool1-5dcdf",
							},
							Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
						},
					}}}
					return nil
				}).AnyTimes(),
				shootInterface.EXPECT().Client().Return(shootClient).AnyTimes(),
				shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), operatingSystemConfigSecretListOptions).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{Items: []corev1.Secret{{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "gardener-node-agent-pool1-5dcdf",
							Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1"},
							Annotations: map[string]string{"checksum/data-script": "foo"},
						},
					}}}
					return nil
				}).AnyTimes(),
			)

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx)).To(Succeed())
		})
	})
})

func clientGet(managedResource *resourcesv1alpha1.ManagedResource) any {
	return func(_ context.Context, _ client.ObjectKey, mr *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
		*mr = *managedResource
		return nil
	}
}
