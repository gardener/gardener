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
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/internaldomainsecret"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("handler", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake err")
		log     logr.Logger
		handler *Handler

		fakeClient client.Client

		secret  *corev1.Secret
		warning admission.Warnings
		err     error

		resourceName         = "foo"
		regularNamespaceName = "regular-namespace"
		gardenNamespaceName  = v1beta1constants.GardenNamespace
		seedName             string
		seedNamespace        string
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Name: resourceName}})
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.Shoot{}, gardencore.ShootSeedName, func(obj client.Object) []string {
				shoot, ok := obj.(*gardencorev1beta1.Shoot)
				if !ok || shoot.Spec.SeedName == nil {
					return nil
				}
				return []string{*shoot.Spec.SeedName}
			}).
			Build()
		handler = &Handler{Logger: log, APIReader: fakeClient, Scheme: kubernetes.GardenScheme}

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

		seedName = "aws"
		seedNamespace = gardenerutils.ComputeGardenNamespace(seedName)
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
			errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if pml, ok := list.(*metav1.PartialObjectMetadataList); ok && pml.GetObjectKind().GroupVersionKind() == corev1.SchemeGroupVersion.WithKind("SecretList") {
						return fakeErr
					}
					return c.List(ctx, list, opts...)
				},
			}).Build()
			handler.APIReader = errClient

			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
		})

		It("should fail because another internal domain secret exists in the garden namespace", func() {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-internal-domain-secret",
					Namespace: gardenNamespaceName,
					Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("there can be only one secret with the 'internal-domain' secret role")))
		})

		It("should fail because another internal domain secret exists in the same seed namespace", func() {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-internal-domain-secret",
					Namespace: seedNamespace,
					Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			secret.Namespace = seedNamespace
			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("there can be only one secret with the 'internal-domain' secret role")))
		})

		It("should fail because the secret misses domain info", func() {
			secret.Annotations = nil
			warning, err = handler.ValidateCreate(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("domain secret has no annotations")))
		})

		It("should pass because no other internal domain secret exists", func() {
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
				errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
					List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
						if pml, ok := list.(*metav1.PartialObjectMetadataList); ok && pml.GetObjectKind().GroupVersionKind() == corev1.SchemeGroupVersion.WithKind("SecretList") {
							return fakeErr
						}
						return c.List(ctx, list, opts...)
					},
				}).Build()
				handler.APIReader = errClient

				warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
				Expect(warning).To(BeNil())

				statusError, ok := err.(*apierrors.StatusError)
				Expect(ok).To(BeTrue())
				Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
				Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
			})

			It("should fail because another internal domain secret exists in the garden namespace", func() {
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-internal-domain-secret",
						Namespace: gardenNamespaceName,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
					},
				}
				Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

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
			errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if pml, ok := list.(*metav1.PartialObjectMetadataList); ok && pml.GetObjectKind().GroupVersionKind() == gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList") {
						return fakeErr
					}
					return c.List(ctx, list, opts...)
				},
			}).Build()
			handler.APIReader = errClient

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
			existingShoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden-foo"},
			}
			Expect(fakeClient.Create(ctx, existingShoot)).To(Succeed())

			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/domain"] = "foobar"
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot change domain because there are still shoots left in the system")))
		})

		It("should forbid because the domain in seed namespace is changed but shoots using the seed exist", func() {
			existingShoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden-foo"},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: &seedName,
				},
			}
			Expect(fakeClient.Create(ctx, existingShoot)).To(Succeed())

			secret.Namespace = seedNamespace
			oldSecret := secret.DeepCopy()
			secret.Annotations["dns.gardener.cloud/domain"] = "foobar"
			warning, err = handler.ValidateUpdate(ctx, oldSecret, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot change domain because there are still shoots left in the system")))
		})

		It("should allow because the domain is changed but no shoots exist", func() {
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
			errClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if pml, ok := list.(*metav1.PartialObjectMetadataList); ok && pml.GetObjectKind().GroupVersionKind() == gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList") {
						return fakeErr
					}
					return c.List(ctx, list, opts...)
				},
			}).Build()
			handler.APIReader = errClient

			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())

			statusError, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())
			Expect(statusError.Status().Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(statusError.Status().Message).To(ContainSubstring(fakeErr.Error()))
		})

		It("should fail because at least one shoot exists", func() {
			existingShoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden-foo"},
			}
			Expect(fakeClient.Create(ctx, existingShoot)).To(Succeed())

			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot delete internal domain secret because there are still shoots left in the system")))
		})

		It("should fail because at least one shoot on the seed exists", func() {
			existingShoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden-foo"},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: &seedName,
				},
			}
			Expect(fakeClient.Create(ctx, existingShoot)).To(Succeed())

			secret.Namespace = seedNamespace
			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot delete internal domain secret because there are still shoots left in the system")))
		})

		It("should pass because no shoots exist", func() {
			warning, err = handler.ValidateDelete(ctx, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})
})
