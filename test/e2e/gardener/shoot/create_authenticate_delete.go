// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	"github.com/gardener/gardener/pkg/utils/retry"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	f := framework.NewGardenerFramework(e2e.DefaultGardenConfig("garden-local"))

	It("Create, Authenticate, Delete", Label("authentication"), func() {
		shoot1 := e2e.DefaultShoot("e2e-auth-one")
		shoot1.Namespace = f.ProjectNamespace
		shoot2 := e2e.DefaultShoot("e2e-auth-two")
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
		}).WithTimeout(time.Minute).Should(Succeed())

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
		}).WithTimeout(10 * time.Second).Should(Succeed())

		By("Verify shoot access via apiserver-proxy endpoint")
		shoot1ClientAPIServerProxy, err := getAPIServerProxyClient(shoot1Client.RESTConfig(), shoot1)
		Expect(err).NotTo(HaveOccurred())

		shoot2ClientAPIServerProxy, err := getAPIServerProxyClient(shoot2Client.RESTConfig(), shoot2)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1ClientAPIServerProxy, shoot1, true)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2ClientAPIServerProxy, shoot2, true)).To(Succeed())
		}).WithTimeout(time.Minute).Should(Succeed())

		By("Verify a shoot cannot be accessed with a client certificate from another shoot by manipulating the apiserver-proxy header")
		shoot1NoAccessClientAPIServerProxy, err := getAPIServerProxyClient(shoot1NoAccessRestConfig, shoot1)
		Expect(err).NotTo(HaveOccurred())

		shoot2NoAccessClientAPIServerProxy, err := getAPIServerProxyClient(shoot2NoAccessRestConfig, shoot2)
		Expect(err).NotTo(HaveOccurred())

		Consistently(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1NoAccessClientAPIServerProxy, shoot1, false)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2NoAccessClientAPIServerProxy, shoot2, false)).To(Succeed())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		By("Verify shoot access using service account token kubeconfig")
		shoot1TokenClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, shoot1Client, "shoot-one")
		Expect(err).NotTo(HaveOccurred())

		shoot2TokenClient, err := access.CreateShootClientFromStaticServiceAccountToken(ctx, shoot2Client, "shoot-two")
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1TokenClient.Client(), shoot1, true)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2TokenClient.Client(), shoot2, true)).To(Succeed())
		}).WithTimeout(time.Minute).Should(Succeed())

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
		}).WithTimeout(10 * time.Second).Should(Succeed())

		By("Verify shoot access using service account token kubeconfig via apiserver-proxy endpoint")
		shoot1TokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot1TokenClient.RESTConfig(), shoot1)
		Expect(err).NotTo(HaveOccurred())

		shoot2TokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot2TokenClient.RESTConfig(), shoot2)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1TokenClientAPIServerProxy, shoot1, true)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2TokenClientAPIServerProxy, shoot2, true)).To(Succeed())
		}).WithTimeout(time.Minute).Should(Succeed())

		By("Verify a shoot cannot be accessed with a service account token from another shoot by manipulating the apiserver-proxy header")
		shoot1NoAccessTokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot1NoAccessTokenRestConfig, shoot1)
		Expect(err).NotTo(HaveOccurred())

		shoot2NoAccessTokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot2NoAccessTokenRestConfig, shoot2)
		Expect(err).NotTo(HaveOccurred())

		Consistently(func(g Gomega) {
			g.Expect(validateShootAccess(ctx, shoot1NoAccessTokenClientAPIServerProxy, shoot1, false)).To(Succeed())
			g.Expect(validateShootAccess(ctx, shoot2NoAccessTokenClientAPIServerProxy, shoot2, false)).To(Succeed())
		}).WithTimeout(10 * time.Second).Should(Succeed())

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
		customClient, err := viewerClientWithCustomTransport(ctx, f.GardenClient, shoot1, &injectHeaderTransport{
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

func getAPIServerProxyClient(restConfig *rest.Config, targetShoot *gardencorev1beta1.Shoot) (client.Client, error) {
	transport, err := newHTTPConnectTransport(restConfig, apiserverexposure.GetAPIServerProxyTargetClusterName(targetShoot.Status.TechnicalID))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP connect transport: %w", err)
	}

	newRestConfig := rest.CopyConfig(restConfig)
	newRestConfig.Host = "https://kubernetes.default.svc.cluster.local"
	c, err := client.New(
		newRestConfig,
		client.Options{
			HTTPClient: &http.Client{
				Transport: transport,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return c, nil
}

func httpClientForRESTConfig(restConfig *rest.Config) (string, *http.Client, error) {
	tlsConfig, err := tlsConfigForRESTConfig(restConfig, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	return restConfig.Host, &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

func viewerClientWithCustomTransport(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, transport http.RoundTripper) (kubernetes.Interface, error) {
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

func tlsConfigForRESTConfig(restConfig *rest.Config, withClientCertificates bool) (*tls.Config, error) {
	rootCAs := x509.NewCertPool()
	if restConfig.CAFile != "" {
		caCert, err := os.ReadFile(restConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		if !rootCAs.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}
	if len(restConfig.CAData) > 0 {
		if !rootCAs.AppendCertsFromPEM(restConfig.CAData) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	tlsConfig := &tls.Config{
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS12,
	}

	if withClientCertificates {
		if restConfig.CertFile != "" && restConfig.KeyFile == "" {
			cert, err := tls.LoadX509KeyPair(restConfig.CertFile, restConfig.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		}
		if len(restConfig.CertData) > 0 && len(restConfig.KeyData) > 0 {
			cert, err := tls.X509KeyPair(restConfig.CertData, restConfig.KeyData)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		}
	}

	return tlsConfig, nil
}

type httpConnectTransport struct {
	httpConnectClient *http.Client
	bearerToken       string
}

func newHTTPConnectTransport(restConfig *rest.Config, targetCluster string) (*httpConnectTransport, error) {
	proxyAddress := strings.Split(strings.TrimPrefix(restConfig.Host, "https://"), ":")[0]
	connectAddress := strings.TrimPrefix(restConfig.Host, "https://") + ":443"

	tlsConfig, err := tlsConfigForRESTConfig(restConfig, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	bearerToken := restConfig.BearerToken
	if restConfig.BearerTokenFile != "" {
		token, err := os.ReadFile(restConfig.BearerTokenFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read bearer token file: %w", err)
		}
		bearerToken = string(token)
	}

	httpConnectClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:8132", proxyAddress), 5*time.Second)
				if err != nil {
					return nil, fmt.Errorf("failed to connect to proxy: %w", err)
				}

				connectRequest := &http.Request{
					Method: http.MethodConnect,
					URL:    &url.URL{Opaque: connectAddress},
					Host:   connectAddress,
					Header: http.Header{},
				}
				connectRequest.Header.Set("Reversed-VPN", targetCluster)

				if err := connectRequest.Write(connection); err != nil {
					return nil, fmt.Errorf("failed sending HTTP CONNECT request: %w", err)
				}

				resp, err := http.ReadResponse(bufio.NewReader(connection), connectRequest)
				if err != nil {
					return nil, fmt.Errorf("failed to read HTTP CONNECT response: %w", err)
				}
				defer func() { utilruntime.HandleError(resp.Body.Close()) }()

				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("proxy returned status: %q", resp.Status)
				}

				return connection, nil
			},
			TLSClientConfig: tlsConfig,
		},
	}

	return &httpConnectTransport{httpConnectClient: httpConnectClient, bearerToken: bearerToken}, nil
}

func (h *httpConnectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if h.httpConnectClient == nil {
		return nil, fmt.Errorf("connection to proxy not established")
	}

	if h.bearerToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.bearerToken))
	}

	return h.httpConnectClient.Do(req)
}

type injectHeaderTransport struct {
	Headers map[string]string
}

func (i *injectHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range i.Headers {
		req.Header.Add(key, value)
	}
	return http.DefaultTransport.RoundTrip(req)
}
