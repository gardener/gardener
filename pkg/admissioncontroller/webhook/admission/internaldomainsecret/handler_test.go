// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internaldomainsecret_test

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/internaldomainsecret"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("handler", func() {
	var (
		ctrl       *gomock.Controller
		mockReader *mockclient.MockReader

		ctx     = context.TODO()
		fakeErr = errors.New("fake err")
		log     logr.Logger
		handler *Handler

		secret            *corev1.Secret
		shootMetadataList *metav1.PartialObjectMetadataList
		warning           admission.Warnings
		err               error

		resourceName         = "foo"
		regularNamespaceName = "regular-namespace"
		gardenNamespaceName  = v1beta1constants.GardenNamespace
		seedName             string
		seedNamespace        string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)

		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Name: resourceName}})
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &Handler{Logger: log, APIReader: mockReader, Scheme: kubernetes.GardenScheme}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: gardenNamespaceName,
				Annotations: map[string]string{
					"dns.gardener.cloud/provider": "foo",
					"dns.gardener.cloud/domain":   "bar",
				},
				Labels: map[string]string{
					"gardener.cloud/role": "internal-domain",
				},
			},
		}
		shootMetadataList = &metav1.PartialObjectMetadataList{}
		shootMetadataList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))

		seedName = "aws"
		seedNamespace = gardenerutils.ComputeGardenNamespace(seedName)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("ignored requests", func() {
		It("should only handle garden and seed namespaces", func() {
			secret.Namespace = regularNamespaceName

			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())

			warning, err = handler.ValidateUpdate(ctx, secret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())

			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})

	Context("create", func() {
		It("should fail because the check for other internal domain secrets failed", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.InNamespace(gardenNamespaceName),
				client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				client.Limit(1),
			).Return(fakeErr)

			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
		})

		It("should fail because another internal domain secret exists in the garden namespace", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.InNamespace(gardenNamespaceName),
				client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				client.Limit(1),
			).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("there can be only one secret with the 'internal-domain' secret role")))
		})

		It("should fail because another internal domain secret exists in the same seed namespace", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.InNamespace(seedNamespace),
				client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				client.Limit(1),
			).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			secret.Namespace = seedNamespace
			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("there can be only one secret with the 'internal-domain' secret role")))
		})

		It("should fail because the secret misses domain info", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.InNamespace(gardenNamespaceName),
				client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				client.Limit(1),
			)

			secret.Annotations = nil
			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("domain secret has no annotations")))
		})

		It("should pass because no other internal domain secret exists", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.InNamespace(gardenNamespaceName),
				client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				client.Limit(1),
			)

			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})

	Context("update", func() {
		Context("when secret is newly labeled with gardener.cloud/role=internal-domain", func() {
			var oldSecret *corev1.Secret

			BeforeEach(func() {
				oldSecret = secret.DeepCopy()
				oldSecret.Labels = nil
			})

			It("should fail because the check for other internal domain secrets failed", func() {
				mockReader.EXPECT().List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
					client.InNamespace(gardenNamespaceName),
					client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
					client.Limit(1),
				).Return(fakeErr)

				warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
				Expect(warning).To(BeNil())

				statusError, ok := err.(*apierrors.StatusError)
				Expect(ok).To(BeTrue())
				Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
				Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
			})

			It("should fail because another internal domain secret exists in the garden namespace", func() {
				mockReader.EXPECT().List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
					client.InNamespace(gardenNamespaceName),
					client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
					client.Limit(1),
				).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
					return nil
				})

				warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
				Expect(warning).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("there can be only one secret with the 'internal-domain' secret role")))
			})
		})

		It("should fail because the old secret misses domain info", func() {
			oldSecret := secret.DeepCopy()
			oldSecret.Annotations = nil
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("domain secret has no annotations")))
		})

		It("should fail because the secret misses domain info", func() {
			oldSecret := secret.DeepCopy()
			secret.Annotations = nil
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("domain secret has no annotations")))
		})

		It("should forbid because the domain is changed but shoot listing failed", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.Limit(1),
			).Return(fakeErr)

			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/domain"] = "foobar"

			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
		})

		It("should forbid because the global domain is changed but shoots exist", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.Limit(1),
			).DoAndReturn(func(_ context.Context, list client.ObjectList, _ client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/domain"] = "foobar"
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot change domain because there are still shoots left in the system")))
		})

		It("should forbid because the domain in seed namespace is changed but shoots using the seed exist", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}),
				client.MatchingFields{gardencore.ShootSeedName: seedName},
				client.Limit(1),
			).DoAndReturn(func(_ context.Context, shoots *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				shoots.Items = []gardencorev1beta1.Shoot{{}}
				return nil
			})

			secret.Namespace = seedNamespace
			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/domain"] = "foobar"
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot change domain because there are still shoots left in the system")))
		})

		It("should allow because the domain is changed but no shoots exist", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.Limit(1),
			)

			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/domain"] = "foobar"
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})

		It("should allow because the domain is not changed", func() {
			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/provider"] = "foobar"
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})

	Context("delete", func() {
		It("should fail because the shoot listing fails", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.Limit(1),
			).Return(fakeErr)

			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
		})

		It("should fail because at least one shoot exists", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.Limit(1),
			).DoAndReturn(func(_ context.Context, list client.ObjectList, _ client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list.(*metav1.PartialObjectMetadataList))
				return nil
			})

			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot delete internal domain secret because there are still shoots left in the system")))
		})

		It("should fail because at least one shoot on the seed exists", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}),
				client.MatchingFields{gardencore.ShootSeedName: seedName},
				client.Limit(1),
			).DoAndReturn(func(_ context.Context, shoots *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				shoots.Items = []gardencorev1beta1.Shoot{{}}
				return nil
			})

			secret.Namespace = seedNamespace
			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot delete internal domain secret because there are still shoots left in the system")))
		})

		It("should pass because no shoots exist", func() {
			mockReader.EXPECT().List(
				gomock.Any(),
				gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}),
				client.Limit(1),
			)

			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})
})
