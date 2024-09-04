// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	namespacedcloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("NamespacedCloudProfile Reconciler", func() {
	const finalizerName = "gardener"

	var (
		ctx        context.Context
		ctrl       *gomock.Controller
		c          *mockclient.MockClient
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

		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		fakeErr = errors.New("fake err")
		reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: c, Recorder: &record.FakeRecorder{}}

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
				Name:            namespacedCloudProfileName,
				Namespace:       namespaceName,
				ResourceVersion: "42",
			},
			Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
				Parent: gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				},
			},
		}

		newExpiryDate = metav1.Now()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return nil because object not found", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return err because object reading failed", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).Return(fakeErr)

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).To(MatchError(fakeErr))
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

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"status":{"cloudProfileSpec":{"kubernetes":{"versions":[{"version":"1.0.0"}]},"machineImages":[],"machineTypes":[]}}}`))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should merge Kubernetes versions correctly", func() {
			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0", ExpirationDate: &newExpiryDate},
			}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"status":{"cloudProfileSpec":{"kubernetes":{"versions":[{"expirationDate":"%s","version":"1.0.0"}]},"machineImages":[],"machineTypes":[]}}}`, newExpiryDate.UTC().Format(time.RFC3339))))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should set observedGeneration correctly", func() {
			namespacedCloudProfile.Generation = 7

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"status":{"cloudProfileSpec":{"kubernetes":{"versions":[{"version":"1.0.0"}]},"machineImages":[],"machineTypes":[]},"observedGeneration":7}}`))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when deletion timestamp set", func() {
			BeforeEach(func() {
				now := metav1.Now()
				namespacedCloudProfile.DeletionTimestamp = &now
				namespacedCloudProfile.Finalizers = []string{finalizerName}

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
					*obj = *namespacedCloudProfile
					return nil
				})
			})

			It("should do nothing because finalizer is not present", func() {
				namespacedCloudProfile.Finalizers = nil

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because Shoot referencing NamespacedCloudProfile exists", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: "test-namespace"},
							Spec: gardencorev1beta1.ShootSpec{
								CloudProfile: &gardencorev1beta1.CloudProfileReference{
									Kind: "NamespacedCloudProfile",
									Name: namespacedCloudProfileName,
								},
							},
						},
					}}).DeepCopyInto(obj)
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("Cannot delete NamespacedCloudProfile")))
			})

			It("should remove the finalizer (error)", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return fakeErr
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should remove the finalizer (no error)", func() {
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ShootList{}).DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(`{"metadata":{"finalizers":null,"resourceVersion":"42"}}`))
					return nil
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when deletion timestamp not set", func() {
			BeforeEach(func() {
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
					namespacedCloudProfile.DeepCopyInto(obj)
					return nil
				})
			})

			It("should ensure the finalizer (error)", func() {
				errToReturn := apierrors.NewNotFound(schema.GroupResource{}, namespaceName+"/"+namespacedCloudProfileName)

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
					return errToReturn
				})

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(err))
			})

			It("should ensure the finalizer (no error)", func() {
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"metadata":{"finalizers":["%s"],"resourceVersion":"42"}}`, finalizerName)))
					return nil
				})

				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
					cloudProfile.DeepCopyInto(obj)
					return nil
				})

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
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

		It("should ignore versions specified only in NamespacedCloudProfile", func() {
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "test-image-namespaced",
					Versions: []gardencorev1beta1.MachineImageVersion{{
						ExpirableVersion:         gardencorev1beta1.ExpirableVersion{Version: "1.0.0"},
						CRI:                      []gardencorev1beta1.CRI{{Name: "containerd"}},
						Architectures:            []string{"amd64"},
						KubeletVersionConstraint: ptr.To("==1.30.0"),
					}},
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
				},
			}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"status":{"cloudProfileSpec":{"machineImages":[{"name":"test-image","updateStrategy":"major","versions":[{"architectures":["amd64"],"cri":[{"name":"containerd"}],"kubeletVersionConstraint":"==1.30.0","version":"1.0.0"}]}],"machineTypes":[]}}}`))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should merge MachineImages correctly", func() {
			newExpiryDate := metav1.Now()
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "test-image",
					Versions: []gardencorev1beta1.MachineImageVersion{{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0", ExpirationDate: &newExpiryDate},
					}},
				},
			}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(fmt.Sprintf(`{"status":{"cloudProfileSpec":{"machineImages":[{"name":"test-image","updateStrategy":"major","versions":[{"architectures":["amd64"],"cri":[{"name":"containerd"}],"expirationDate":"%s","kubeletVersionConstraint":"==1.30.0","version":"1.0.0"}]}],"machineTypes":[]}}}`, newExpiryDate.UTC().Format(time.RFC3339))))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
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

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				Expect(patch.Data(o)).To(BeEquivalentTo(`{"status":{"cloudProfileSpec":{"machineImages":[],"machineTypes":[{"cpu":"1","gpu":"5","memory":"3Gi","name":"test-type-namespaced"}]}}}`))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
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

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespacedCloudProfileName, Namespace: namespaceName}, gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.NamespacedCloudProfile, _ ...client.GetOption) error {
				namespacedCloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any())

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: cloudProfileName}, gomock.AssignableToTypeOf(&gardencorev1beta1.CloudProfile{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.CloudProfile, _ ...client.GetOption) error {
				cloudProfile.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.NamespacedCloudProfile{}), gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
				// Order of machine type array in patch is not guaranteed
				Expect(patch.Data(o)).To(And(
					ContainSubstring(`{"status":{"cloudProfileSpec":{"machineImages":[],"machineTypes":[`),
					ContainSubstring(`{"cpu":"1","gpu":"5","memory":"3Gi","name":"test-type-namespaced"}`),
					ContainSubstring(`{"cpu":"2","gpu":"7","memory":"10Gi","name":"test-type"}`),
				))
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespacedCloudProfileName, Namespace: namespaceName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
