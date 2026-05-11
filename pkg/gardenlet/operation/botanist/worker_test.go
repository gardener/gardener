// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"fmt"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
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
)

var _ = Describe("Worker", func() {
	var (
		ctrl                  *gomock.Controller
		c                     client.Client
		sm                    secretsmanager.Interface
		worker                *mockworker.MockInterface
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		infrastructure        *mockinfrastructure.MockInterface
		botanist              *Botanist

		ctx                                   = context.TODO()
		namespace                             = "namespace"
		fakeErr                               = errors.New("fake")
		shootState                            = &gardencorev1beta1.ShootState{}
		infrastructureProviderStatus          = &runtime.RawExtension{Raw: []byte("infrastatus")}
		workerNameToOperatingSystemConfigMaps = map[string]*operatingsystemconfig.OperatingSystemConfigs{"foo": {}}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair", Namespace: namespace}})).To(Succeed())

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
			c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
					return fakeErr
				},
			}).Build()

			workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, c)
			Expect(workerPoolToNodes).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return an empty map when there are no nodes", func() {
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
						Name:   "node1",
						Labels: map[string]string{"worker.gardener.cloud/pool": pool1},
					},
				}
				node2 = corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node2",
						Labels: map[string]string{"worker.gardener.cloud/pool": pool2},
					},
				}
				node3 = corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node3"},
				}
			)

			Expect(c.Create(ctx, &node1)).To(Succeed())
			Expect(c.Create(ctx, &node2)).To(Succeed())
			Expect(c.Create(ctx, &node3)).To(Succeed())

			workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			Expect(workerPoolToNodes).To(Equal(map[string][]corev1.Node{
				pool1: {node1},
				pool2: {node2},
			}))
		})
	})

	Describe("#WorkerPoolToOperatingSystemConfigSecretMetaMap", func() {
		It("should return an error when the list fails", func() {
			c := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
					return fakeErr
				},
			}).Build()

			workerPoolToCloudConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, c, "operating-system-config")
			Expect(workerPoolToCloudConfigSecretMeta).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return an empty map when there are no secrets", func() {
			workerPoolToCloudConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, c, "operating-system-config")
			Expect(workerPoolToCloudConfigSecretMeta).To(BeEmpty())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a map of secret checksums per worker pool if the label and the annotation are present", func() {
			var (
				pool1     = "pool1"
				pool2     = "pool2"
				checksum1 = "foo"
				checksum2 = "bar"

				secret1 = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: metav1.NamespaceSystem,
						Labels: map[string]string{
							"worker.gardener.cloud/pool": pool1,
							"gardener.cloud/role":        "operating-system-config",
						},
						Annotations: map[string]string{"checksum/data-script": checksum1},
					},
				}
				secret2 = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret2",
						Namespace: metav1.NamespaceSystem,
						Labels: map[string]string{
							"worker.gardener.cloud/pool": pool2,
							"gardener.cloud/role":        "operating-system-config",
						},
						Annotations: map[string]string{"checksum/data-script": checksum2},
					},
				}
				secret3WithoutLabel = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret3",
						Namespace:   metav1.NamespaceSystem,
						Labels:      map[string]string{"gardener.cloud/role": "operating-system-config"},
						Annotations: map[string]string{"checksum/data-script": "baz"},
					},
				}
				secret4WithoutAnnotations = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret4",
						Namespace: metav1.NamespaceSystem,
						Labels: map[string]string{
							"worker.gardener/cloud": pool2,
							"gardener.cloud/role":   "operating-system-config",
						},
					},
				}
			)

			Expect(c.Create(ctx, &secret1)).To(Succeed())
			Expect(c.Create(ctx, &secret2)).To(Succeed())
			Expect(c.Create(ctx, &secret3WithoutLabel)).To(Succeed())
			Expect(c.Create(ctx, &secret4WithoutAnnotations)).To(Succeed())

			workerPoolToCloudConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, c, "operating-system-config")
			Expect(err).NotTo(HaveOccurred())
			Expect(workerPoolToCloudConfigSecretMeta).To(Equal(map[string]metav1.ObjectMeta{
				pool1: {
					Name:            "secret1",
					Namespace:       metav1.NamespaceSystem,
					ResourceVersion: "1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": pool1,
						"gardener.cloud/role":        "operating-system-config",
					},
					Annotations: map[string]string{"checksum/data-script": checksum1},
				},
				pool2: {
					Name:            "secret2",
					Namespace:       metav1.NamespaceSystem,
					ResourceVersion: "1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": pool2,
						"gardener.cloud/role":        "operating-system-config"},
					Annotations: map[string]string{"checksum/data-script": checksum2},
				},
			}))
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
			seedFakeClient  client.Client
			seedInterface   *fakekubernetes.ClientSet
			shootFakeClient client.Client
			shootInterface  *fakekubernetes.ClientSet

			controlPlaneNamespace = "shoot--foo--bar"
		)

		BeforeEach(func() {
			botanist = &Botanist{Operation: &operation.Operation{
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
				},
			}}

			seedFakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			shootFakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			seedInterface = fakekubernetes.NewClientSetBuilder().WithClient(seedFakeClient).Build()
			botanist.SeedClientSet = seedInterface

			shootInterface = fakekubernetes.NewClientSetBuilder().WithClient(shootFakeClient).Build()
			botanist.ShootClientSet = shootInterface
		})

		It("should fail when the operating system config secret was not updated yet", func() {
			DeferCleanup(test.WithVars(
				&IntervalWaitOperatingSystemConfigUpdated, time.Millisecond,
				&GetTimeoutWaitOperatingSystemConfigUpdated, func(*shootpkg.Shoot) time.Duration { return 5 * time.Millisecond },
			))

			// Create ManagedResource with Generation != ObservedGeneration (not populated yet)
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "shoot-gardener-node-agent",
					Namespace:  controlPlaneNamespace,
					Generation: 2,
				},
			}
			Expect(seedFakeClient.Create(ctx, mr)).To(Succeed())

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, false)).To(MatchError(ContainSubstring("the operating system configs for the worker nodes were not populated yet")))
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

			// Create ManagedResource with matching Generation/ObservedGeneration and healthy conditions
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "shoot-gardener-node-agent",
					Namespace:  controlPlaneNamespace,
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
			}
			Expect(seedFakeClient.Create(ctx, mr)).To(Succeed())

			// Create a node with outdated checksum in the shoot client
			Expect(shootFakeClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool":                            "pool1",
						"worker.gardener.cloud/kubernetes-version":              "1.24.0",
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool1-c63c0",
					},
					Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
				},
			})).To(Succeed())

			// Create OSC secret with different checksum
			Expect(shootFakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "gardener-node-agent-pool1-c63c0",
					Namespace:   metav1.NamespaceSystem,
					Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1", "gardener.cloud/role": "operating-system-config"},
					Annotations: map[string]string{"checksum/data-script": "bar"},
				},
			})).To(Succeed())

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, false)).To(MatchError(ContainSubstring("is outdated")))
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

			// Create ManagedResource with matching Generation/ObservedGeneration and healthy conditions
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "shoot-gardener-node-agent",
					Namespace:  controlPlaneNamespace,
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
			}
			Expect(seedFakeClient.Create(ctx, mr)).To(Succeed())

			// Create node with matching checksum in shoot client
			Expect(shootFakeClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool":                            "pool1",
						"worker.gardener.cloud/kubernetes-version":              "1.26.0",
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool1-5dcdf",
					},
					Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
				},
			})).To(Succeed())

			// Create OSC secret with matching checksum
			Expect(shootFakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "gardener-node-agent-pool1-5dcdf",
					Namespace:   metav1.NamespaceSystem,
					Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1", "gardener.cloud/role": "operating-system-config"},
					Annotations: map[string]string{"checksum/data-script": "foo"},
				},
			})).To(Succeed())

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, false)).To(Succeed())
		})

		It("should succeed when tolerating errors and the managed resource becomes healthy after a transient error", func() {
			DeferCleanup(test.WithVars(
				&IntervalWaitOperatingSystemConfigUpdated, time.Millisecond,
				&GetTimeoutWaitOperatingSystemConfigUpdated, func(*shootpkg.Shoot) time.Duration { return time.Second },
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

			// Override seedFakeClient with an interceptor that simulates a transient connection error on the first Get
			getCallCount := 0
			seedFakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*resourcesv1alpha1.ManagedResource); ok {
							getCallCount++
							if getCallCount == 1 {
								return fmt.Errorf("dial tcp 10.2.0.99:443: connect: connection refused")
							}
						}
						return c.Get(ctx, key, obj, opts...)
					},
				}).
				Build()
			seedInterface = fakekubernetes.NewClientSetBuilder().WithClient(seedFakeClient).Build()
			botanist.SeedClientSet = seedInterface

			Expect(seedFakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "shoot-gardener-node-agent",
					Namespace:  controlPlaneNamespace,
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
			})).To(Succeed())

			Expect(shootFakeClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool":                            "pool1",
						"worker.gardener.cloud/kubernetes-version":              "1.26.0",
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool1-5dcdf",
					},
					Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
				},
			})).To(Succeed())

			Expect(shootFakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "gardener-node-agent-pool1-5dcdf",
					Namespace:   metav1.NamespaceSystem,
					Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1", "gardener.cloud/role": "operating-system-config"},
					Annotations: map[string]string{"checksum/data-script": "foo"},
				},
			})).To(Succeed())

			Expect(botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, true)).To(Succeed())
		})
	})
})
