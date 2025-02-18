// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"
	"crypto/x509/pkix"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Bootstrap", func() {
	var (
		ctrl       *gomock.Controller
		reader     *mockclient.MockReader
		writer     *mockclient.MockWriter
		seedClient *mockclient.MockClient
		ctx        context.Context
		ctxCancel  context.CancelFunc
		testLogger = logr.Discard()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		reader = mockclient.NewMockReader(ctrl)
		writer = mockclient.NewMockWriter(ctrl)
		seedClient = mockclient.NewMockClient(ctrl)
		ctx, ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)
	})

	AfterEach(func() {
		ctrl.Finish()
		ctxCancel()
	})

	Describe("#RequestKubeconfigWithBootstrapClient", func() {
		var (
			seedName = "test"

			kubeClient            *fake.Clientset
			bootstrapClientConfig *rest.Config

			gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection
			kubeconfigKey          client.ObjectKey
			bootstrapKubeconfigKey client.ObjectKey

			approvedCSR = certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "approved-csr",
				},
				Status: certificatesv1.CertificateSigningRequestStatus{
					Conditions: []certificatesv1.CertificateSigningRequestCondition{
						{
							Type: certificatesv1.CertificateApproved,
						},
					},
					Certificate: []byte("my-cert"),
				},
			}

			deniedCSR = certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "denied-csr",
				},
				Status: certificatesv1.CertificateSigningRequestStatus{
					Conditions: []certificatesv1.CertificateSigningRequestCondition{
						{
							Type: certificatesv1.CertificateDenied,
						},
					},
				},
			}

			failedCSR = certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "failed-csr",
				},
				Status: certificatesv1.CertificateSigningRequestStatus{
					Conditions: []certificatesv1.CertificateSigningRequestCondition{
						{
							Type: certificatesv1.CertificateFailed,
						},
					},
				},
			}
		)

		BeforeEach(func() {
			secretReference := corev1.SecretReference{
				Name:      "gardenlet-kubeconfig",
				Namespace: "garden",
			}
			kubeconfigKey = kubernetesutils.ObjectKeyFromSecretRef(secretReference)

			bootstrapSecretReference := corev1.SecretReference{
				Name:      "bootstrap-kubeconfig",
				Namespace: "garden",
			}
			bootstrapKubeconfigKey = kubernetesutils.ObjectKeyFromSecretRef(bootstrapSecretReference)

			kubeClient = fake.NewSimpleClientset()
			kubeClient.Fake = testing.Fake{Resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "certificatesigningrequests",
							Namespaced: true,
							Group:      certificatesv1.GroupName,
							Version:    certificatesv1.SchemeGroupVersion.Version,
							Kind:       "CertificateSigningRequest",
						},
					},
				},
			}}

			// gardenClientConnection with required bootstrap secret kubeconfig secret
			// in a non-test environment we would use two different secrets
			gardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
				BootstrapKubeconfig: &bootstrapSecretReference,
				KubeconfigSecret:    &secretReference,
			}

			// rest config for the bootstrap client
			bootstrapClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
				Insecure: false,
				CAFile:   "filepath",
			}}
		})

		It("should not return an error", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return approvedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &approvedCSR, nil
			})

			bootstrapClientSet := kubernetesfake.NewClientSetBuilder().
				WithRESTConfig(bootstrapClientConfig).
				WithKubernetes(kubeClient).
				Build()

			seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{}))

			seedClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
				DoAndReturn(func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					Expect(secret.Name).To(Equal(gardenClientConnection.KubeconfigSecret.Name))
					Expect(secret.Namespace).To(Equal(gardenClientConnection.KubeconfigSecret.Namespace))
					Expect(secret.Data).ToNot(BeNil())
					Expect(secret.Data[kubernetes.KubeConfig]).ToNot(BeEmpty())
					return nil
				})
			seedClient.EXPECT().Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenClientConnection.BootstrapKubeconfig.Name,
					Namespace: gardenClientConnection.BootstrapKubeconfig.Namespace,
				},
			})

			kubeconfig, csrName, seedName, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, seedClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedName, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(kubeconfig).ToNot(BeEmpty())
			Expect(csrName).ToNot(BeEmpty())
			Expect(seedName).ToNot(BeEmpty())
		})

		It("should return an error - the CSR got denied", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return deniedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &deniedCSR, nil
			})

			bootstrapClientSet := kubernetesfake.NewClientSetBuilder().
				WithRESTConfig(bootstrapClientConfig).
				WithKubernetes(kubeClient).
				Build()

			_, _, _, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, seedClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedName, nil)
			Expect(err).To(MatchError(ContainSubstring("is denied")))
		})

		It("should return an error - the CSR failed", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return failedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &failedCSR, nil
			})

			bootstrapClientSet := kubernetesfake.NewClientSetBuilder().
				WithRESTConfig(bootstrapClientConfig).
				WithKubernetes(kubeClient).
				Build()

			_, _, _, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, seedClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedName, nil)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})
	})

	Describe("#DeleteBootstrapAuth", func() {
		var (
			csrName = "csr-name"
			csrKey  = client.ObjectKey{Name: csrName}
		)

		It("should return an error because the CSR was not found", func() {
			reader.EXPECT().
				Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).
				Return(apierrors.NewNotFound(schema.GroupResource{Resource: "CertificateSigningRequests"}, csrName))

			Expect(DeleteBootstrapAuth(ctx, reader, writer, csrName)).NotTo(Succeed())
		})

		It("should delete nothing because the username in the CSR does not match a known pattern", func() {
			reader.EXPECT().
				Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).
				Return(nil)

			Expect(DeleteBootstrapAuth(ctx, reader, writer, csrName)).To(Succeed())
		})

		It("should delete the bootstrap token secret", func() {
			var (
				bootstrapTokenID         = "12345"
				bootstrapTokenSecretName = "bootstrap-token-" + bootstrapTokenID
				bootstrapTokenUserName   = bootstraptokenapi.BootstrapUserPrefix + bootstrapTokenID
				bootstrapTokenSecret     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: bootstrapTokenSecretName}}
			)

			gomock.InOrder(
				reader.EXPECT().
					Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, csr *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
						csr.Spec.Username = bootstrapTokenUserName
						return nil
					}),
				writer.EXPECT().
					Delete(ctx, bootstrapTokenSecret),
			)

			Expect(DeleteBootstrapAuth(ctx, reader, writer, csrName)).To(Succeed())
		})

		It("should delete the service account and cluster role binding", func() {
			var (
				seedName                = "foo"
				serviceAccountName      = "foo"
				serviceAccountNamespace = v1beta1constants.GardenNamespace
				serviceAccountUserName  = serviceaccount.MakeUsername(serviceAccountNamespace, serviceAccountName)
				serviceAccount          = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: serviceAccountNamespace, Name: serviceAccountName}}

				clusterRoleBinding = &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: gardenletbootstraputil.ClusterRoleBindingName(serviceAccountNamespace, seedName)}}
			)

			gomock.InOrder(
				reader.EXPECT().
					Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1.CertificateSigningRequest{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, csr *certificatesv1.CertificateSigningRequest, _ ...client.GetOption) error {
						csr.Spec.Username = serviceAccountUserName
						return nil
					}),
				writer.EXPECT().
					Delete(ctx, serviceAccount),
				writer.EXPECT().
					Delete(ctx, clusterRoleBinding),
			)

			Expect(DeleteBootstrapAuth(ctx, reader, writer, csrName)).To(Succeed())
		})
	})
})
