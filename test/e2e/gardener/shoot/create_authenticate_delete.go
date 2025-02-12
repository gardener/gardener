// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	f := framework.NewGardenerFramework(e2e.DefaultGardenConfig("garden-local"))

	It("Create, Authenticate, Delete", Label("authentication"), func() {
		shoot1 := e2e.DefaultWorkerlessShoot("e2e-auth-one")
		shoot1.Namespace = f.ProjectNamespace
		shoot2 := e2e.DefaultWorkerlessShoot("e2e-auth-two")
		shoot2.Namespace = f.ProjectNamespace

		By("Create Shoots")
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
		defer cancel()

		Expect(f.CreateShoot(ctx, shoot1, false)).To(Succeed())
		Expect(f.CreateShoot(ctx, shoot2, false)).To(Succeed())

		Expect(f.WaitForShootToBeCreated(ctx, shoot1)).To(Succeed())
		Expect(f.WaitForShootToBeCreated(ctx, shoot2)).To(Succeed())

		By("Verify shoot access using admin kubeconfig")
		shoot1Client, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, shoot1)
		Expect(err).NotTo(HaveOccurred())

		shoot2Client, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, shoot2)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1Client.Client(), shoot1, true)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2Client.Client(), shoot2, true)).To(Succeed())
		}).Should(Succeed())

		By("Verify a shoot cannot be accessed with a client certificate from another shoot")
		shoot1NoAccessRestConfig := copyRESTConfigAndInjectAuthorization(shoot1Client.RESTConfig(), shoot2Client.RESTConfig())
		shoot1NoAccessClient, err := client.New(shoot1NoAccessRestConfig, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		shoot2NoAccessRestConfig := copyRESTConfigAndInjectAuthorization(shoot2Client.RESTConfig(), shoot1Client.RESTConfig())
		shoot2NoAccessClient, err := client.New(shoot2NoAccessRestConfig, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		Consistently(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1NoAccessClient, shoot1, false)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2NoAccessClient, shoot2, false)).To(Succeed())
		}, "10s").Should(Succeed())

		By("Verify shoot access using service account token kubeconfig")
		shoot1TokenClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, shoot1Client, "shoot-one")
		Expect(err).NotTo(HaveOccurred())

		shoot2TokenClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, shoot2Client, "shoot-two")
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1TokenClient.Client(), shoot1, true)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2TokenClient.Client(), shoot2, true)).To(Succeed())
		}).Should(Succeed())

		By("Verify a shoot cannot be accessed with a service account token from another shoot")
		shoot1NoAccessTokenRestConfig := copyRESTConfigAndInjectAuthorization(shoot1TokenClient.RESTConfig(), shoot2TokenClient.RESTConfig())
		shoot1NoAccessTokenClient, err := client.New(shoot1NoAccessTokenRestConfig, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		shoot2NoAccessTokenRestConfig := copyRESTConfigAndInjectAuthorization(shoot2TokenClient.RESTConfig(), shoot1TokenClient.RESTConfig())
		shoot2NoAccessTokenClient, err := client.New(shoot2NoAccessTokenRestConfig, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		Consistently(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1NoAccessTokenClient, shoot1, false)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2NoAccessTokenClient, shoot2, false)).To(Succeed())
		}, "10s").Should(Succeed())

		By("Verify that authentication with istio tls termination cannot be bypassed")
		externalAddress, httpClient, err := httpClientForRESTConfig(shoot1Client.RESTConfig())
		Expect(err).To(Succeed())

		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, externalAddress, http.NoBody)
		Expect(err).To(Succeed())
		httpRequest.Header = map[string][]string{
			"X-Remote-User":  {"kubernetes-admin"},
			"X-Remote-Group": {"system:masters"},
		}

		var httpResponse *http.Response
		Eventually(func(g Gomega) {
			var err error
			httpResponse, err = httpClient.Do(httpRequest)
			g.Expect(err).To(Succeed())
		}).Should(Succeed())

		Expect(httpResponse.StatusCode).To(Equal(http.StatusUnauthorized))

		By("Verify that users cannot escalate their privileges with istio tls termination")
		customClient, err := clientWithCustomTransport(ctx, f.GardenClient, shoot1, &injectHeaderTransport{
			Headers: map[string]string{
				"X-Remote-User":  "fake-kubernetes-admin",
				"X-Remote-Group": "system:masters",
			},
		})
		Expect(err).ToNot(HaveOccurred())

		var selfSubjectReview *authenticationv1.SelfSubjectReview
		Eventually(func(g Gomega) {
			var err error
			selfSubjectReview, err = customClient.Kubernetes().AuthenticationV1().SelfSubjectReviews().
				Create(context.TODO(), &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
			g.Expect(err).ToNot(HaveOccurred())
		}).Should(Succeed())

		Expect(selfSubjectReview.Status.UserInfo.Username).To(Equal("kubernetes-admin"))
		Expect(selfSubjectReview.Status.UserInfo.Groups).To(Equal([]string{"gardener.cloud:system:viewers", "system:authenticated"}))

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()

		Expect(f.DeleteShoot(ctx, shoot1)).To(Succeed())
		Expect(f.DeleteShoot(ctx, shoot2)).To(Succeed())

		Expect(f.WaitForShootToBeDeleted(ctx, shoot1)).To(Succeed())
		Expect(f.WaitForShootToBeDeleted(ctx, shoot2)).To(Succeed())
	})
})

