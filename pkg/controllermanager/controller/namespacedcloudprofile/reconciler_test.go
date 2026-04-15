// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	namespacedcloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NamespacedCloudProfile Reconciler", func() {
	const finalizerName = "gardener"

	var (
		ctx        context.Context
		fakeClient client.Client
		reconciler reconcile.Reconciler

		fakeErr error

		namespaceName              string
		cloudProfileName           string
		namespacedCloudProfileName string

		cloudProfile           *gardencorev1beta1.CloudProfile
		namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

		newExpiryDate metav1.Time
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeErr = errors.New("fake err")

		namespaceName = "test-namespace"
		cloudProfileName = "test-cloudprofile"
		namespacedCloudProfileName = "test-namespacedcloudprofile"

		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: cloudProfileName,
			},
		}
		namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namespacedCloudProfileName,
				Namespace: namespaceName,
			},
			Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
				Parent: gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				},
			},
		}

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithStatusSubresource(&gardencorev1beta1.NamespacedCloudProfile{}).
			WithIndex(
				&gardencorev1beta1.NamespacedCloudProfile{},
				core.NamespacedCloudProfileParentRefName,
				indexer.NamespacedCloudProfileParentRefNameIndexerFunc,
			).
			Build()
		reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

		newExpiryDate = metav1.NewTime(time.Now().Truncate(time.Second))
	})

	It("should return nil because object not found", func() {
		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return err because object reading failed", func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fakeErr
				},
			}).
			Build()
		reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).To(MatchError(fakeErr))
	})

	Context("merge status", func() {
		It("should apply the CloudProfile providerConfig to the NamespacedCloudProfile status on spec change", func() {
			cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"key":"value"}`)}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			// Verify status was updated
			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.ProviderConfig).ToNot(BeNil())
			Expect(updated.Status.CloudProfileSpec.ProviderConfig.Raw).To(Equal([]byte(`{"key":"value"}`)))
		})

		It("should ignore an existing NamespacedCloudProfile status providerConfig on spec change", func() {
			cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"key":"value"}`)}
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"key2":"value2"}`)}
			namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"key":"value","key2":"value2"}`)}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			// Verify status was updated - the parent providerConfig should be set in the status
			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.ProviderConfig).ToNot(BeNil())
			Expect(updated.Status.CloudProfileSpec.ProviderConfig.Raw).To(Equal([]byte(`{"key":"value"}`)))
		})

		It("should sync the architecture capabilities", func() {
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "test-image-namespaced",
					Versions: []gardencorev1beta1.MachineImageVersion{{
						ExpirableVersion:         gardencorev1beta1.ExpirableVersion{Version: "1.1.2"},
						CRI:                      []gardencorev1beta1.CRI{{Name: "containerd"}},
						Architectures:            []string{"arm64"},
						KubeletVersionConstraint: ptr.To("==1.30.0"),
					}},
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
				},
			}
			cloudProfile.Spec.MachineCapabilities = []gardencorev1beta1.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.MachineCapabilities).To(Equal([]gardencorev1beta1.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
			}))
			Expect(updated.Status.CloudProfileSpec.MachineImages).To(HaveLen(1))
			Expect(updated.Status.CloudProfileSpec.MachineImages[0].Versions[0].Architectures).To(ConsistOf("arm64"))
			Expect(updated.Status.CloudProfileSpec.MachineImages[0].Versions[0].CapabilityFlavors).To(HaveLen(1))
			Expect(updated.Status.CloudProfileSpec.MachineImages[0].Versions[0].CapabilityFlavors[0].Capabilities).To(HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"arm64"}))
		})
	})

	Context("merge Kubernetes versions", func() {
		BeforeEach(func() {
			cloudProfile.Spec.Kubernetes = gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.0.0"},
				},
			}
			namespacedCloudProfile.Spec.Kubernetes = &gardencorev1beta1.KubernetesSettings{}
		})

		It("should ignore versions specified only in NamespacedCloudProfile", func() {
			namespacedCloudProfile.Spec.Kubernetes = &gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.2.3"},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.Kubernetes.Versions).To(Equal([]gardencorev1beta1.ExpirableVersion{{Version: "1.0.0"}}))
		})

		It("should merge Kubernetes versions correctly", func() {
			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0", ExpirationDate: &newExpiryDate},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.Kubernetes.Versions).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Version":        Equal("1.0.0"),
					"ExpirationDate": Equal(&newExpiryDate),
				}),
			))
		})

		It("should set observedGeneration correctly", func() {
			namespacedCloudProfile.Generation = 7

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			// The fake client doesn't track generation, so we check observedGeneration equals what the reconciler sets.
			// The reconciler reads the object's generation and sets observedGeneration to it.
			Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
		})

		Context("when deletion timestamp set", func() {
			BeforeEach(func() {
				namespacedCloudProfile.Finalizers = []string{finalizerName}

				Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())
				Expect(fakeClient.Delete(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, namespacedCloudProfile)).To(Succeed())
			})

			It("should do nothing because finalizer is not present", func() {
				updated := namespacedCloudProfile.DeepCopy()
				updated.Finalizers = []string{"some-other-finalizer"}
				Expect(fakeClient.Patch(ctx, updated, client.MergeFrom(namespacedCloudProfile))).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, namespacedCloudProfile)).To(Succeed())
				Expect(namespacedCloudProfile.Finalizers).To(Equal([]string{"some-other-finalizer"}))
			})

			It("should return an error because Shoot referencing NamespacedCloudProfile exists", func() {
				shoot := &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: namespaceName},
					Spec: gardencorev1beta1.ShootSpec{
						CloudProfile: &gardencorev1beta1.CloudProfileReference{
							Kind: "NamespacedCloudProfile",
							Name: namespacedCloudProfileName,
						},
					},
				}
				Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("Cannot delete NamespacedCloudProfile")))
			})

			It("should remove the finalizer (error)", func() {
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetes.GardenScheme).
					WithStatusSubresource(&gardencorev1beta1.NamespacedCloudProfile{}).
					WithInterceptorFuncs(interceptor.Funcs{
						Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
							return fakeErr
						},
					}).
					Build()
				reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

				ncp := namespacedCloudProfile.DeepCopy()
				ncp.Finalizers = []string{finalizerName}
				ncp.ResourceVersion = ""
				Expect(fakeClient.Create(ctx, ncp)).To(Succeed())
				Expect(fakeClient.Delete(ctx, ncp.DeepCopy())).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should remove the finalizer (no error)", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, &gardencorev1beta1.NamespacedCloudProfile{})).To(BeNotFoundError())
			})
		})

		Context("when deletion timestamp not set", func() {
			It("should ensure the finalizer (error)", func() {
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetes.GardenScheme).
					WithStatusSubresource(&gardencorev1beta1.NamespacedCloudProfile{}).
					WithInterceptorFuncs(interceptor.Funcs{
						Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
							return fakeErr
						},
					}).
					Build()
				reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

				Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(HaveOccurred())
			})

			It("should ensure the finalizer (no error)", func() {
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}

				Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
				Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				updated := &gardencorev1beta1.NamespacedCloudProfile{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
				Expect(updated.Finalizers).To(ContainElement(finalizerName))
			})
		})
	})

	Context("merge machine images", func() {
		BeforeEach(func() {
			cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "test-image",
					Versions: []gardencorev1beta1.MachineImageVersion{{
						ExpirableVersion:         gardencorev1beta1.ExpirableVersion{Version: "1.0.0"},
						CRI:                      []gardencorev1beta1.CRI{{Name: "containerd"}},
						Architectures:            []string{"amd64"},
						KubeletVersionConstraint: ptr.To("==1.30.0"),
					}},
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
				},
			}
		})

		It("should add machine images specified only in NamespacedCloudProfile", func() {
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "test-image-namespaced",
					Versions: []gardencorev1beta1.MachineImageVersion{{
						ExpirableVersion:         gardencorev1beta1.ExpirableVersion{Version: "1.1.2"},
						CRI:                      []gardencorev1beta1.CRI{{Name: "containerd"}},
						Architectures:            []string{"arm64"},
						KubeletVersionConstraint: ptr.To("==1.30.0"),
					}},
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.MachineImages).To(HaveLen(2))
			Expect(updated.Status.CloudProfileSpec.MachineImages).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal("test-image"),
					"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion":         Equal(gardencorev1beta1.ExpirableVersion{Version: "1.0.0", ExpirationDate: nil, Classification: nil, Lifecycle: nil}),
						"CRI":                      Equal([]gardencorev1beta1.CRI{{Name: "containerd", ContainerRuntimes: nil}}),
						"Architectures":            ConsistOf("amd64"),
						"KubeletVersionConstraint": Equal(ptr.To("==1.30.0")),
					})),
				}),
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal("test-image-namespaced"),
					"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion":         Equal(gardencorev1beta1.ExpirableVersion{Version: "1.1.2", ExpirationDate: nil, Classification: nil, Lifecycle: nil}),
						"CRI":                      Equal([]gardencorev1beta1.CRI{{Name: "containerd", ContainerRuntimes: nil}}),
						"Architectures":            ConsistOf("arm64"),
						"KubeletVersionConstraint": Equal(ptr.To("==1.30.0")),
					})),
				}),
			))
		})

		It("should merge MachineImages correctly", func() {
			newExpiryDate := metav1.NewTime(time.Now().Truncate(time.Second))
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "test-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						// override existing version with new expiration date
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0", ExpirationDate: &newExpiryDate}},
						// add new version
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.2"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.MachineImages).To(HaveLen(1))
			Expect(updated.Status.CloudProfileSpec.MachineImages[0].Name).To(Equal("test-image"))
			versions := updated.Status.CloudProfileSpec.MachineImages[0].Versions
			Expect(versions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"ExpirableVersion": Equal(gardencorev1beta1.ExpirableVersion{Version: "1.0.0", ExpirationDate: &newExpiryDate, Classification: nil, Lifecycle: nil}),
			}), MatchFields(IgnoreExtras, Fields{
				"ExpirableVersion": Equal(gardencorev1beta1.ExpirableVersion{Version: "1.1.2", ExpirationDate: nil, Classification: nil, Lifecycle: nil}),
			})))
		})

		It("should merge MachineImages with overridden updateStrategy correctly", func() {
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name:           "test-image",
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMinor),
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.MachineImages).To(HaveLen(1))
			Expect(updated.Status.CloudProfileSpec.MachineImages[0].UpdateStrategy).To(Equal(ptr.To(gardencorev1beta1.UpdateStrategyMinor)))
			Expect(updated.Status.CloudProfileSpec.MachineImages[0].Versions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"ExpirableVersion":         Equal(gardencorev1beta1.ExpirableVersion{Version: "1.0.0", ExpirationDate: nil, Classification: nil, Lifecycle: nil}),
				"CRI":                      Equal([]gardencorev1beta1.CRI{{Name: "containerd", ContainerRuntimes: nil}}),
				"Architectures":            ConsistOf("amd64"),
				"KubeletVersionConstraint": Equal(ptr.To("==1.30.0")),
			})))
		})
	})

	Context("merge machine types", func() {
		BeforeEach(func() {
			cloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
				{
					CPU:          resource.MustParse("2"),
					GPU:          resource.MustParse("7"),
					Memory:       resource.MustParse("10Gi"),
					Name:         "test-type",
					Storage:      nil,
					Usable:       nil,
					Architecture: nil,
				},
			}
		})

		It("should successfully add types specified only in NamespacedCloudProfile", func() {
			cloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{}
			namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
				{
					CPU:    resource.MustParse("1"),
					GPU:    resource.MustParse("5"),
					Memory: resource.MustParse("3Gi"),
					Name:   "test-type-namespaced",
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.MachineTypes).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Name":         Equal("test-type-namespaced"),
				"CPU":          Equal(resource.MustParse("1")),
				"GPU":          Equal(resource.MustParse("5")),
				"Memory":       Equal(resource.MustParse("3Gi")),
				"Architecture": Equal(ptr.To("amd64")),
			})))

		})

		It("should successfully add types specified in CloudProfile and NamespacedCloudProfile", func() {
			namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
				{
					CPU:    resource.MustParse("1"),
					GPU:    resource.MustParse("5"),
					Memory: resource.MustParse("3Gi"),
					Name:   "test-type-namespaced",
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile.DeepCopy())).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())

			updated := &gardencorev1beta1.NamespacedCloudProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, updated)).To(Succeed())
			Expect(updated.Status.CloudProfileSpec.MachineTypes).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Name":         Equal("test-type"),
					"CPU":          Equal(resource.MustParse("2")),
					"GPU":          Equal(resource.MustParse("7")),
					"Memory":       Equal(resource.MustParse("10Gi")),
					"Architecture": Equal(ptr.To("amd64")),
				}),
				MatchFields(IgnoreExtras, Fields{
					"Name":         Equal("test-type-namespaced"),
					"CPU":          Equal(resource.MustParse("1")),
					"GPU":          Equal(resource.MustParse("5")),
					"Memory":       Equal(resource.MustParse("3Gi")),
					"Architecture": Equal(ptr.To("amd64")),
				}),
			))
		})
	})

	Describe("#Reconcile", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			reconciler *namespacedcloudprofilecontroller.Reconciler

			namespaceName string

			cloudProfile           *gardencorev1beta1.CloudProfile
			namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
				WithStatusSubresource(&gardencorev1beta1.NamespacedCloudProfile{}).
				WithIndex(
					&gardencorev1beta1.NamespacedCloudProfile{},
					core.NamespacedCloudProfileParentRefName,
					indexer.NamespacedCloudProfileParentRefNameIndexerFunc,
				).Build()
			reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: fakeClient, Recorder: &events.FakeRecorder{}}

			namespaceName = "garden-test"

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{},
			}

			namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "n-profile-1",
					Namespace:  namespaceName,
					Generation: 1,
				},
				Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
					Parent: gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: cloudProfile.Name,
					},
				},
			}
		})

		Describe("machineImages and providerConfig with versioned rawExtension object", func() {
			BeforeEach(func() {
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "machine-image-1", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}},
					}},
				}
				cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Object: &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-1", Versions: []v1alpha1.MachineImageVersion{{Version: "1.0.0", Image: "local-dev:1.0.0"}}},
				}}}
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "machine-image-2", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "2.0.0"}},
					}},
				}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Object: &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-2", Versions: []v1alpha1.MachineImageVersion{{Version: "2.0.0", Image: "local-dev:2.0.0"}}},
				}}}
				Expect(fakeClient.Create(ctx, namespacedCloudProfile)).To(Succeed())
			})

			It("should successfully reconcile the NamespacedCloudProfile", func() {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfile.Name, Namespace: namespaceName}})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(ConsistOf(
					gardencorev1beta1.MachineImage{Name: "machine-image-1", Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"},
							Architectures:    []string{"amd64"},
						},
					}},
					gardencorev1beta1.MachineImage{Name: "machine-image-2", Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "2.0.0"},
							Architectures:    []string{"amd64"},
						},
					}},
				))
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw).To(Equal(
					[]byte(`{"machineImages":[{"name":"machine-image-1","versions":[{"image":"local-dev:1.0.0","version":"1.0.0"}]}]}`)))

				// Simulate a status update by the provider extension.
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Object: &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-1", Versions: []v1alpha1.MachineImageVersion{{Version: "1.0.0", Image: "local-dev:1.0.0"}}},
					{Name: "machine-image-2", Versions: []v1alpha1.MachineImageVersion{{Version: "2.0.0", Image: "local-dev:2.0.0"}}},
				}}}
				Expect(fakeClient.Status().Update(ctx, namespacedCloudProfile)).To(Succeed())
				statusConfig := &v1alpha1.CloudProfileConfig{}
				_, _, err = serializer.NewCodecFactory(fakeClient.Scheme()).UniversalDeserializer().Decode(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw, nil, statusConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(statusConfig).To(BeEquivalentTo(&v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-1", Versions: []v1alpha1.MachineImageVersion{{Version: "1.0.0", Image: "local-dev:1.0.0"}}},
					{Name: "machine-image-2", Versions: []v1alpha1.MachineImageVersion{{Version: "2.0.0", Image: "local-dev:2.0.0"}}},
				}}))
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw).To(SatisfyAll(
					ContainSubstring(`{"machineImages":[{"`),
					ContainSubstring(`{"name":"machine-image-1","versions":[`),
					ContainSubstring(`{"name":"machine-image-2","versions":[`),
					ContainSubstring(`"}]}]}`),
				))

				// Update NamespacedCloudProfile spec.
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "machine-image-2", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "3.0.0"}},
					}},
				}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Object: &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-2", Versions: []v1alpha1.MachineImageVersion{{Version: "3.0.0", Image: "local-dev:3.0.0"}}},
				}}}
				Expect(fakeClient.Update(ctx, namespacedCloudProfile)).To(Succeed())

				_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfile.Name, Namespace: namespaceName}})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(ConsistOf(
					gardencorev1beta1.MachineImage{Name: "machine-image-1", Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"},
							Architectures:    []string{"amd64"},
						},
					}},
					gardencorev1beta1.MachineImage{Name: "machine-image-2", Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "3.0.0"},
							Architectures:    []string{"amd64"},
						},
					}},
				))
				// Make sure that status providerConfig is set to cloudProfile providerConfig again.
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw).To(Equal(
					[]byte(`{"machineImages":[{"name":"machine-image-1","versions":[{"image":"local-dev:1.0.0","version":"1.0.0"}]}]}`)))
			})
		})

		Describe("custom CloudProfile spec overrides, that are added to the parent CloudProfile afterwards", func() {
			It("should successfully reconcile with custom machine images being added to the parent CloudProfile", func() {
				// Create CloudProfile.
				Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

				// Create NamespacedCloudProfile.
				namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{{
					Name:           "machine-image-1",
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
					Versions:       []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, Architectures: []string{"amd64", "arm64"}}}},
				}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Object: &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-1", Versions: []v1alpha1.MachineImageVersion{{Version: "1.0.0", Image: "local-dev:1.0.0-nscpfl"}}},
				}}}
				Expect(fakeClient.Create(ctx, namespacedCloudProfile)).To(Succeed())

				// Reconcile NamespacedCloudProfile Status.
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfile.Name, Namespace: namespaceName}})
				Expect(err).ToNot(HaveOccurred())

				// Add custom machine image from NamespacedCloudProfile to parent CloudProfile.
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{{
					Name:           "machine-image-1",
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMinor),
					Versions:       []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}}}},
				}
				cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Object: &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{
					{Name: "machine-image-1", Versions: []v1alpha1.MachineImageVersion{{Version: "1.0.0", Image: "local-dev:1.0.0-cpfl"}}},
				}}}
				Expect(fakeClient.Update(ctx, cloudProfile)).To(Succeed())

				// Reconcile NamespacedCloudProfile Status again.
				_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfile.Name, Namespace: namespaceName}})
				Expect(err).ToNot(HaveOccurred())

				// Expect that the NamespacedCloudProfile Status is as expected.
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(Equal(namespacedCloudProfile.Spec.MachineImages))
			})
		})

		Describe("#MergeCloudProfiles", func() {
			Describe("retain order of parent CloudProfile when merging", func() {
				var (
					cloudProfile           *gardencorev1beta1.CloudProfile
					namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
				)

				BeforeEach(func() {
					cloudProfile = &gardencorev1beta1.CloudProfile{
						Spec: gardencorev1beta1.CloudProfileSpec{
							MachineCapabilities: []gardencorev1beta1.CapabilityDefinition{
								{Name: "architecture", Values: []string{"amd64", "arm64"}},
								{Name: "cap-a", Values: []string{"featured", "standard"}},
								{Name: "cap-b", Values: []string{"one", "two", "three"}},
							},
							MachineImages: []gardencorev1beta1.MachineImage{
								{Name: "image-a"},
								{
									Name: "image-b",
									Versions: []gardencorev1beta1.MachineImageVersion{
										{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0"}, Architectures: []string{"amd64"}, CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{{Capabilities: map[string]gardencorev1beta1.CapabilityValues{
											"architecture": {"amd64"},
											"cap-b":        {"three", "two", "one"},
											"cap-a":        {"standard"},
										}}}},
										{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "2.0"}, Architectures: []string{"amd64"}, CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{{Capabilities: map[string]gardencorev1beta1.CapabilityValues{
											"architecture": {"amd64"},
										}}}},
										{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "3.0"}, Architectures: []string{"arm64"}, CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{{Capabilities: map[string]gardencorev1beta1.CapabilityValues{
											"architecture": {"arm64"},
											"cap-a":        {"featured"},
										}}}},
									},
								},
								{Name: "image-c"},
							},
							MachineTypes: []gardencorev1beta1.MachineType{
								{Name: "type-a", Architecture: ptr.To("amd64")},
								{Name: "type-b", Architecture: ptr.To("amd64")},
								{Name: "type-c", Architecture: ptr.To("amd64")},
							},
							VolumeTypes: []gardencorev1beta1.VolumeType{
								{Name: "volume-a"},
								{Name: "volume-b"},
								{Name: "volume-c"},
							},
							Kubernetes: gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
								{Version: "1.35.0"},
								{Version: "1.34.1"},
								{Version: "1.34.0"},
							}},
						},
					}
					namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{}
				})

				It("should match the CloudProfile for no overrides", func() {
					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					Expect(namespacedCloudProfile.Status.CloudProfileSpec).To(Equal(cloudProfile.Spec))
				})

				It("should add new elements and apply overrides consistently while keeping the existing elements ordered", func() {
					expirationDate := metav1.NewTime(time.Now().Add(time.Hour))

					namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{
							Name: "image-b",
							Versions: []gardencorev1beta1.MachineImageVersion{
								{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "2.0", ExpirationDate: &expirationDate}},
								{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.0"}, Architectures: []string{"amd64"}, CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{{Capabilities: map[string]gardencorev1beta1.CapabilityValues{
									"architecture": {"amd64"},
								}}}},
							},
						},
					}
					namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
						{Name: "type-d", Architecture: ptr.To("amd64")},
					}
					namespacedCloudProfile.Spec.VolumeTypes = []gardencorev1beta1.VolumeType{
						{Name: "volume-d"},
					}
					namespacedCloudProfile.Spec.Kubernetes = ptr.To(gardencorev1beta1.KubernetesSettings{
						Versions: []gardencorev1beta1.ExpirableVersion{
							{Version: "1.34.1", ExpirationDate: &expirationDate},
						},
					})

					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					expectedSpec := cloudProfile.Spec.DeepCopy()

					expectedSpec.MachineImages[1].Versions[1].ExpirationDate = &expirationDate
					expectedSpec.MachineImages[1].Versions = append(expectedSpec.MachineImages[1].Versions, gardencorev1beta1.MachineImageVersion{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.0"},
						Architectures:    []string{"amd64"},
						CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{{Capabilities: map[string]gardencorev1beta1.CapabilityValues{
							"architecture": {"amd64"},
						}}},
					})
					expectedSpec.MachineTypes = append(expectedSpec.MachineTypes, gardencorev1beta1.MachineType{Name: "type-d", Architecture: ptr.To("amd64"), Capabilities: map[string]gardencorev1beta1.CapabilityValues{
						"architecture": {"amd64"},
					}})
					expectedSpec.VolumeTypes = append(expectedSpec.VolumeTypes, gardencorev1beta1.VolumeType{Name: "volume-d"})
					expectedSpec.Kubernetes.Versions[1].ExpirationDate = &expirationDate

					Expect(namespacedCloudProfile.Status.CloudProfileSpec).To(Equal(*expectedSpec))
				})
			})

			Describe("merge limits.maxNodesTotal correctly", func() {
				var (
					cloudProfile           *gardencorev1beta1.CloudProfile
					namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
				)

				BeforeEach(func() {
					cloudProfile = &gardencorev1beta1.CloudProfile{}
					namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{}
				})

				It("should stay nil if no value is provided at all", func() {
					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits).To(BeNil())
				})

				It("should apply only the value from the CloudProfile", func() {
					cloudProfile.Spec.Limits = &gardencorev1beta1.Limits{MaxNodesTotal: ptr.To(int32(10))}

					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(10))))
				})

				It("should apply only the value from the NamespacedCloudProfile", func() {
					namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{MaxNodesTotal: ptr.To(int32(10))}

					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(10))))
				})

				It("should apply the higher overridden value from the NamespacedCloudProfile", func() {
					cloudProfile.Spec.Limits = &gardencorev1beta1.Limits{MaxNodesTotal: ptr.To(int32(10))}
					namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{MaxNodesTotal: ptr.To(int32(99))}

					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(99))))
				})

				It("should apply a lower value from the NamespacedCloudProfile", func() {
					cloudProfile.Spec.Limits = &gardencorev1beta1.Limits{MaxNodesTotal: ptr.To(int32(124))}
					namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{MaxNodesTotal: ptr.To(int32(20))}

					namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

					Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(20))))
				})
			})

			Describe("Transform to parent CloudProfile capability/legacy format functionality", func() {
				var (
					cloudProfile           *gardencorev1beta1.CloudProfile
					namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
				)

				BeforeEach(func() {
					cloudProfile = &gardencorev1beta1.CloudProfile{}
					namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{}
				})

				When("parent CloudProfile has capability definitions", func() {
					BeforeEach(func() {
						cloudProfile.Spec.MachineCapabilities = []gardencorev1beta1.CapabilityDefinition{
							{Name: "architecture", Values: []string{"amd64", "arm64"}},
						}
					})

					It("should transform legacy architectures to capability flavors when parent is in capability format", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: "ubuntu",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
										Architectures:    []string{"amd64", "arm64"},
									},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(HaveLen(1))
						Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions).To(HaveLen(1))
						version := namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions[0]
						Expect(version.CapabilityFlavors).To(HaveLen(2))
						Expect(version.CapabilityFlavors[0].Capabilities).To(HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"amd64"}))
						Expect(version.CapabilityFlavors[1].Capabilities).To(HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"arm64"}))
						Expect(version.Architectures).To(ConsistOf("amd64", "arm64"))
					})

					It("should add architecture to capabilityFlavors", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: "ubuntu",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
										Architectures:    []string{"amd64"},
									},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						version := namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions[0]
						Expect(version.CapabilityFlavors).To(HaveLen(1))
						Expect(version.CapabilityFlavors[0].Capabilities).To(HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"amd64"}))
						Expect(version.Architectures).To(ConsistOf("amd64"))
					})

					It("should preserve existing capability flavors", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: "ubuntu",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
										CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
											{
												Capabilities: gardencorev1beta1.Capabilities{
													"architecture": []string{"arm64"},
													"gpu":          []string{"nvidia"},
												},
											},
										},
										Architectures: []string{"arm64"},
									},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						version := namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions[0]
						Expect(version.CapabilityFlavors).To(HaveLen(1))
						Expect(version.CapabilityFlavors[0].Capabilities).To(HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"arm64"}))
						Expect(version.CapabilityFlavors[0].Capabilities).To(HaveKeyWithValue("gpu", gardencorev1beta1.CapabilityValues{"nvidia"}))
					})

					It("should set architecture capabilities for machine types", func() {
						namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
							{
								Name:   "m5.large",
								CPU:    resource.MustParse("2"),
								Memory: resource.MustParse("8Gi"),
							},
							{
								Name:         "m5.arm.large",
								CPU:          resource.MustParse("2"),
								Memory:       resource.MustParse("8Gi"),
								Architecture: ptr.To("arm64"),
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						machineTypes := namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes
						Expect(machineTypes).To(HaveLen(2))

						Expect(machineTypes).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Architecture": Equal(ptr.To("amd64")),
								"Capabilities": HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"amd64"}),
							}),
							MatchFields(IgnoreExtras, Fields{
								"Architecture": Equal(ptr.To("arm64")),
								"Capabilities": HaveKeyWithValue("architecture", gardencorev1beta1.CapabilityValues{"arm64"}),
							}),
						))
					})
				})

				When("parent CloudProfile has no capability definitions", func() {
					BeforeEach(func() {
						cloudProfile.Spec.MachineCapabilities = nil
					})

					It("should preserve existing architectures", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: "ubuntu",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
										Architectures:    []string{"amd64", "arm64"},
									},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						version := namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions[0]
						Expect(version.Architectures).To(ConsistOf("amd64", "arm64"))
						Expect(version.CapabilityFlavors).To(BeNil())
					})

					It("should clear capabilities for machine types", func() {
						namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
							{
								Name:   "m5.large",
								CPU:    resource.MustParse("2"),
								Memory: resource.MustParse("8Gi"),
								Capabilities: gardencorev1beta1.Capabilities{
									"architecture": []string{"amd64"},
									"gpu":          []string{"nvidia"},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						machineType := namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes[0]
						Expect(machineType.Architecture).To(Equal(ptr.To("amd64")))
						Expect(machineType.Capabilities).To(BeNil())
					})

					It("should handle multiple capability flavors with different architectures", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: "ubuntu",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
										CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
											{
												Capabilities: gardencorev1beta1.Capabilities{
													"architecture": []string{"amd64"},
												},
											},
											{
												Capabilities: gardencorev1beta1.Capabilities{
													"architecture": []string{"arm64"},
												},
											},
										},
									},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						version := namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions[0]
						Expect(version.Architectures).To(ConsistOf("amd64", "arm64"))
						Expect(version.CapabilityFlavors).To(BeNil())
					})
				})

				Context("edge cases", func() {
					It("should handle empty machine images list", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(BeEmpty())
					})

					It("should handle empty machine types list", func() {
						namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes).To(BeEmpty())
					})

					It("should handle machine image versions with no architectures and no capability flavors", func() {
						cloudProfile.Spec.MachineCapabilities = []gardencorev1beta1.CapabilityDefinition{
							{Name: "architecture", Values: []string{"amd64"}},
						}
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: "ubuntu",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
									},
								},
							},
						}

						namespacedcloudprofilecontroller.MergeCloudProfiles(namespacedCloudProfile, cloudProfile)

						version := namespacedCloudProfile.Status.CloudProfileSpec.MachineImages[0].Versions[0]
						// no architectures specified so should default to amd64 as per parent capability definition
						Expect(version.CapabilityFlavors).To(BeEmpty())
						Expect(version.Architectures).To(ConsistOf("amd64"))
					})
				})
			})
		})
	})
})
