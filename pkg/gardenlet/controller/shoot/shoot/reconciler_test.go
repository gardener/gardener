// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx           = context.TODO()
		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		namespace  *corev1.Namespace
		shoot      *gardencorev1beta1.Shoot
		reconciler *Reconciler
		fakeClock  *testclock.FakeClock
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-local"},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot",
				Namespace: "namespace",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "worker1", UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate)},
						{Name: "worker2", UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate)},
						{Name: "worker3", UpdateStrategy: new(gardencorev1beta1.AutoRollingUpdate)},
						{Name: "worker4", UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate)},
						{Name: "worker5", UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate)},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.33.0",
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: "shoot--namespace--shoot",
				Credentials: &gardencorev1beta1.ShootCredentials{
					Rotation: &gardencorev1beta1.ShootCredentialsRotation{
						CertificateAuthorities: &gardencorev1beta1.CARotation{
							PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
								{Name: "worker1"},
								{Name: "worker3"},
								{Name: "worker5"},
							},
						},
						ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
							PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
								{Name: "worker2"},
								{Name: "worker3"},
								{Name: "worker4"},
							},
						},
					},
				},
				InPlaceUpdates: &gardencorev1beta1.InPlaceUpdatesStatus{
					PendingWorkerUpdates: &gardencorev1beta1.PendingWorkerUpdates{
						AutoInPlaceUpdate:   []string{"worker1", "worker4"},
						ManualInPlaceUpdate: []string{"worker2", "worker5"},
					},
				},
				ManualWorkerPoolRollout: &gardencorev1beta1.ManualWorkerPoolRollout{
					PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
						{Name: "worker1"},
						{Name: "worker3"},
					},
				},
			},
		}
	})

	Describe("#removeNonExistentPoolsFromPendingWorkersRollouts", func() {
		It("should remove non-existent pools from pending workers rollouts", func() {
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
				{Name: "worker3", UpdateStrategy: new(gardencorev1beta1.AutoRollingUpdate)},
				{Name: "worker4", UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate)},
				{Name: "worker5", UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate)},
			}

			removeNonExistentPoolsFromPendingWorkersRollouts(shoot, false)

			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts).To(ConsistOf(
				gardencorev1beta1.PendingWorkersRollout{Name: "worker3"},
				gardencorev1beta1.PendingWorkersRollout{Name: "worker5"},
			))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts).To(ConsistOf(
				gardencorev1beta1.PendingWorkersRollout{Name: "worker3"},
				gardencorev1beta1.PendingWorkersRollout{Name: "worker4"},
			))

			Expect(shoot.Status.ManualWorkerPoolRollout.PendingWorkersRollouts).To(ConsistOf(
				gardencorev1beta1.PendingWorkersRollout{Name: "worker3"},
			))

			Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("worker4"))
			Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("worker5"))
		})

		It("should remove all worker pools from pending workers rollouts if hibernation is enabled", func() {
			removeNonExistentPoolsFromPendingWorkersRollouts(shoot, true)

			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts).To(BeEmpty())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts).To(BeEmpty())
			Expect(shoot.Status.InPlaceUpdates).To(BeNil())
		})
	})

	Describe("#patchShootStatusOperationSuccess", func() {
		BeforeEach(func() {
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()

			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

			fakeDateAndTime, _ := time.Parse(time.DateTime, "2024-05-14 19:59:39")
			fakeClock = testclock.NewFakeClock(fakeDateAndTime)

			reconciler = &Reconciler{
				GardenClient:  gardenClient,
				SeedClientSet: seedClientSet,
				Clock:         fakeClock,
			}

			Expect(gardenClient.Create(ctx, namespace)).To(Succeed())
			Expect(gardenClient.Create(ctx, shoot)).To(Succeed())

			shootForCluster := shoot.DeepCopy()
			shootForCluster.TypeMeta = metav1.TypeMeta{APIVersion: gardencorev1beta1.SchemeGroupVersion.String(), Kind: "Shoot"}
			Expect(seedClient.Create(ctx, &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: shoot.Status.TechnicalID},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{Object: shootForCluster},
				},
			})).To(Succeed())

			DeferCleanup(func() {
				Expect(gardenClient.Delete(ctx, shoot)).To(Succeed())
				Expect(gardenClient.Delete(ctx, namespace)).To(Succeed())
			})
		})

		It("should clean the pending workers", func() {
			shoot.Status.ManualWorkerPoolRollout.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{{Name: "worker-1"}}
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.ManualWorkerPoolRollout.PendingWorkersRollouts).To(BeNil())
		})

		It("should not set the rotation status to Prepared if current status is Preparing and manual in-place update pending workers are present", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationPreparing
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationPreparing
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
		})

		It("should set the rotation status to Prepared if current status is Preparing and manual in-place update pending workers are empty", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationPreparing
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationPreparing
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
		})

		It("should not set the rotation status to Prepared if current status is WaitingForWorkersRollout and manual in-place update pending workers are present", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts = nil
			shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
		})

		It("should not set the rotation status to Prepared if current status is WaitingForWorkersRollout and certificateAuthorities.PendingWorkersRollouts are present", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
		})

		It("should set the rotation status to Prepared if current status is WaitingForWorkersRollout and manual in-place update pending workers and certificateAuthorities.PendingWorkersRollouts are empty", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts = nil
			shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts = nil
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
		})
	})

	Describe("#updateShootStatusOperationStart", func() {
		var (
			seed *gardencorev1beta1.Seed

			address = func(name, domain string) gardencorev1beta1.ShootAdvertisedAddress {
				return gardencorev1beta1.ShootAdvertisedAddress{Name: name, URL: "https://api." + domain}
			}
			externalAddress = address(v1beta1constants.AdvertisedAddressExternal, "shoot.example.com")

			startOperation = func(operationType gardencorev1beta1.LastOperationType) {
				GinkgoHelper()
				Expect(reconciler.updateShootStatusOperationStart(ctx, shoot, seed, "project", shoot.Status.TechnicalID, operationType)).To(Succeed())
				Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			}
		)

		BeforeEach(func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.MutableShootDomains, true))

			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()
			reconciler = &Reconciler{
				GardenClient: gardenClient,
				Clock:        testclock.NewFakeClock(time.Now()),
				Identity:     &gardencorev1beta1.Gardener{},
			}

			seed = &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{DNS: gardencorev1beta1.SeedDNS{
				InternalDomains: []gardencorev1beta1.SeedDNSProviderConfig{{Domain: "internal.new.example.com"}, {Domain: "internal.old.example.com"}},
			}}}

			shoot.Annotations = map[string]string{
				v1beta1constants.GardenerOperation:                           v1beta1constants.OperationRotateCAStart,
				v1beta1constants.AnnotationConfirmationMigrateInternalDomain: "true",
			}
			shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
				externalAddress,
				address(v1beta1constants.AdvertisedAddressInternal, "shoot.project.internal.old.example.com"),
			}
			Expect(gardenClient.Create(ctx, shoot)).To(Succeed())
		})

		It("should migrate the internal domain if a CA rotation starts with the confirmation", func() {
			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				externalAddress,
				address(v1beta1constants.AdvertisedAddressInternal, "shoot.project.internal.new.example.com"),
				address(v1beta1constants.AdvertisedAddressPriorInternal, "shoot.project.internal.old.example.com"),
			}))
			Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationConfirmationMigrateInternalDomain))
		})

		It("should migrate the internal domain if a CA rotation without workers rollout starts with the confirmation", func() {
			shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.OperationRotateCAStartWithoutWorkersRollout
			Expect(gardenClient.Update(ctx, shoot)).To(Succeed())

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(ContainElement(address(v1beta1constants.AdvertisedAddressPriorInternal, "shoot.project.internal.old.example.com")))
		})

		It("should migrate the internal domain if a credentials rotation starts with the confirmation", func() {
			shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.OperationRotateCredentialsStart
			Expect(gardenClient.Update(ctx, shoot)).To(Succeed())

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(ContainElement(address(v1beta1constants.AdvertisedAddressPriorInternal, "shoot.project.internal.old.example.com")))
		})

		It("should remove the internal domain if the seed disables it", func() {
			seed.Spec.DNS.InternalDomainEnabled = new(false)

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				externalAddress,
				address(v1beta1constants.AdvertisedAddressPriorInternal, "shoot.project.internal.old.example.com"),
			}))
		})

		It("should add the internal domain without a prior internal domain if the shoot has no internal domain", func() {
			shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{externalAddress}
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				externalAddress,
				address(v1beta1constants.AdvertisedAddressInternal, "shoot.project.internal.new.example.com"),
			}))
		})

		It("should keep the internal domain if the CA rotation starts without the confirmation", func() {
			delete(shoot.Annotations, v1beta1constants.AnnotationConfirmationMigrateInternalDomain)
			Expect(gardenClient.Update(ctx, shoot)).To(Succeed())
			addresses := shoot.Status.AdvertisedAddresses

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal(addresses))
		})

		It("should keep the internal domain and the confirmation if no CA rotation starts", func() {
			delete(shoot.Annotations, v1beta1constants.GardenerOperation)
			Expect(gardenClient.Update(ctx, shoot)).To(Succeed())
			addresses := shoot.Status.AdvertisedAddresses

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal(addresses))
			Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.AnnotationConfirmationMigrateInternalDomain, "true"))
		})

		It("should change nothing but remove the confirmation if the shoot already uses the internal domain of the seed", func() {
			shoot.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
				externalAddress,
				address(v1beta1constants.AdvertisedAddressInternal, "shoot.project.internal.new.example.com"),
			}
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())
			addresses := shoot.Status.AdvertisedAddresses

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal(addresses))
			Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationConfirmationMigrateInternalDomain))
		})

		It("should change nothing for a new shoot without advertised addresses", func() {
			shoot.Status.AdvertisedAddresses = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			startOperation(gardencorev1beta1.LastOperationTypeCreate)

			Expect(shoot.Status.AdvertisedAddresses).To(BeEmpty())
		})

		It("should keep the internal domain if the feature gate is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.MutableShootDomains, false))
			addresses := shoot.Status.AdvertisedAddresses

			startOperation(gardencorev1beta1.LastOperationTypeReconcile)

			Expect(shoot.Status.AdvertisedAddresses).To(Equal(addresses))
		})
	})

	Describe("#removeFinalizerFromShoot", func() {
		BeforeEach(func() {
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()

			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

			fakeDateAndTime, _ := time.Parse(time.DateTime, "2024-05-14 19:59:39")
			fakeClock = testclock.NewFakeClock(fakeDateAndTime)

			reconciler = &Reconciler{
				GardenClient:  gardenClient,
				SeedClientSet: seedClientSet,
				Clock:         fakeClock,
			}

			deletionTimestamp := metav1.NewTime(fakeClock.Now())
			shoot.DeletionTimestamp = &deletionTimestamp
			shoot.Finalizers = []string{gardencorev1beta1.GardenerName}
		})

		It("should emit an event and return the error if the finalizer removal fails", func() {
			fakeRecorder := &events.FakeRecorder{Events: make(chan string, 1)}

			gardenClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithStatusSubresource(&gardencorev1beta1.Shoot{}).
				WithInterceptorFuncs(interceptor.Funcs{
					// Let the status patch ('Delete Succeeded') through, but fail the subsequent finalizer removal
					// (a merge patch on the Shoot's metadata) to exercise the failure path.
					Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if _, ok := obj.(*gardencorev1beta1.Shoot); ok && patch.Type() == types.MergePatchType {
							return fmt.Errorf("fake patch error")
						}
						return c.Patch(ctx, obj, patch, opts...)
					},
				}).
				Build()

			reconciler.GardenClient = gardenClient
			reconciler.Recorder = fakeRecorder

			Expect(gardenClient.Create(ctx, shoot)).To(Succeed())

			err := reconciler.removeFinalizerFromShoot(ctx, logr.Discard(), shoot)
			Expect(err).To(MatchError(ContainSubstring("failed to remove finalizer")))

			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("failed to remove finalizer")))
		})
	})
})
