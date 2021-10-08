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

package botanist_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockinfrastructure "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/infrastructure/mock"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/mock"
	mockworker "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/worker/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Worker", func() {
	var (
		ctrl                  *gomock.Controller
		c                     *mockclient.MockClient
		worker                *mockworker.MockInterface
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		infrastructure        *mockinfrastructure.MockInterface
		botanist              *Botanist

		ctx                                   = context.TODO()
		fakeErr                               = fmt.Errorf("fake")
		shootState                            = &gardencorev1alpha1.ShootState{}
		sshPublicKey                          = []byte("key")
		infrastructureProviderStatus          = &runtime.RawExtension{Raw: []byte("infrastatus")}
		workerNameToOperatingSystemConfigMaps = map[string]*operatingsystemconfig.OperatingSystemConfigs{"foo": {}}
		labelSelectorCloudConfigRole          = client.MatchingLabels{"gardener.cloud/role": "cloud-config"}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		worker = mockworker.NewMockInterface(ctrl)
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)
		infrastructure = mockinfrastructure.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Infrastructure:        infrastructure,
						OperatingSystemConfig: operatingSystemConfig,
						Worker:                worker,
					},
				},
			},
		}}
		botanist.SetShootState(shootState)
		botanist.StoreSecret("ssh-keypair", &corev1.Secret{Data: map[string][]byte{"id_rsa.pub": sshPublicKey}})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployWorker", func() {
		BeforeEach(func() {
			infrastructure.EXPECT().ProviderStatus().Return(infrastructureProviderStatus)
			operatingSystemConfig.EXPECT().WorkerNameToOperatingSystemConfigsMap().Return(workerNameToOperatingSystemConfigMaps)

			worker.EXPECT().SetSSHPublicKey(sshPublicKey)
			worker.EXPECT().SetInfrastructureProviderStatus(infrastructureProviderStatus)
			worker.EXPECT().SetWorkerNameToOperatingSystemConfigsMap(workerNameToOperatingSystemConfigMaps)
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
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
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

	Describe("#WorkerPoolToCloudConfigSecretChecksumMap", func() {
		It("should return an error when the list fails", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), labelSelectorCloudConfigRole).Return(fakeErr)

			workerPoolToCloudConfigSecretChecksum, err := WorkerPoolToCloudConfigSecretChecksumMap(ctx, c)
			Expect(workerPoolToCloudConfigSecretChecksum).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return an empty map when there are no secrets", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), labelSelectorCloudConfigRole)

			workerPoolToCloudConfigSecretChecksum, err := WorkerPoolToCloudConfigSecretChecksumMap(ctx, c)
			Expect(workerPoolToCloudConfigSecretChecksum).To(BeEmpty())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a map of cloud-config secret checksums per worker pool if the label and the annotation are present", func() {
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

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), labelSelectorCloudConfigRole).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
				*list = corev1.SecretList{Items: []corev1.Secret{secret1, secret2, secret3WithoutLabel, secret4WithoutAnnotations}}
				return nil
			})

			workerPoolToCloudConfigSecretChecksum, err := WorkerPoolToCloudConfigSecretChecksumMap(ctx, c)
			Expect(workerPoolToCloudConfigSecretChecksum).To(Equal(map[string]string{
				pool1: checksum1,
				pool2: checksum2,
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	DescribeTable("#CloudConfigUpdatedForAllWorkerPools",
		func(workers []gardencorev1beta1.Worker, workerPoolToNodes map[string][]corev1.Node, workerPoolToCloudConfigSecretChecksum map[string]string, matcher gomegatypes.GomegaMatcher) {
			Expect(CloudConfigUpdatedForAllWorkerPools(workers, workerPoolToNodes, workerPoolToCloudConfigSecretChecksum)).To(matcher)
		},

		Entry("secret checksum missing",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			nil,
			nil,
			BeNil(),
		),
		Entry("annotation outdated",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			map[string][]corev1.Node{"pool1": {{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"checksum/cloud-config-data": "outdated"}}}}},
			map[string]string{"pool1": "foo"},
			MatchError(ContainSubstring("is outdated")),
		),
		Entry("skip node in deletion",
			[]gardencorev1beta1.Worker{{Name: "pool1"}},
			map[string][]corev1.Node{"pool1": {{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"checksum/cloud-config-data": "outdated"}}, Spec: corev1.NodeSpec{Taints: []corev1.Taint{{Key: MCMPreferNoScheduleKey, Effect: corev1.TaintEffectPreferNoSchedule}}}}}},
			map[string]string{"pool1": "foo"},
			BeNil(),
		),
		Entry("everything up-to-date",
			[]gardencorev1beta1.Worker{{Name: "pool1"}, {Name: "pool2"}},
			map[string][]corev1.Node{
				"pool1": {{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"checksum/cloud-config-data": "uptodate1"}}}},
				"pool2": {{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"checksum/cloud-config-data": "uptodate2"}}}},
			},
			map[string]string{"pool1": "uptodate1", "pool2": "uptodate2"},
			BeNil(),
		),
	)

	Describe("#WaitUntilCloudConfigUpdatedForAllWorkerPools", func() {
		var (
			seedInterface  *mockkubernetes.MockInterface
			seedClient     *mockclient.MockClient
			shootInterface *mockkubernetes.MockInterface
			shootClient    *mockclient.MockClient

			namespace = "shoot--foo--bar"
			name      = "shoot-cloud-config-execution"
		)

		BeforeEach(func() {
			botanist = &Botanist{Operation: &operation.Operation{
				Shoot: &shootpkg.Shoot{
					SeedNamespace: namespace,
				},
			}}

			seedInterface = mockkubernetes.NewMockInterface(ctrl)
			seedClient = mockclient.NewMockClient(ctrl)
			botanist.K8sSeedClient = seedInterface

			shootInterface = mockkubernetes.NewMockInterface(ctrl)
			shootClient = mockclient.NewMockClient(ctrl)
			botanist.K8sShootClient = shootInterface
		})

		It("should fail when the cloud-config user data script secret was not updated yet", func() {
			oldInterval := IntervalWaitCloudConfigUpdated
			defer func() { IntervalWaitCloudConfigUpdated = oldInterval }()
			IntervalWaitCloudConfigUpdated = time.Millisecond

			oldTimeout := TimeoutWaitCloudConfigUpdated
			defer func() { TimeoutWaitCloudConfigUpdated = oldTimeout }()
			TimeoutWaitCloudConfigUpdated = 5 * time.Millisecond

			gomock.InOrder(
				seedInterface.EXPECT().Client().Return(seedClient),
				seedClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{Generation: 2},
					Status:     resourcesv1alpha1.ManagedResourceStatus{ObservedGeneration: 1},
				})).AnyTimes(),
			)

			Expect(botanist.WaitUntilCloudConfigUpdatedForAllWorkerPools(ctx)).To(MatchError(ContainSubstring("the cloud-config user data scripts for the worker nodes were not populated yet")))
		})

		It("should fail when the cloud-config was not updated for all worker pools", func() {
			oldInterval := IntervalWaitCloudConfigUpdated
			defer func() { IntervalWaitCloudConfigUpdated = oldInterval }()
			IntervalWaitCloudConfigUpdated = time.Millisecond

			oldTimeout := TimeoutWaitCloudConfigUpdated
			defer func() { TimeoutWaitCloudConfigUpdated = oldTimeout }()
			TimeoutWaitCloudConfigUpdated = time.Millisecond

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
				seedClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
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
							Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1"},
							Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
						},
					}}}
					return nil
				}).AnyTimes(),
				shootInterface.EXPECT().Client().Return(shootClient).AnyTimes(),
				shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), labelSelectorCloudConfigRole).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{Items: []corev1.Secret{{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1"},
							Annotations: map[string]string{"checksum/data-script": "bar"},
						},
					}}}
					return nil
				}).AnyTimes(),
			)

			Expect(botanist.WaitUntilCloudConfigUpdatedForAllWorkerPools(ctx)).To(MatchError(ContainSubstring("is outdated")))
		})

		It("should succeed when the cloud-config was updated for all worker pools", func() {
			oldInterval := IntervalWaitCloudConfigUpdated
			defer func() { IntervalWaitCloudConfigUpdated = oldInterval }()
			IntervalWaitCloudConfigUpdated = time.Millisecond

			oldTimeout := TimeoutWaitCloudConfigUpdated
			defer func() { TimeoutWaitCloudConfigUpdated = oldTimeout }()
			TimeoutWaitCloudConfigUpdated = time.Millisecond

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
				seedClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
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
							Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1"},
							Annotations: map[string]string{"checksum/cloud-config-data": "foo"},
						},
					}}}
					return nil
				}).AnyTimes(),
				shootInterface.EXPECT().Client().Return(shootClient).AnyTimes(),
				shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), labelSelectorCloudConfigRole).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{Items: []corev1.Secret{{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{"worker.gardener.cloud/pool": "pool1"},
							Annotations: map[string]string{"checksum/data-script": "foo"},
						},
					}}}
					return nil
				}).AnyTimes(),
			)

			Expect(botanist.WaitUntilCloudConfigUpdatedForAllWorkerPools(ctx)).To(Succeed())
		})
	})
})

func clientGet(managedResource *resourcesv1alpha1.ManagedResource) interface{} {
	return func(_ context.Context, _ client.ObjectKey, mr *resourcesv1alpha1.ManagedResource) error {
		*mr = *managedResource
		return nil
	}
}