func copyRESTConfigAndInjectAuthorization(restConfig *rest.Config, authorization *rest.Config) *rest.Config {
	newRESTConfig := rest.CopyConfig(restConfig)
	newRESTConfig.BearerToken = authorization.BearerToken
	newRESTConfig.BearerTokenFile = authorization.BearerTokenFile
	newRESTConfig.CertData = authorization.CertData
	newRESTConfig.CertFile = authorization.CertFile
	newRESTConfig.KeyData = authorization.KeyData
	newRESTConfig.KeyFile = authorization.KeyFile
	newRESTConfig.Username = authorization.Username
	newRESTConfig.Password = authorization.Password

	return newRESTConfig
}

func validateShootAccess(ctx context.Context, shootClient client.Client, shoot *gardencorev1beta1.Shoot, shouldHaveAccess bool) error {
	var clusterIdentity corev1.ConfigMap

	if err := retry.UntilTimeout(ctx, 500*time.Millisecond, 5*time.Second, func(ctx context.Context) (bool, error) {
		if err := shootClient.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: "cluster-identity"}, &clusterIdentity); err != nil {
			if !shouldHaveAccess && isAuthorizationError(err) {
				return retry.SevereError(err)
			}
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		if !shouldHaveAccess && isAuthorizationError(err) {
			return nil
		}
		return err
	}

	if !shouldHaveAccess {
		return fmt.Errorf("unexpected access to kube-apiserver of shoot %q", shoot.Name)
	}

	if shoot.Status.ClusterIdentity == nil {
		return fmt.Errorf("shoot %q has no cluster-identity", shoot.Name)
	}

	if clusterIdentity.Data["cluster-identity"] != *shoot.Status.ClusterIdentity {
		return fmt.Errorf("connection to the wrong kube-apiserver established: expected cluster-identity %q, got cluster-identity %q", *shoot.Status.ClusterIdentity, clusterIdentity.Data["cluster-identity"])
	}

	return nil
}

func isAuthorizationError(err error) bool {
	return apierrors.IsUnauthorized(err) || strings.HasSuffix(err.Error(), "remote error: tls: unknown certificate authority")
}

func httpClientForRESTConfig(restConfig *rest.Config) (string, *http.Client, error) {
	rootCAs := x509.NewCertPool()
	if restConfig.CAFile != "" {
		caCert, err := os.ReadFile(restConfig.CAFile)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		if !rootCAs.AppendCertsFromPEM(caCert) {
			return "", nil, fmt.Errorf("failed to append CA certificate")
		}
	}
	if len(restConfig.CAData) > 0 {
		if !rootCAs.AppendCertsFromPEM(restConfig.CAData) {
			return "", nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	return restConfig.Host, &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    rootCAs,
				MinVersion: tls.VersionTLS12,
			},
		},
	}, nil
}

func clientWithCustomTransport(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, transport http.RoundTripper) (kubernetes.Interface, error) {
	viewer, err := access.RequestViewerKubeconfigForShoot(ctx, gardenClient, shoot, ptr.To[int64](7200))
	if err != nil {
		return nil, fmt.Errorf("failed to request viewer kubeconfig: %w", err)
	}

	c, err := kubernetes.NewClientFromBytes(
		viewer,
		kubernetes.WithClientOptions(client.Options{
			HTTPClient: &http.Client{
				Transport: transport,
			},
			Scheme: kubernetes.ShootScheme,
		}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return c, err
}

type injectHeaderTransport struct {
	Headers map[string]string
}

func (c *injectHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range c.Headers {
		req.Header.Add(key, value)
	}
	return http.DefaultTransport.RoundTrip(req)
}
