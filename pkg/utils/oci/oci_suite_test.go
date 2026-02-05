// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	"github.com/distribution/distribution/v3/registry/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	helmregistry "helm.sh/helm/v3/pkg/registry"

	netutils "github.com/gardener/gardener/pkg/utils/net"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

func TestOCI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils OCI Suite")
}

var (
	registryAddress       string
	exampleChartDigest    string
	rawChart              []byte
	authProvider          *testAuthProvider
	testCACert, testCAKey []byte
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithCancel(context.Background())
	DeferCleanup(cancel)

	ca, err := (&secretsutils.CertificateSecretConfig{
		Name:        "test-ca",
		CommonName:  "TestCA",
		CertType:    secretsutils.CACert,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}).GenerateCertificate()
	Expect(err).NotTo(HaveOccurred())

	caData := ca.SecretData()
	testCACert = caData[secretsutils.DataKeyCertificateCA]
	testCAKey = caData[secretsutils.DataKeyPrivateKeyCA]

	certDir := GinkgoT().TempDir()

	err = os.WriteFile(filepath.Join(certDir, "tls.crt"), testCACert, 0600)
	Expect(err).NotTo(HaveOccurred())
	err = os.WriteFile(filepath.Join(certDir, "tls.key"), testCAKey, 0600)
	Expect(err).NotTo(HaveOccurred())

	certPool := x509.NewCertPool()
	Expect(certPool.AppendCertsFromPEM(testCACert)).To(BeTrue())

	c, err := helmregistry.NewClient(helmregistry.ClientOptHTTPClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    certPool,
				MinVersion: tls.VersionTLS12,
			},
		},
	}))
	Expect(err).NotTo(HaveOccurred())

	registryAddress, err = startTestRegistry(ctx, certDir)
	Expect(err).NotTo(HaveOccurred())
	rawChart, err = os.ReadFile("./testdata/example-0.1.0.tgz")
	Expect(err).NotTo(HaveOccurred())
	res, err := c.Push(rawChart, fmt.Sprintf("%s/charts/example:0.1.0", registryAddress))
	Expect(err).NotTo(HaveOccurred())
	exampleChartDigest = res.Manifest.Digest
})

func startTestRegistry(ctx context.Context, certDir string) (string, error) {
	config := &configuration.Configuration{}
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]any{}}

	port, host, err := netutils.SuggestPort("")
	if err != nil {
		return "", err
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = 3 * time.Second

	// Configure TLS
	config.HTTP.TLS.Certificate = filepath.Join(certDir, "tls.crt")
	config.HTTP.TLS.Key = filepath.Join(certDir, "tls.key")

	// setup logger options
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"

	// register a test auth provider
	authProvider = &testAuthProvider{}
	config.Auth = configuration.Auth{"oci-suite-test": map[string]any{}}
	if err := auth.Register("oci-suite-test", func(_ map[string]any) (auth.AccessController, error) {
		return authProvider, nil
	}); err != nil {
		return "", err
	}

	reg, err := registry.NewRegistry(ctx, config)
	if err != nil {
		return "", err
	}
	go func() {
		_ = reg.ListenAndServe()
	}()
	return addr, nil
}

type testAuthProvider struct {
	receivedAuthorization string
}

var _ auth.AccessController = &testAuthProvider{}

func (a *testAuthProvider) Authorized(r *http.Request, _ ...auth.Access) (*auth.Grant, error) {
	if r.Method == "GET" && strings.Contains(r.URL.Path, "/blobs/") {
		a.receivedAuthorization = r.Header.Get("Authorization")
	}
	return &auth.Grant{User: auth.UserInfo{Name: "dummy"}}, nil
}
