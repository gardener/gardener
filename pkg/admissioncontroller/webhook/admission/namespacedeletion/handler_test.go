// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedeletion_test

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/namespacedeletion"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.Background()
		log logr.Logger

		handler *Handler

		ctrl       *gomock.Controller
		mockClient *mockclient.MockClient
		mockReader *mockclient.MockReader

		namespaceName     = "foo"
		projectName       = "bar"
		namespace         *corev1.Namespace
		shootMetadataList *metav1.PartialObjectMetadataList
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		ctrl = gomock.NewController(GinkgoT())
		mockClient = mockclient.NewMockClient(ctrl)
		mockReader = mockclient.NewMockReader(ctrl)

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespaceName,
				Labels: map[string]string{"project.gardener.cloud/name": projectName},
			},
		}

		shootMetadataList = &metav1.PartialObjectMetadataList{}
		shootMetadataList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))

		handler = &Handler{Logger: log, APIReader: mockReader, Client: mockClient}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	test := func(matcher gomegatypes.GomegaMatcher) {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Name: namespaceName}})
		warning, err := handler.ValidateDelete(ctx, nil)
		Expect(warning).To(BeNil())
		Expect(err).To(matcher)
	}

	It("should pass because no projects available", func() {
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
			namespace.DeepCopyInto(obj)
			return nil
		})
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

		test(Succeed())
	})

	It("should pass because namespace is not project related", func() {
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
			(&corev1.Namespace{}).DeepCopyInto(obj)
			return nil
		})

		test(Succeed())
	})

	It("should fail because get namespace fails", func() {
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(errors.New("fake"))

		test(MatchError(ContainSubstring("fake")))
	})

	It("should fail because getting the projects fails", func() {
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
			namespace.DeepCopyInto(obj)
			return nil
		})
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).Return(errors.New("fake"))

		test(MatchError(ContainSubstring("fake")))
	})

	It("should pass because namespace is already gone", func() {
		mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

		test(Succeed())
	})

	Context("related project available", func() {
		var relatedProject gardencorev1beta1.Project

		It("should pass because namespace is already marked for deletion", func() {
			mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				now := metav1.Now()
				namespace.SetDeletionTimestamp(&now)
				namespace.DeepCopyInto(obj)
				return nil
			})
			mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}))

			test(Succeed())
		})

		It("should forbid namespace deletion because project is not marked for deletion", func() {
			mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
				namespace.DeepCopyInto(obj)
				return nil
			})
			mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}))

			test(MatchError(ContainSubstring("direct deletion of namespace")))
		})

		Context("related project marked for deletion ", func() {
			BeforeEach(func() {
				now := metav1.Now()
				relatedProject.SetDeletionTimestamp(&now)

				mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: namespaceName}, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Namespace, _ ...client.GetOption) error {
					namespace.DeepCopyInto(obj)
					return nil
				})
				mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: projectName}, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Project, _ ...client.GetOption) error {
					relatedProject.DeepCopyInto(obj)
					return nil
				})
				mockClient.EXPECT().Scheme().Return(kubernetes.GardenScheme)
			})

			It("should fail because listing shoots fails", func() {
				mockReader.EXPECT().List(gomock.Any(), shootMetadataList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(_ context.Context, _ *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					return errors.New("fake")
				})

				test(MatchError(ContainSubstring("fake")))
			})

			It("should pass because namespace is does not contain any shoots", func() {
				mockReader.EXPECT().List(gomock.Any(), shootMetadataList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(_ context.Context, _ *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					return nil
				})

				test(Succeed())
			})

			It("should forbid namespace deletion because it still contain shoots", func() {
				mockReader.EXPECT().List(gomock.Any(), shootMetadataList, client.InNamespace(namespaceName), client.Limit(1)).DoAndReturn(func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					list.Items = []metav1.PartialObjectMetadata{{ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: namespaceName}}}
					return nil
				})

				test(MatchError(ContainSubstring("still contains Shoots")))
			})
		})
	})

	It("should do nothing for on Create operation", func() {
		warning, err := handler.ValidateCreate(ctx, namespace)
		Expect(warning).To(BeNil())
		Expect(err).NotTo(HaveOccurred())
	})

	It("should do nothing for on Update operation", func() {
		warning, err := handler.ValidateUpdate(ctx, nil, namespace)
		Expect(warning).To(BeNil())
		Expect(err).NotTo(HaveOccurred())
	})
})
