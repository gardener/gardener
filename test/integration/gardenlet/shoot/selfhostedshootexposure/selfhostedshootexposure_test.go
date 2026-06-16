// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("SelfHostedShootExposure controller tests", func() {
	const exposureType = "local"

	var (
		shoot         *gardencorev1beta1.Shoot
		dnsRecord     *extensionsv1alpha1.DNSRecord
		dnsRecordName = shootName + "-" + v1beta1constants.DNSRecordExternalName
	)

	// controlPlaneNode returns a healthy control-plane Node with the given addresses.
	controlPlaneNode := func(name string, addresses ...corev1.NodeAddress) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": "", testID: testRunID},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Addresses:  addresses,
			},
		}
	}

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: v1beta1constants.GardenNamespace,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName: new("cloudprofile1"),
				Region:           "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: exposureType,
					Workers: []gardencorev1beta1.Worker{{
						Name:    "control-plane",
						Minimum: 1,
						Maximum: 1,
						Machine: gardencorev1beta1.Machine{
							Type:  "large",
							Image: &gardencorev1beta1.ShootMachineImage{Name: "some-image", Version: new("1.0.0")},
						},
						ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
					}},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.31.1"},
				Networking: &gardencorev1beta1.Networking{
					Type:       new("foo-networking"),
					Services:   new("10.0.0.0/16"),
					Pods:       new("10.1.0.0/16"),
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				},
			},
		}

		// The external DNSRecord already exists (created during bootstrapping); the controller only keeps it in sync.
		dnsRecord = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsRecordName,
				Namespace: metav1.NamespaceSystem,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{Type: exposureType},
				SecretRef:   corev1.SecretReference{Name: "dnsrecord-secret", Namespace: metav1.NamespaceSystem},
				Name:        "api.root.example.com",
				RecordType:  extensionsv1alpha1.DNSRecordTypeA,
				// A stale bootstrap value the controller is expected to overwrite.
				Values: []string{"9.9.9.9"},
			},
		}
	})

	// createNode/createShoot/createDNSRecord create the object and register cleanup.
	createNode := func(n *corev1.Node) {
		ExpectWithOffset(1, testClient.Create(ctx, n)).To(Succeed())
		DeferCleanup(func() {
			ExpectWithOffset(1, testClient.Delete(ctx, n)).To(Or(Succeed(), BeNotFoundError()))
		})
	}
	createShoot := func() {
		ExpectWithOffset(1, testClient.Create(ctx, shoot)).To(Succeed())
		DeferCleanup(func() {
			ExpectWithOffset(1, testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
		})
	}
	createDNSRecord := func() {
		ExpectWithOffset(1, testClient.Create(ctx, dnsRecord)).To(Succeed())
		DeferCleanup(func() {
			ExpectWithOffset(1, testClient.Delete(ctx, dnsRecord)).To(Or(Succeed(), BeNotFoundError()))
		})
	}

	dnsRecordValues := func() func() []string {
		return func() []string {
			updated := &extensionsv1alpha1.DNSRecord{}
			if err := testClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated); err != nil {
				return nil
			}
			return updated.Spec.Values
		}
	}

	Context("DNS-based exposure", func() {
		BeforeEach(func() {
			shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{DNS: &gardencorev1beta1.DNSExposure{}}
		})

		It("should keep the external DNSRecord in sync with the control-plane node external addresses", func() {
			createNode(controlPlaneNode("cp-1", corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"}))
			createDNSRecord()
			createShoot()

			Eventually(dnsRecordValues()).Should(ConsistOf("1.2.3.4"))

			updated := &extensionsv1alpha1.DNSRecord{}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsRecord), updated)).To(Succeed())
			Expect(updated.Spec.RecordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(updated.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
		})

		It("should delete a SelfHostedShootExposure left over from a previous extension-based exposure", func() {
			createNode(controlPlaneNode("cp-1", corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"}))
			createDNSRecord()
			leftover := createExposure()
			createShoot()

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(leftover), leftover)
			}).Should(BeNotFoundError(), "orphaned SelfHostedShootExposure should be deleted")
			Eventually(dnsRecordValues()).Should(ConsistOf("1.2.3.4"))
		})
	})

	Context("extension-based exposure", func() {
		BeforeEach(func() {
			shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{
				Extension: &gardencorev1beta1.ExtensionExposure{Type: new(exposureType)},
			}
		})

		It("should propagate ingress changes reported by the extension to the DNSRecord", func() {
			createNode(controlPlaneNode("cp-1", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}))
			createDNSRecord()
			exposure := createExposure()
			stampExposureIngress(exposure, "172.18.0.10")
			createShoot()

			Eventually(dnsRecordValues()).Should(ConsistOf("172.18.0.10"))

			stampExposureIngress(exposure, "172.18.0.11")
			Eventually(dnsRecordValues()).Should(ConsistOf("172.18.0.11"))
		})
	})

	Context("extension-based exposure with continuousEndpointUpdate disabled", func() {
		BeforeEach(func() {
			shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{
				Extension: &gardencorev1beta1.ExtensionExposure{Type: new(exposureType)},
			}
			createControllerRegistrationAndInstallation(new(false))
		})

		It("should neither deploy a SelfHostedShootExposure nor touch the DNSRecord", func() {
			createNode(controlPlaneNode("cp-1", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}))
			createDNSRecord()
			createShoot()

			exposure := &extensionsv1alpha1.SelfHostedShootExposure{}
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: shootName}, exposure)
			}).Should(BeNotFoundError(), "SelfHostedShootExposure must not be created")
			// The DNSRecord keeps its bootstrap value untouched.
			Expect(dnsRecordValues()()).To(ConsistOf("9.9.9.9"))
		})
	})

	Context("exposure disabled (switch-off handoff)", func() {
		It("should point the DNSRecord at the node addresses and delete the SelfHostedShootExposure", func() {
			// Exposure is omitted, but a SelfHostedShootExposure from the prior extension-based setup still exists.
			createNode(controlPlaneNode("cp-1", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}))
			createDNSRecord()
			exposure := createExposure()
			createShoot()

			Eventually(dnsRecordValues()).Should(ConsistOf("10.0.0.1"))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(exposure), exposure)
			}).Should(BeNotFoundError(), "SelfHostedShootExposure should be deleted after the final DNSRecord update")
		})
	})
})

