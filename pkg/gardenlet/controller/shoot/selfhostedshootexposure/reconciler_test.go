// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/selfhostedshootexposure"
)

var _ = Describe("Reconciler", func() {
	const shootName = "my-shoot"

	var (
		ctx = context.Background()

		gardenClient  client.Client
		shootClient   client.Client
		runtimeScheme *runtime.Scheme
		reconciler    *Reconciler

		shoot             *gardencorev1beta1.Shoot
		dnsRecord         *extensionsv1alpha1.DNSRecord
		internalDNSRecord *extensionsv1alpha1.DNSRecord

		// controlPlaneNode returns a healthy control-plane Node with the given addresses.
		controlPlaneNode = func(name string, addresses ...corev1.NodeAddress) *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
					Addresses:  addresses,
				},
			}
		}
		externalIP = func(address string) corev1.NodeAddress {
			return corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: address}
		}
		internalIP = func(address string) corev1.NodeAddress {
			return corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: address}
		}
		createDNSRecords = func() {
			GinkgoHelper()
			Expect(shootClient.Create(ctx, dnsRecord)).To(Succeed())
			Expect(shootClient.Create(ctx, internalDNSRecord)).To(Succeed())
		}
		getDNSRecord = func(dnsRecord *extensionsv1alpha1.DNSRecord) *extensionsv1alpha1.DNSRecord {
			GinkgoHelper()
			updated := &extensionsv1alpha1.DNSRecord{}
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
			return updated
		}
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: "garden-project"},
			Spec: gardencorev1beta1.ShootSpec{
				CredentialsBindingName: new("credentials"),
				DNS:                    &gardencorev1beta1.DNS{Domain: new("root.example.com")},
				Provider: gardencorev1beta1.Provider{
					Type: "local",
					Workers: []gardencorev1beta1.Worker{{
						Name: "control-plane",
						ControlPlane: &gardencorev1beta1.WorkerControlPlane{
							// DNS-based exposure (Extension is nil).
							Exposure: &gardencorev1beta1.Exposure{DNS: &gardencorev1beta1.DNSExposure{}},
						},
					}},
				},
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				},
			},
		}

		// The external and internal DNSRecords already exist (created during bootstrapping), the controller only keeps
		// them in sync.
		dnsRecord = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName + "-" + v1beta1constants.DNSRecordExternalName,
				Namespace: metav1.NamespaceSystem,
			},
		}
		internalDNSRecord = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName + "-" + v1beta1constants.DNSRecordInternalName,
				Namespace: metav1.NamespaceSystem,
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(shoot).Build()

		runtimeScheme = runtime.NewScheme()
		Expect(kubernetes.AddSeedSchemeToScheme(runtimeScheme)).To(Succeed())
		Expect(extensionsv1alpha1.AddToScheme(runtimeScheme)).To(Succeed())
		shootClient = fakeclient.NewClientBuilder().WithScheme(runtimeScheme).Build()

		reconciler = &Reconciler{
			GardenClient: gardenClient,
			ShootClient:  shootClient,
			ShootKey:     client.ObjectKeyFromObject(shoot),
			Clock:        testclock.NewFakeClock(time.Now()),
		}
	})

	It("should do nothing if the Shoot is gone", func() {
		Expect(gardenClient.Delete(ctx, shoot)).To(Succeed())

		Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))
	})

	It("should do nothing if the control plane exposure is not configured", func() {
		shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = nil
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(shoot).Build()
		reconciler.GardenClient = gardenClient

		Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))
	})

	Context("DNS-based exposure", func() {
		It("should patch the external and internal DNSRecords with the sorted node addresses", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.5"), internalIP("10.0.0.2")))).To(Succeed())
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-2", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			updated := getDNSRecord(dnsRecord)
			Expect(updated.Spec.RecordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			// Equal (not ConsistOf) on purpose: the values must be sorted so equal node sets don't cause unnecessary updates.
			Expect(updated.Spec.Values).To(Equal([]string{"1.2.3.4", "1.2.3.5"}))
			Expect(updated.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))

			updatedInternal := getDNSRecord(internalDNSRecord)
			Expect(updatedInternal.Spec.RecordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(updatedInternal.Spec.Values).To(Equal([]string{"10.0.0.1", "10.0.0.2"}))
			Expect(updatedInternal.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
		})

		It("should not patch the DNSRecords when they are already up-to-date", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			dnsRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeA
			dnsRecord.Spec.Values = []string{"1.2.3.4"}
			internalDNSRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeA
			internalDNSRecord.Spec.Values = []string{"10.0.0.1"}
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			// No reconcile annotation means the no-op guard skipped the patch entirely.
			Expect(getDNSRecord(dnsRecord).Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			Expect(getDNSRecord(internalDNSRecord).Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
		})

		It("should fall back to a node's internal address if it has no external one (e.g. local setups)", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			Expect(getDNSRecord(dnsRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))
			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))
		})

		It("should ignore the addresses of unhealthy nodes", func() {
			unhealthy := controlPlaneNode("cp-2", externalIP("1.2.3.5"), internalIP("10.0.0.2"))
			unhealthy.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			Expect(shootClient.Create(ctx, unhealthy)).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			Expect(getDNSRecord(dnsRecord).Spec.Values).To(Equal([]string{"1.2.3.4"}))
			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))
		})

		It("should fail if no healthy node has a usable address", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", corev1.NodeAddress{Type: corev1.NodeHostName, Address: "cp-1"}))).To(Succeed())
			createDNSRecords()

			_, err := reconciler.Reconcile(ctx, reconcile.Request{})
			Expect(err).To(MatchError(ContainSubstring("matches a configured IP family")))
		})

		It("should update the internal DNSRecord even if the external DNSRecord cannot be updated", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			Expect(shootClient.Create(ctx, internalDNSRecord)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{})
			Expect(err).To(MatchError(ContainSubstring("failed getting DNSRecord")))

			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))
		})

		It("should update the external DNSRecord even if the internal DNSRecord cannot be updated", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4")))).To(Succeed())
			createDNSRecords()

			_, err := reconciler.Reconcile(ctx, reconcile.Request{})
			Expect(err).To(MatchError(ContainSubstring("failed reconciling internal DNSRecord")))

			Expect(getDNSRecord(dnsRecord).Spec.Values).To(Equal([]string{"1.2.3.4"}))
		})

		It("should not touch the internal DNSRecord for shoots with unmanaged infrastructure", func() {
			shoot.Spec.CredentialsBindingName = nil
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(shoot).Build()
			reconciler.GardenClient = gardenClient

			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			Expect(getDNSRecord(dnsRecord).Spec.Values).To(Equal([]string{"1.2.3.4"}))
			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(BeEmpty())
		})

		It("should delete a SelfHostedShootExposure left over from a previous extension-based exposure", func() {
			leftover := &extensionsv1alpha1.SelfHostedShootExposure{
				ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: metav1.NamespaceSystem},
			}
			Expect(shootClient.Create(ctx, leftover)).To(Succeed())
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			Expect(getDNSRecord(dnsRecord).Spec.Values).To(Equal([]string{"1.2.3.4"}))

			err := shootClient.Get(ctx, client.ObjectKeyFromObject(leftover), leftover)
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "orphaned SelfHostedShootExposure must be deleted")
		})
	})

	Context("extension-based exposure", func() {
		const exposureType = "local"

		var (
			fakeClock             *testclock.FakeClock
			endpointUpdateEnabled *bool
		)

		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
			// defaulting to true
			endpointUpdateEnabled = nil

			shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{
				Extension: &gardencorev1beta1.ExtensionExposure{Type: new(exposureType)},
			}

			// The runtime client simulates the exposure extension controller: the moment a SelfHostedShootExposure is
			// created/patched with the reconcile operation annotation, it stamps a successful status with an ingress so
			// the component's Wait succeeds on its first poll.
			stampExposureReady := func(ctx context.Context, c client.WithWatch, obj client.Object) error {
				exposure, ok := obj.(*extensionsv1alpha1.SelfHostedShootExposure)
				if !ok {
					return nil
				}
				if err := c.Get(ctx, client.ObjectKeyFromObject(exposure), exposure); err != nil {
					return err
				}
				if _, hasOp := exposure.Annotations[v1beta1constants.GardenerOperation]; !hasOp {
					return nil
				}

				withoutOp := exposure.DeepCopy()
				delete(exposure.Annotations, v1beta1constants.GardenerOperation)
				if err := c.Patch(ctx, exposure, client.MergeFrom(withoutOp)); err != nil {
					return err
				}

				beforeStatus := exposure.DeepCopy()
				exposure.Status.ObservedGeneration = exposure.Generation
				exposure.Status.LastError = nil
				exposure.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:           gardencorev1beta1.LastOperationTypeReconcile,
					State:          gardencorev1beta1.LastOperationStateSucceeded,
					LastUpdateTime: metav1.NewTime(fakeClock.Now().UTC().Add(time.Second)),
				}
				exposure.Status.Ingress = []corev1.LoadBalancerIngress{{IP: "5.6.7.8"}}
				return c.Status().Patch(ctx, exposure, client.MergeFrom(beforeStatus))
			}

			shootClient = fakeclient.NewClientBuilder().
				WithScheme(runtimeScheme).
				WithStatusSubresource(&extensionsv1alpha1.SelfHostedShootExposure{}).
				WithInterceptorFuncs(interceptor.Funcs{
					Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						if err := c.Create(ctx, obj, opts...); err != nil {
							return err
						}
						return stampExposureReady(ctx, c, obj)
					},
					Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if err := c.Patch(ctx, obj, patch, opts...); err != nil {
							return err
						}
						return stampExposureReady(ctx, c, obj)
					},
				}).
				Build()
		})

		JustBeforeEach(func() {
			controllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "provider-local"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{{
						Kind:                     extensionsv1alpha1.SelfHostedShootExposureResource,
						Type:                     exposureType,
						ContinuousEndpointUpdate: endpointUpdateEnabled,
					}},
				},
			}
			// The controller derives the ControllerRegistration from the shoot's ControllerInstallations.
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "provider-local"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration.Name},
					ShootRef:        &corev1.ObjectReference{Name: shoot.Name, Namespace: shoot.Namespace},
				},
			}

			gardenClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithObjects(shoot, controllerRegistration, controllerInstallation).
				WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.ShootRefName, func(o client.Object) []string {
					if ref := o.(*gardencorev1beta1.ControllerInstallation).Spec.ShootRef; ref != nil {
						return []string{ref.Name}
					}
					return nil
				}).
				WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.ShootRefNamespace, func(o client.Object) []string {
					if ref := o.(*gardencorev1beta1.ControllerInstallation).Spec.ShootRef; ref != nil {
						return []string{ref.Namespace}
					}
					return nil
				}).
				Build()

			reconciler = &Reconciler{
				GardenClient: gardenClient,
				ShootClient:  shootClient,
				ShootKey:     client.ObjectKeyFromObject(shoot),
				Clock:        fakeClock,
			}
		})

		It("should deploy the SelfHostedShootExposure and patch the DNSRecords from its ingress and the nodes", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			// The exposure resource was created carrying the control-plane node endpoints (all addresses, the extension
			// decides which to use).
			exposure := &extensionsv1alpha1.SelfHostedShootExposure{}
			Expect(shootClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: shootName}, exposure)).To(Succeed())
			Expect(exposure.Spec.Type).To(Equal(exposureType))
			Expect(exposure.Spec.Endpoints).To(HaveLen(1))
			Expect(exposure.Spec.Endpoints[0].NodeName).To(Equal("cp-1"))

			// The DNSRecord was updated from the extension-reported ingress, not from the node addresses.
			updated := &extensionsv1alpha1.DNSRecord{}
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
			Expect(updated.Spec.RecordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(updated.Spec.Values).To(Equal([]string{"5.6.7.8"}))
			Expect(updated.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))

			// The internal DNSRecord points directly at the control-plane nodes, independent of the exposure.
			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))
		})

		It("should reuse the reported ingress without re-triggering the extension when endpoints are unchanged", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			// An already-reconciled exposure targeting the same endpoints, reporting a distinct ingress.
			exposure := &extensionsv1alpha1.SelfHostedShootExposure{
				ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: metav1.NamespaceSystem},
				Spec: extensionsv1alpha1.SelfHostedShootExposureSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{Type: exposureType},
					Endpoints: []extensionsv1alpha1.ControlPlaneEndpoint{
						{NodeName: "cp-1", Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}}},
					},
				},
			}
			Expect(shootClient.Create(ctx, exposure)).To(Succeed())
			exposure.Status.ObservedGeneration = exposure.Generation
			exposure.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile, State: gardencorev1beta1.LastOperationStateSucceeded}
			exposure.Status.Ingress = []corev1.LoadBalancerIngress{{IP: "9.9.9.9"}}
			Expect(shootClient.Status().Update(ctx, exposure)).To(Succeed())

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			// The DNSRecord was set from the pre-existing ingress (9.9.9.9); a re-trigger would have stamped 5.6.7.8.
			updated := &extensionsv1alpha1.DNSRecord{}
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
			Expect(updated.Spec.Values).To(Equal([]string{"9.9.9.9"}))
		})

		When("the extension opted out of endpoint updates via the ControllerRegistration", func() {
			BeforeEach(func() {
				endpointUpdateEnabled = new(false)
			})

			It("should not deploy the SelfHostedShootExposure nor touch the external DNSRecord", func() {
				Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", internalIP("10.0.0.1")))).To(Succeed())
				createDNSRecords()

				Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

				exposure := &extensionsv1alpha1.SelfHostedShootExposure{}
				err := shootClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: shootName}, exposure)
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "SelfHostedShootExposure must not be created")

				updated := &extensionsv1alpha1.DNSRecord{}
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
				Expect(updated.Spec.Values).To(BeEmpty())
				Expect(updated.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))

				// The internal DNSRecord is still kept in sync with the control-plane nodes.
				Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))
			})
		})
	})

	Context("extension-based exposure disabled", func() {
		BeforeEach(func() {
			// Exposure is omitted; a SelfHostedShootExposure from the prior extension-based setup may still exist.
			shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = nil
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(shoot).Build()
			reconciler.GardenClient = gardenClient
		})

		It("should flip the DNSRecords to the node addresses and delete the SelfHostedShootExposure", func() {
			exposure := &extensionsv1alpha1.SelfHostedShootExposure{
				ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: metav1.NamespaceSystem},
			}
			Expect(shootClient.Create(ctx, exposure)).To(Succeed())
			// The node is unhealthy, but the final update includes all control-plane nodes.
			node := controlPlaneNode("cp-1", internalIP("10.0.0.1"))
			node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
			Expect(shootClient.Create(ctx, node)).To(Succeed())
			// The external record still points at the old extension ingress, the internal one at an old node.
			dnsRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeA
			dnsRecord.Spec.Values = []string{"5.6.7.8"}
			internalDNSRecord.Spec.RecordType = extensionsv1alpha1.DNSRecordTypeA
			internalDNSRecord.Spec.Values = []string{"10.0.0.2"}
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			// The DNSRecord was flipped to the control-plane node addresses one last time.
			updated := &extensionsv1alpha1.DNSRecord{}
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
			Expect(updated.Spec.Values).To(Equal([]string{"10.0.0.1"}))
			Expect(updated.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(Equal([]string{"10.0.0.1"}))

			// The SelfHostedShootExposure was deleted, leaving the records to the operator.
			err := shootClient.Get(ctx, client.ObjectKeyFromObject(exposure), exposure)
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "SelfHostedShootExposure must be deleted")
		})

		It("should do nothing when no SelfHostedShootExposure exists (DNS-based or never managed)", func() {
			Expect(shootClient.Create(ctx, controlPlaneNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")))).To(Succeed())
			createDNSRecords()

			Expect(reconciler.Reconcile(ctx, reconcile.Request{})).To(Equal(reconcile.Result{}))

			updated := &extensionsv1alpha1.DNSRecord{}
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
			Expect(updated.Spec.Values).To(BeEmpty())
			Expect(updated.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			Expect(getDNSRecord(internalDNSRecord).Spec.Values).To(BeEmpty())
		})
	})
})
