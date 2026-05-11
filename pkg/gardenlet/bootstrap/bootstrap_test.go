// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"
	"crypto/x509/pkix"
	"os"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Bootstrap", func() {
	var (
		runtimeClient client.Client
		ctx           context.Context
		ctxCancel     context.CancelFunc
		testLogger    = logr.Discard()
	)

	BeforeEach(func() {
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		ctx, ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)
	})

	AfterEach(func() {
		ctxCancel()
	})

	Describe("#RequestKubeconfigWithBootstrapClient", func() {
		var (
			kubeClient            *fake.Clientset
			bootstrapClientConfig *rest.Config

			kubeconfigKey          client.ObjectKey
			bootstrapKubeconfigKey client.ObjectKey
			selfHostedShootMeta    *types.NamespacedName

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

			kubeClient = fake.NewClientset()
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

			// rest config for the bootstrap client
			bootstrapClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
				Insecure: false,
				CAFile:   "filepath",
			}}
		})

		When("gardenlet is responsible for seed", func() {
			var seedConfig = &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{ObjectMeta: metav1.ObjectMeta{Name: "test"}}}

			It("should not return an error", func() {
				DeferCleanup(test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
					return approvedCSR.Name, nil
				}))

				kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, &approvedCSR, nil
				})

				bootstrapClientSet := fakekubernetes.NewClientSetBuilder().
					WithRESTConfig(bootstrapClientConfig).
					WithKubernetes(kubeClient).
					Build()

				kubeconfig, csrName, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, runtimeClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedConfig, selfHostedShootMeta, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(kubeconfig).ToNot(BeEmpty())
				Expect(csrName).ToNot(BeEmpty())

				secret := &corev1.Secret{}
				Expect(runtimeClient.Get(ctx, kubeconfigKey, secret)).To(Succeed())
				Expect(secret.Data[kubernetes.KubeConfig]).ToNot(BeEmpty())

				Expect(runtimeClient.Get(ctx, bootstrapKubeconfigKey, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should return an error - the CSR got denied", func() {
				DeferCleanup(test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
					return deniedCSR.Name, nil
				}))

				kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, &deniedCSR, nil
				})

				bootstrapClientSet := fakekubernetes.NewClientSetBuilder().
					WithRESTConfig(bootstrapClientConfig).
					WithKubernetes(kubeClient).
					Build()

				_, _, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, runtimeClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedConfig, selfHostedShootMeta, nil)
				Expect(err).To(MatchError(ContainSubstring("is denied")))
			})

			It("should return an error - the CSR failed", func() {
				DeferCleanup(test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
					return failedCSR.Name, nil
				}))

				kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, &failedCSR, nil
				})

				bootstrapClientSet := fakekubernetes.NewClientSetBuilder().
					WithRESTConfig(bootstrapClientConfig).
					WithKubernetes(kubeClient).
					Build()

				_, _, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, runtimeClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedConfig, selfHostedShootMeta, nil)
				Expect(err).To(MatchError(ContainSubstring("failed")))
			})
		})

		When("gardenlet is responsible for shoot", func() {
			BeforeEach(func() {
				Expect(os.Setenv("NAMESPACE", "kube-system")).To(Succeed())
				DeferCleanup(func() { Expect(os.Setenv("NAMESPACE", "")).To(Succeed()) })

				selfHostedShootMeta = &types.NamespacedName{Namespace: "shoot-namespace", Name: "shoot-name"}
			})

			It("should not return an error", func() {
				DeferCleanup(test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
					return approvedCSR.Name, nil
				}))

				kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, &approvedCSR, nil
				})

				bootstrapClientSet := fakekubernetes.NewClientSetBuilder().
					WithRESTConfig(bootstrapClientConfig).
					WithKubernetes(kubeClient).
					Build()

				kubeconfig, csrName, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, runtimeClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, nil, selfHostedShootMeta, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(kubeconfig).ToNot(BeEmpty())
				Expect(csrName).ToNot(BeEmpty())

				secret := &corev1.Secret{}
				Expect(runtimeClient.Get(ctx, kubeconfigKey, secret)).To(Succeed())
				Expect(secret.Data[kubernetes.KubeConfig]).ToNot(BeEmpty())

				Expect(runtimeClient.Get(ctx, bootstrapKubeconfigKey, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should return an error - the CSR got denied", func() {
				DeferCleanup(test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
					return deniedCSR.Name, nil
				}))

				kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, &deniedCSR, nil
				})

				bootstrapClientSet := fakekubernetes.NewClientSetBuilder().
					WithRESTConfig(bootstrapClientConfig).
					WithKubernetes(kubeClient).
					Build()

				_, _, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, runtimeClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, nil, selfHostedShootMeta, nil)
				Expect(err).To(MatchError(ContainSubstring("is denied")))
			})

			It("should return an error - the CSR failed", func() {
				DeferCleanup(test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
					return failedCSR.Name, nil
				}))

				kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, &failedCSR, nil
				})

				bootstrapClientSet := fakekubernetes.NewClientSetBuilder().
					WithRESTConfig(bootstrapClientConfig).
					WithKubernetes(kubeClient).
					Build()

				_, _, err := RequestKubeconfigWithBootstrapClient(ctx, testLogger, runtimeClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, nil, selfHostedShootMeta, nil)
				Expect(err).To(MatchError(ContainSubstring("failed")))
			})
		})
	})

	Describe("#DeleteBootstrapAuth", func() {
		var (
			csrName = "csr-name"
			csrKey  = client.ObjectKey{Name: csrName}
		)

		It("should return an error because the CSR was not found", func() {
			Expect(DeleteBootstrapAuth(ctx, runtimeClient, runtimeClient, csrName)).NotTo(Succeed())
		})

		It("should delete nothing because the username in the CSR does not match a known pattern", func() {
			Expect(runtimeClient.Create(ctx, &certificatesv1.CertificateSigningRequest{ObjectMeta: metav1.ObjectMeta{Name: csrKey.Name}})).To(Succeed())

			Expect(DeleteBootstrapAuth(ctx, runtimeClient, runtimeClient, csrName)).To(Succeed())
		})

		It("should delete the bootstrap token secret", func() {
			var (
				bootstrapTokenID         = "12345"
				bootstrapTokenSecretName = "bootstrap-token-" + bootstrapTokenID
				bootstrapTokenUserName   = bootstraptokenapi.BootstrapUserPrefix + bootstrapTokenID
				bootstrapTokenSecret     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: bootstrapTokenSecretName}}
			)

			csr := &certificatesv1.CertificateSigningRequest{ObjectMeta: metav1.ObjectMeta{Name: csrKey.Name}, Spec: certificatesv1.CertificateSigningRequestSpec{Username: bootstrapTokenUserName}}
			Expect(runtimeClient.Create(ctx, csr)).To(Succeed())
			Expect(runtimeClient.Create(ctx, bootstrapTokenSecret)).To(Succeed())

			Expect(DeleteBootstrapAuth(ctx, runtimeClient, runtimeClient, csrName)).To(Succeed())

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(bootstrapTokenSecret), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should delete the service account and cluster role binding", func() {
			var (
				seedName                = "foo"
				serviceAccountName      = "foo"
				serviceAccountNamespace = v1beta1constants.GardenNamespace
				serviceAccountUserName  = serviceaccount.MakeUsername(serviceAccountNamespace, serviceAccountName)
				serviceAccount          = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: serviceAccountNamespace, Name: serviceAccountName}}
				clusterRoleBinding      = &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: gardenletbootstraputil.ClusterRoleBindingName(serviceAccountNamespace, seedName)}}
			)

			csr := &certificatesv1.CertificateSigningRequest{ObjectMeta: metav1.ObjectMeta{Name: csrKey.Name}, Spec: certificatesv1.CertificateSigningRequestSpec{Username: serviceAccountUserName}}
			Expect(runtimeClient.Create(ctx, csr)).To(Succeed())
			Expect(runtimeClient.Create(ctx, serviceAccount)).To(Succeed())
			Expect(runtimeClient.Create(ctx, clusterRoleBinding)).To(Succeed())

			Expect(DeleteBootstrapAuth(ctx, runtimeClient, runtimeClient, csrName)).To(Succeed())

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), &corev1.ServiceAccount{})).To(BeNotFoundError())
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
		})
	})
})
