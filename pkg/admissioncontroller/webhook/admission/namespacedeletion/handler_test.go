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
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/namespacedeletion"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.Background()
		log logr.Logger

		handler *Handler

		fakeClient client.Client

		namespaceName = "foo"
		projectName   = "bar"
		namespace     *corev1.Namespace
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespaceName,
				Labels: map[string]string{"project.gardener.cloud/name": projectName},
			},
		}

		handler = &Handler{Logger: log, APIReader: fakeClient, Client: fakeClient}
	})

	test := func(matcher gomegatypes.GomegaMatcher) {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Name: namespaceName}})
		warning, err := handler.ValidateDelete(ctx, nil)
		Expect(warning).To(BeNil())
		Expect(err).To(matcher)
	}

	It("should pass because no projects available", func() {
		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
		// Project not created → Get returns NotFound

		test(Succeed())
	})

	It("should pass because namespace is not project related", func() {
		Expect(fakeClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		})).To(Succeed())

		test(Succeed())
	})

	It("should fail because get namespace fails", func() {
		errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*corev1.Namespace); ok {
					return errors.New("fake")
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).Build()
		handler.Client = errClient
		handler.APIReader = errClient

		test(MatchError(ContainSubstring("fake")))
	})

	It("should fail because getting the projects fails", func() {
		errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
			WithObjects(namespace).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*gardencorev1beta1.Project); ok {
						return errors.New("fake")
					}
					return c.Get(ctx, key, obj, opts...)
				},
			}).Build()
		handler.Client = errClient
		handler.APIReader = errClient

		test(MatchError(ContainSubstring("fake")))
	})

	It("should pass because namespace is already gone", func() {
		// Namespace not created → Get returns NotFound

		test(Succeed())
	})

	Context("related project available", func() {
		It("should pass because namespace is already marked for deletion", func() {
			now := metav1.Now()
			ns := namespace.DeepCopy()
			ns.Finalizers = []string{"kubernetes"}
			ns.SetDeletionTimestamp(&now)
			project := &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName},
			}
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
				WithObjects(ns, project).Build()
			handler.Client = fakeClient
			handler.APIReader = fakeClient

			test(Succeed())
		})

		It("should forbid namespace deletion because project is not marked for deletion", func() {
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

			project := &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName},
			}
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			test(MatchError(ContainSubstring("direct deletion of namespace")))
		})

		Context("related project marked for deletion ", func() {
			BeforeEach(func() {
				now := metav1.Now()
				project := &gardencorev1beta1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:              projectName,
						Finalizers:        []string{"kubernetes"},
						DeletionTimestamp: &now,
					},
				}
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
					WithObjects(namespace, project).Build()
				handler.Client = fakeClient
				handler.APIReader = fakeClient
			})

			It("should fail because listing shoots fails", func() {
				now := metav1.Now()
				project := &gardencorev1beta1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:              projectName,
						Finalizers:        []string{"kubernetes"},
						DeletionTimestamp: &now,
					},
				}
				errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
					WithObjects(namespace, project).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
							if pml, ok := list.(*metav1.PartialObjectMetadataList); ok && pml.GetObjectKind().GroupVersionKind() == gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList") {
								return errors.New("fake")
							}

							return c.List(ctx, list, opts...)
						},
					}).Build()
				handler.Client = errClient
				handler.APIReader = errClient

				test(MatchError(ContainSubstring("fake")))
			})

			It("should pass because namespace is does not contain any shoots", func() {
				test(Succeed())
			})

			It("should forbid namespace deletion because it still contain shoots", func() {
				existingShoot := &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: namespaceName},
				}
				Expect(fakeClient.Create(ctx, existingShoot)).To(Succeed())

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