// createExposure creates a SelfHostedShootExposure in the control-plane namespace of the test shoot (registering
// cleanup, since the controller is expected to delete it) and returns it.
func createExposure() *extensionsv1alpha1.SelfHostedShootExposure {
	exposure := &extensionsv1alpha1.SelfHostedShootExposure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shootName,
			Namespace: metav1.NamespaceSystem,
			Labels:    map[string]string{testID: testRunID},
		},
		Spec: extensionsv1alpha1.SelfHostedShootExposureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{Type: "local"},
			Port:        443,
			Endpoints: []extensionsv1alpha1.ControlPlaneEndpoint{
				{NodeName: "cp-1", Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}}},
			},
		},
	}
	ExpectWithOffset(1, testClient.Create(ctx, exposure)).To(Succeed())
	DeferCleanup(func() {
		ExpectWithOffset(1, testClient.Delete(ctx, exposure)).To(Or(Succeed(), BeNotFoundError()))
	})
	return exposure
}

// stampExposureIngress marks the SelfHostedShootExposure as successfully reconciled with the given ingress IP.
func stampExposureIngress(exposure *extensionsv1alpha1.SelfHostedShootExposure, ip string) {
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(exposure), exposure)).To(Succeed())
	patch := client.MergeFrom(exposure.DeepCopy())
	exposure.Status.ObservedGeneration = exposure.Generation
	exposure.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1.LastOperationTypeReconcile,
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		LastUpdateTime: metav1.Now(),
	}
	exposure.Status.Ingress = []corev1.LoadBalancerIngress{{IP: ip}}
	ExpectWithOffset(1, testClient.Status().Patch(ctx, exposure, patch)).To(Succeed())
}

// createControllerRegistrationAndInstallation registers a provider-local ControllerRegistration declaring the
// SelfHostedShootExposure resource (with the given continuousEndpointUpdate value) and a ControllerInstallation
// referencing the test shoot, and registers cleanup.
func createControllerRegistrationAndInstallation(continuousEndpointUpdate *bool) {
	registration := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "provider-local-" + testRunID, Labels: map[string]string{testID: testRunID}},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{{
				Kind:                     extensionsv1alpha1.SelfHostedShootExposureResource,
				Type:                     "local",
				ContinuousEndpointUpdate: continuousEndpointUpdate,
			}},
		},
	}
	ExpectWithOffset(1, testClient.Create(ctx, registration)).To(Succeed())
	DeferCleanup(func() {
		ExpectWithOffset(1, testClient.Delete(ctx, registration)).To(Or(Succeed(), BeNotFoundError()))
	})

	installation := &gardencorev1beta1.ControllerInstallation{
		ObjectMeta: metav1.ObjectMeta{Name: "provider-local-" + testRunID, Labels: map[string]string{testID: testRunID}},
		Spec: gardencorev1beta1.ControllerInstallationSpec{
			RegistrationRef: corev1.ObjectReference{Name: registration.Name},
			SeedRef:         &corev1.ObjectReference{Name: "seed"},
			ShootRef:        &corev1.ObjectReference{Name: shootName, Namespace: v1beta1constants.GardenNamespace},
		},
	}
	ExpectWithOffset(1, testClient.Create(ctx, installation)).To(Succeed())
	DeferCleanup(func() {
		ExpectWithOffset(1, testClient.Delete(ctx, installation)).To(Or(Succeed(), BeNotFoundError()))
	})
}
