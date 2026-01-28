// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
)

func TestOCI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils OCI Suite")
}

var (
	registryAddress    string
	exampleChartDigest string
	rawChart           []byte
	authProvider       *testAuthProvider
	registryCABundle   []byte
	certDir            string
	err                error
)

const (
	// spellchecker:off
	testCACert = `-----BEGIN CERTIFICATE-----
MIIFHTCCAwWgAwIBAgIUZ1o0RFU8zwU6ZePlKMaR3RW5Kd8wDQYJKoZIhvcNAQEL
BQAwETEPMA0GA1UEAwwGVGVzdENBMB4XDTI2MDEyMjA5MDkwOVoXDTM2MDEyMDA5
MDkwOVowETEPMA0GA1UEAwwGVGVzdENBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8A
MIICCgKCAgEA1gW3zLIeATCcpL9cVmVeX+mgch3Fhh3t6DxLKQNQdMDg68u8r0Nw
5dDV+ZmWl9OiTZrn2k29XlJsfCzM1MkkzpENTlbHg4R8cYe9dHrRI7S2iTf89QMx
FS+FFlXujqwgWqfTBdkDhMZ+ZlJ3L6MeeUjTi3x225TEhv1SCDeQtLFdSKwLSPlK
FP3fU89zjYXvGdmb6+w9v60vzGZZBGXOJj3dd1InS6Utx0SnFgBr47wWOHJXXz3M
zzsKNf05e9MkZVyH5jhlK5H6WuuMOmwvT6/kZKnh3dCtvLlcVODdlxPWBd8tPff5
IvE47auIfBzwXn7E/hXHZuHCOk+j7cd3hvnkUUbEtHl5ML4CEyYGCYGspOIqPVJW
LkTTYZ10ikbsNIRAFJpPAdSwgnhoHXZJ0AhXmxwJHoFp2Bx2Rrn8F/dOEWsKTgCj
mud9XVUCuUJ7cz/rLKqyB0/ju6Y7je0ypJjZBCdrESKMHYoXc6xEPnOQRzkJBqbo
gjL/d9AmDYd9uoK8GqjM8OgZSkpjm3ZIbNRrchcZYUHsMIlOhh+17uxJTjh9srV4
2lTsrMZLdSnEwz+vjDly6yKNtk7jfE4T1CW3RMp1jq/wnY7n9zEkjLVtWag+zMzs
vBDvxDhSFXSmU/dKz8QjUJ9IJ/eU1eyFWxwIxlW/b7Adr7vYJ+HM9B8CAwEAAaNt
MGswGgYDVR0RBBMwEYIJbG9jYWxob3N0hwR/AAABMAwGA1UdEwQFMAMBAf8wCwYD
VR0PBAQDAgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMB0GA1UdDgQWBBSuRBo/TRU+
DBroW4/u+TGGyyehEjANBgkqhkiG9w0BAQsFAAOCAgEAYTTBFwmZArt1jgBB5s9W
C+rYZm7VO98u39IyNMnGq7lbm3WnvGc3PF/tlv2gPq6DObla/2At4jjTHia9bPJJ
u1gzGW+zk/rcOUtg4bbyUs+gCAeswhuFBfLzjj5GvVuUfxeQ0jAYtnoaa4bZhUbD
8NGq6CE4s+bUwEaiuy9XeUuf6nfo9rpOqJfPbiEIWMd8JaWw82FA7r/he36t9NMw
erh5nlpX/kMSeldfyrdyqafzJrU+pvbfki1abomhW8ekW97GuO9MJxyRkQAYJaxk
9lMzRaK1uDVoYPbAADDYNElaaikgYTYDwguuR08m5h4RDEMSsOWqUb5FXVYZaXmA
jY5TCJCcpH4iPXDLVSf7Qkh0tmdFazD3eWbPeXBL3kJ3dDpIg9p/8b1xcd8hRiQk
uH7hJlr20SU/PdLE+qTWXKYcVvA5D65NgooaS1iFnDMAlVDjke1VMSeWQCpXmjJ3
pF9cMYb2THAkN2vzU+fTTNI3+qqxPVaoh8YUPL6Ioogg6ZwaTrlwO+EBrQtv7gDj
8Sm3d7CyEeVXHpQzbs/5y8n+PCV5b7YmKAbg52vLtNBwhtHyeesp+9Vp+lKZf+F3
w/JIOrM8IRNQHo2ukVTImVnvusN6g/dkr+oJqju5dulZXoRfOShl/4Y/borkD6am
H3vIfd2MUfoFLntH/rCuY84=
-----END CERTIFICATE-----`

	testCAKey = `-----BEGIN PRIVATE KEY-----
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
OZrazvpOeloQYN/zp4qgz/kHRst8lT0=
-----END PRIVATE KEY-----`
	// spellchecker:on
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithCancel(context.Background())
	DeferCleanup(cancel)

	certDir = GinkgoT().TempDir()
	registryCABundle = []byte(testCACert)

	err = os.WriteFile(filepath.Join(certDir, "tls.crt"), []byte(testCACert), 0600)
	Expect(err).NotTo(HaveOccurred())
	err = os.WriteFile(filepath.Join(certDir, "tls.key"), []byte(testCAKey), 0600)
	Expect(err).NotTo(HaveOccurred())

	certPool := x509.NewCertPool()
	Expect(certPool.AppendCertsFromPEM(registryCABundle)).To(BeTrue())

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
