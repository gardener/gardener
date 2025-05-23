// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	certificatesclientv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/certificatesigningrequest"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx                    = context.TODO()
		ctrl                   *gomock.Controller
		c                      *mockclient.MockClient
		fakeCertificatesClient certificatesclientv1.CertificateSigningRequestInterface

		sar                *authorizationv1.SubjectAccessReview
		csr                *certificatesv1.CertificateSigningRequest
		reconciler         reconcile.Reconciler
		privateKey         *rsa.PrivateKey
		certificateSubject *pkix.Name

		errNotFound = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		fakeClient := fakeclientset.NewSimpleClientset()
		fakeCertificatesClient = fakeClient.CertificatesV1().CertificateSigningRequests()

		privateKey, _ = secretsutils.FakeGenerateKey(rand.Reader, 4096)
		csr = &certificatesv1.CertificateSigningRequest{
			TypeMeta: metav1.TypeMeta{Kind: "CertificateSigningRequest"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-csr",
			},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Usages: []certificatesv1.KeyUsage{
					certificatesv1.UsageDigitalSignature,
					certificatesv1.UsageKeyEncipherment,
					certificatesv1.UsageClientAuth,
				},
				SignerName: certificatesv1.KubeAPIServerClientSignerName,
			},
		}

		sar = &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: false,
				Denied:  false,
			},
		}

		reconciler = &Reconciler{Client: c, CertificatesClient: fakeCertificatesClient}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return nil because object not found", func() {
		c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(csr), &certificatesv1.CertificateSigningRequest{}).Return(errNotFound)

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when csr is in final state", func() {
		It("should ignore it because certificate is present in the status field", func() {
			c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(csr), gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
					csr.Status.Certificate = []byte("test-certificate")
					csr.DeepCopyInto(obj)
					return nil
				}).AnyTimes()

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore it because csr is Approved", func() {
			c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(csr), gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
					csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
						Type: certificatesv1.CertificateApproved,
					})
					csr.DeepCopyInto(obj)
					return nil
				}).AnyTimes()

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("non seedclient csr", func() {
		BeforeEach(func() {
			certificateSubject = &pkix.Name{
				Organization: []string{"foo"},
			}
			csrData, err := certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			csr.Spec.Request = csrData

			c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(csr), gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
					csr.DeepCopyInto(obj)
					return nil
				}).AnyTimes()
		})

		It("should ignore the csr because csr does not match the requirements for a client certificate for a seed", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("seedclient csr", func() {
		BeforeEach(func() {
			certificateSubject = &pkix.Name{
				Organization: []string{v1beta1constants.SeedsGroup},
				CommonName:   v1beta1constants.SeedUserNamePrefix + "csr-test",
			}
			csrData, err := certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			csr.Spec.Request = csrData

			c.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&authorizationv1.SubjectAccessReview{})).DoAndReturn(func(_ context.Context, obj *authorizationv1.SubjectAccessReview, _ ...client.CreateOption) error {
				// For the simplicity of test, we are assuming SubjectAccessReview will be allowed if user is admin and not allowed for other users.
				if obj.Spec.User == "admin" {
					sar.Status = authorizationv1.SubjectAccessReviewStatus{
						Allowed: true,
					}
				} else {
					sar.Status = authorizationv1.SubjectAccessReviewStatus{
						Allowed: false,
					}
				}
				sar.DeepCopyInto(obj)
				return nil
			})

			reconciler = &Reconciler{Client: c, CertificatesClient: fakeCertificatesClient}
		})

		It("should result an error when user does not has authorization for seedclient subresource (sar.Status.Allowed is false)", func() {
			c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(csr), gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
					csr.Spec.Username = "foo"
					csr.DeepCopyInto(obj)
					return nil
				}).AnyTimes()
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("recognized CSR but SubjectAccessReview was not allowed")))
		})

		It("should approve the csr when user has authorization for seedclient subresource (sar.Status.Allowed is true)", func() {
			_, err := fakeCertificatesClient.Create(ctx, csr, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(csr), gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
					csr.Spec.Username = "admin"
					csr.DeepCopyInto(obj)
					return nil
				}).AnyTimes()

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
