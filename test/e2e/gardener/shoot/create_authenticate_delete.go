// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/apimachinery/pkg/watch"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/utils/access"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Authenticate, Delete", Ordered, Label("authentication"), func() {
		shoot1 := NewTestContext().ForShoot(DefaultShoot("e2e-auth-one"))
		shoot2 := NewTestContext().ForShoot(DefaultShoot("e2e-auth-two"))

		ItShouldCreateShoot(shoot1)
		ItShouldCreateShoot(shoot2)

		ItShouldWaitForShootToBeReconciledAndHealthy(shoot1)
		ItShouldGetResponsibleSeed(shoot1)
		seed.ItShouldInitializeSeedClient(&shoot1.SeedContext)

		ItShouldWaitForShootToBeReconciledAndHealthy(shoot2)
		ItShouldGetResponsibleSeed(shoot2)
		seed.ItShouldInitializeSeedClient(&shoot2.SeedContext)

		var shoot1Client, shoot2Client, shoot1TokenClient, shoot2TokenClient kubernetes.Interface

		It("should verify shoot access using admin kubeconfig", func(ctx SpecContext) {
			var err error

			shoot1Client, err = access.CreateShootClientFromAdminKubeconfig(ctx, shoot1.GardenClientSet, shoot1.Shoot)
			Expect(err).NotTo(HaveOccurred())

			shoot2Client, err = access.CreateShootClientFromAdminKubeconfig(ctx, shoot2.GardenClientSet, shoot2.Shoot)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1Client.Client(), shoot1.Shoot, true)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2Client.Client(), shoot2.Shoot, true)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify shoot access using service account token kubeconfig", func(ctx SpecContext) {
			var err error

			shoot1TokenClient, err = access.CreateTargetClientFromDynamicServiceAccountToken(ctx, shoot1Client, "shoot-one")
			Expect(err).NotTo(HaveOccurred())

			shoot2TokenClient, err = access.CreateTargetClientFromDynamicServiceAccountToken(ctx, shoot2Client, "shoot-two")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1TokenClient.Client(), shoot1.Shoot, true)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2TokenClient.Client(), shoot2.Shoot, true)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify shoot access via apiserver-proxy endpoint", func(ctx SpecContext) {
			shoot1ClientAPIServerProxy, err := getAPIServerProxyClient(shoot1Client.RESTConfig(), shoot1.Shoot)
			Expect(err).NotTo(HaveOccurred())

			shoot2ClientAPIServerProxy, err := getAPIServerProxyClient(shoot2Client.RESTConfig(), shoot2.Shoot)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1ClientAPIServerProxy, shoot1.Shoot, true)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2ClientAPIServerProxy, shoot2.Shoot, true)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify shoot access using service account token kubeconfig via apiserver-proxy endpoint", func(ctx SpecContext) {
			shoot1TokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot1TokenClient.RESTConfig(), shoot1.Shoot)
			Expect(err).NotTo(HaveOccurred())

			shoot2TokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot2TokenClient.RESTConfig(), shoot2.Shoot)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1TokenClientAPIServerProxy, shoot1.Shoot, true)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2TokenClientAPIServerProxy, shoot2.Shoot, true)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify a shoot cannot be accessed with a client certificate from another shoot", func(ctx SpecContext) {
			shoot1NoAccessRestConfig := copyRESTConfigAndInjectAuthorization(shoot1Client.RESTConfig(), shoot2Client.RESTConfig())
			shoot1NoAccessClient, err := client.New(shoot1NoAccessRestConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			shoot2NoAccessRestConfig := copyRESTConfigAndInjectAuthorization(shoot2Client.RESTConfig(), shoot1Client.RESTConfig())
			shoot2NoAccessClient, err := client.New(shoot2NoAccessRestConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			Consistently(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1NoAccessClient, shoot1.Shoot, false)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2NoAccessClient, shoot2.Shoot, false)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify a shoot cannot be accessed with a client certificate from another shoot by manipulating the apiserver-proxy header", func(ctx SpecContext) {
			shoot1NoAccessRestConfig := copyRESTConfigAndInjectAuthorization(shoot1Client.RESTConfig(), shoot2Client.RESTConfig())
			shoot1NoAccessClientAPIServerProxy, err := getAPIServerProxyClient(shoot1NoAccessRestConfig, shoot1.Shoot)
			Expect(err).NotTo(HaveOccurred())

			shoot2NoAccessRestConfig := copyRESTConfigAndInjectAuthorization(shoot2Client.RESTConfig(), shoot1Client.RESTConfig())
			shoot2NoAccessClientAPIServerProxy, err := getAPIServerProxyClient(shoot2NoAccessRestConfig, shoot2.Shoot)
			Expect(err).NotTo(HaveOccurred())

			Consistently(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1NoAccessClientAPIServerProxy, shoot1.Shoot, false)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2NoAccessClientAPIServerProxy, shoot2.Shoot, false)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("Verify a shoot cannot be accessed with a service account token from another shoot", func(ctx SpecContext) {
			shoot1NoAccessTokenRestConfig := copyRESTConfigAndInjectAuthorization(shoot1TokenClient.RESTConfig(), shoot2TokenClient.RESTConfig())
			shoot1NoAccessTokenClient, err := client.New(shoot1NoAccessTokenRestConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			shoot2NoAccessTokenRestConfig := copyRESTConfigAndInjectAuthorization(shoot2TokenClient.RESTConfig(), shoot1TokenClient.RESTConfig())
			shoot2NoAccessTokenClient, err := client.New(shoot2NoAccessTokenRestConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			Consistently(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1NoAccessTokenClient, shoot1.Shoot, false)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2NoAccessTokenClient, shoot2.Shoot, false)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify a shoot cannot be accessed with a service account token from another shoot by manipulating the apiserver-proxy header", func(ctx SpecContext) {
			shoot1NoAccessTokenRestConfig := copyRESTConfigAndInjectAuthorization(shoot1TokenClient.RESTConfig(), shoot2TokenClient.RESTConfig())
			shoot1NoAccessTokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot1NoAccessTokenRestConfig, shoot1.Shoot)
			Expect(err).NotTo(HaveOccurred())

			shoot2NoAccessTokenRestConfig := copyRESTConfigAndInjectAuthorization(shoot2TokenClient.RESTConfig(), shoot1TokenClient.RESTConfig())
			shoot2NoAccessTokenClientAPIServerProxy, err := getAPIServerProxyClient(shoot2NoAccessTokenRestConfig, shoot2.Shoot)
			Expect(err).NotTo(HaveOccurred())

			Consistently(func(g Gomega) {
				g.Expect(validateShootAccess(ctx, shoot1NoAccessTokenClientAPIServerProxy, shoot1.Shoot, false)).To(Succeed())
				g.Expect(validateShootAccess(ctx, shoot2NoAccessTokenClientAPIServerProxy, shoot2.Shoot, false)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("should verify that authentication with istio tls termination cannot be bypassed", func(ctx SpecContext) {
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
		})

		It("Verify that users cannot escalate their privileges with istio tls termination", func(ctx SpecContext) {
			customClient, err := viewerClientWithCustomTransport(ctx, shoot1.GardenClientSet, shoot1.Shoot, &injectHeaderTransport{
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

			Expect(selfSubjectReview.Status.UserInfo.Username).To(HaveSuffix(":admin-user"))
			Expect(selfSubjectReview.Status.UserInfo.Groups).To(Equal([]string{"gardener.cloud:system:viewers", "system:authenticated"}))
		})

		It("should terminate requests and watches on persistent connections once the client certificate expires", func(ctx SpecContext) {
			// The admin kubeconfig subresource enforces a 10 minute minimum validity, so sign a short-lived certificate
			// ourselves. O=gardener.cloud:system:admins grants cluster-admin via the authenticating proxy.
			certData, keyData, notAfter := signShortLivedShootAdminClientCertificate(ctx, shoot1.SeedClient, shoot1.Shoot.Status.TechnicalID, shortClientCertificateValidity)

			restConfig := rest.CopyConfig(shoot1Client.RESTConfig())
			restConfig.CertData = certData
			restConfig.KeyData = keyData
			restConfig.CertFile = ""
			restConfig.KeyFile = ""
			restConfig.BearerToken = ""
			restConfig.BearerTokenFile = ""

			directClient, err := kubernetesclientset.NewForConfig(rest.CopyConfig(restConfig))
			Expect(err).NotTo(HaveOccurred())

			proxyClient, err := getAPIServerProxyClientSet(restConfig, shoot1.Shoot)
			Expect(err).NotTo(HaveOccurred())

			verifyRequestsAndWatchesTerminateAfterCertificateExpiry(ctx, notAfter, map[string]kubernetesclientset.Interface{
				"apiserver endpoint":       directClient,
				"apiserver-proxy endpoint": proxyClient,
			})
		})

		ItShouldDeleteShoot(shoot1)
		ItShouldDeleteShoot(shoot2)

		ItShouldWaitForShootToBeDeleted(shoot1)
		ItShouldWaitForShootToBeDeleted(shoot2)
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
	transport, err := newHTTPConnectTransport(restConfig, apiserverexposure.GetAPIServerProxyTargetClusterName(targetShoot.Status.TechnicalID), vpnseedserver.HTTPProxyGatewayPort)
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

func getAPIServerProxyClientSet(restConfig *rest.Config, shoot *gardencorev1beta1.Shoot) (kubernetesclientset.Interface, error) {
	transport, err := newHTTPConnectTransport(restConfig, apiserverexposure.GetAPIServerProxyTargetClusterName(shoot.Status.TechnicalID), vpnseedserver.HTTPProxyGatewayPort)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP connect transport: %w", err)
	}

	proxyConfig := rest.CopyConfig(restConfig)
	proxyConfig.Host = "https://kubernetes.default.svc.cluster.local"

	return kubernetesclientset.NewForConfigAndClient(proxyConfig, &http.Client{Transport: transport})
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
	viewer, err := access.RequestViewerKubeconfigForShoot(ctx, gardenClient, shoot, new(int64(7200)))
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

func newHTTPConnectTransport(restConfig *rest.Config, targetCluster string, proxyPort int) (*httpConnectTransport, error) {
	proxyAddress, _, _ := strings.Cut(strings.TrimPrefix(restConfig.Host, "https://"), ":")
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
				connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", proxyAddress, proxyPort), 5*time.Second)
				if err != nil {
					return nil, fmt.Errorf("failed to connect to proxy: %w", err)
				}

				connectRequest := &http.Request{
					Method: http.MethodConnect,
					URL:    &url.URL{Opaque: connectAddress},
					Host:   connectAddress,
					Header: http.Header{},
				}
				header := "Reversed-VPN"
				if proxyPort == vpnseedserver.HTTPProxyGatewayPort {
					header = "X-Gardener-Destination"
				}
				connectRequest.Header.Add(header, targetCluster)

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

	return h.httpConnectClient.Do(req) // #nosec: G704 -- Test code with controlled URLs.
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

const shortClientCertificateValidity = 2 * time.Minute

// signShortLivedShootAdminClientCertificate reads the shoot's client CA (ca-client) from its control plane namespace on
// the seed and signs a short-lived client certificate with it. It returns the certificate and private key in PEM format
// as well as the certificate's expiration time.
func signShortLivedShootAdminClientCertificate(ctx context.Context, seedClient client.Client, controlPlaneNamespace string, validity time.Duration) ([]byte, []byte, time.Time) {
	GinkgoHelper()

	caClientSecrets := &corev1.SecretList{}
	Expect(seedClient.List(ctx, caClientSecrets, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
		secretsmanager.LabelKeyName:            v1beta1constants.SecretNameCAClient,
		secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyManagerIdentity: v1beta1constants.SecretManagerIdentityGardenlet,
	})).To(Succeed())
	Expect(caClientSecrets.Items).NotTo(BeEmpty(), "expected a ca-client secret in namespace %q", controlPlaneNamespace)

	caClientSecret := caClientSecrets.Items[0]
	for i := range caClientSecrets.Items {
		if caClientSecrets.Items[i].CreationTimestamp.After(caClientSecret.CreationTimestamp.Time) {
			caClientSecret = caClientSecrets.Items[i]
		}
	}

	clientCA, err := secretsutils.LoadCertificate("", caClientSecret.Data[secretsutils.DataKeyPrivateKeyCA], caClientSecret.Data[secretsutils.DataKeyCertificateCA])
	Expect(err).NotTo(HaveOccurred())

	certificate, err := (&secretsutils.CertificateSecretConfig{
		Name:         "e2e-client-cert-expiry",
		CommonName:   "gardener.cloud:admin:e2e-client-cert-expiry",
		Organization: []string{v1beta1constants.ShootSystemAdminsGroupName},
		CertType:     secretsutils.ClientCert,
		Validity:     &validity,
		SigningCA:    clientCA,
	}).GenerateCertificate()
	Expect(err).NotTo(HaveOccurred())

	return certificate.CertificatePEM, certificate.PrivateKeyPEM, certificate.Certificate.NotAfter
}

// verifyRequestsAndWatchesTerminateAfterCertificateExpiry asserts that, for every given persistent client, list requests
// and a kept-open watch keep working until the client certificate expires (notAfter) and are then rejected with an
// unauthorized error - only after expiry. List polling and watch maintenance run concurrently per client (within a
// single validity window) so each client's success->rejection transition is observed independently.
func verifyRequestsAndWatchesTerminateAfterCertificateExpiry(ctx context.Context, notAfter time.Time, clients map[string]kubernetesclientset.Interface) {
	GinkgoHelper()

	// Slack for the sub-second difference between the test's and Envoy's clocks and the route timeout timer precision.
	const clockSlack = 2 * time.Second

	probes := make(map[string]*connectionProbe, len(clients))
	for name, shootClient := range clients {
		By(fmt.Sprintf("opening a watch and establishing the persistent connection via the %s", name))

		namespaceWatch, err := shootClient.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		probe := &connectionProbe{
			watch:    &watchMaintainer{done: make(chan struct{})},
			listDone: make(chan struct{}),
		}
		probes[name] = probe
		go probe.watch.run(ctx, shootClient, namespaceWatch)

		Eventually(ctx, func(g Gomega) {
			_, err := shootClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
			g.Expect(err).NotTo(HaveOccurred())
		}).WithTimeout(time.Minute).Should(Succeed())

		go probe.pollListUntilRejected(ctx, shootClient)
	}

	for name, probe := range probes {
		By(fmt.Sprintf("verifying that requests and watches are rejected with an unauthorized error only after the certificate expired via the %s", name))

		Eventually(ctx, isClosed(probe.listDone)).WithTimeout(time.Until(notAfter) + time.Minute).WithPolling(time.Second).Should(BeTrue())
		Expect(probe.listRejectedAt).To(BeTemporally(">=", notAfter.Add(-clockSlack)), "requests were rejected before the certificate expired")

		Eventually(ctx, isClosed(probe.watch.done)).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeTrue())
		Expect(probe.watch.rejectedAt).To(BeTemporally(">=", notAfter.Add(-clockSlack)),
			"the watch was rejected with an unauthorized error before the certificate expired (rejected at %v, certificate expired at %v)", probe.watch.rejectedAt, notAfter)
	}
}

func isClosed(ch <-chan struct{}) func() bool {
	return func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
}

// connectionProbe tracks, per persistent client, when list requests and the kept-open watch are first rejected with an
// unauthorized error.
type connectionProbe struct {
	watch *watchMaintainer

	listDone       chan struct{}
	listRejectedAt time.Time
}

// pollListUntilRejected polls list requests until one is rejected with an unauthorized error, recording the time.
func (p *connectionProbe) pollListUntilRejected(ctx context.Context, shootClient kubernetesclientset.Interface) {
	defer close(p.listDone)

	for ctx.Err() == nil {
		if _, err := shootClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1}); apierrors.IsUnauthorized(err) {
			p.listRejectedAt = time.Now()
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// watchMaintainer keeps a watch alive, re-establishing it on transient errors, until it is rejected with an unauthorized error.
type watchMaintainer struct {
	done       chan struct{}
	rejectedAt time.Time
}

// run drains the given watch and re-establishes it whenever it ends, retrying transient errors. It returns (closing
// done) once re-establishing fails with an unauthorized error or the context is canceled.
func (m *watchMaintainer) run(ctx context.Context, shootClient kubernetesclientset.Interface, namespaceWatch watch.Interface) {
	defer close(m.done)

	for {
		for range namespaceWatch.ResultChan() {
		}
		namespaceWatch.Stop()

		for {
			if ctx.Err() != nil {
				return
			}

			var err error
			namespaceWatch, err = shootClient.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{})
			if err == nil {
				break
			}
			if apierrors.IsUnauthorized(err) {
				m.rejectedAt = time.Now()
				return
			}

			// Transient error (e.g. a freshly dialed connection failing its handshake); retry - the persistent
			// connection yields the unauthorized error.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}
