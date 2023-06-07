/*
Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	testclock "k8s.io/utils/clock/testing"

	authenticationapi "github.com/gardener/gardener/pkg/apis/authentication"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Admin Kubeconfig", func() {
	var (
		ctx context.Context
		obj *authenticationapi.AdminKubeconfigRequest

		shoot           *gardencore.Shoot
		shootState      *gardencore.ShootState
		caClusterSecret *corev1.Secret
		caClientSecret  *gardencore.InternalSecret

		akcREST          *AdminKubeconfigREST
		createValidation registryrest.ValidateObjectFunc

		shootStateGetter     *fakeGetter
		shootGetter          *fakeGetter
		secretLister         *fakeSecretLister
		internalSecretLister *fakeInternalSecretLister

		clusterCACert1 = []byte("cluster-ca-cert1")
		clusterCACert2 = []byte("cluster-ca-cert2")

		clientCACert1Name = "minikubeCA"
		clientCACert1     = []byte(`-----BEGIN CERTIFICATE-----
MIIDBjCCAe6gAwIBAgIBATANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwptaW5p
a3ViZUNBMB4XDTIxMDMyNTE0MjczN1oXDTMxMDMyNDE0MjczN1owFTETMBEGA1UE
AxMKbWluaWt1YmVDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALsW
8jU6AUP1t9Wp6xOTAYhjrEPGixP+iCj9cSX5XkShpVNYNemwCqpDNOetKAAtFQMk
pco1isfuB876bNY+/bC5YCrYprzljS+EYAb+/eD/ahURnXXy09yfBrGTMvr6ti8B
L5DqlDqhHu3sekIMSedrcCs10dDckgl4lghoRSoCad3/LLqOYTPDD7VLKJup4JgS
3J1s6AxvBeeRAh94avTP+4MP4PBIewrq0CODA+rf9xfGlOrRYU5ZJnIPFCM6uEIA
xpYJl9tKuyN23kZ1BJtlenHYiR4fouXE05S0U5pw+z3WvOyNRsVQ2BViZOsVnmD6
wVrPBuZRG2NMCfEzjAECAwEAAaNhMF8wDgYDVR0PAQH/BAQDAgKkMB0GA1UdJQQW
MBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW
BBQwmHrSlJ/ytlShbhPeeMmKGnsneDANBgkqhkiG9w0BAQsFAAOCAQEABeF0WNol
mSS/hnbMFIfI8Fe90uefiO3hryBUJVBb9eaDXRRjCh9Dhj5pwxUBRyKbPHFQLQMe
YWq2Vg6vWEjDEISnthcK6n5oPIwzV5zNWek7sW3DSzFdYru8KDQReVnzBdMNIDZI
OnM7+5534rkP8/eIX58QFcVibjM34BfqNQgHW5vFXobYoIX2wfMysLZVESYQdU9P
14S7fj3Ui4IrBqElF30QUmAe6bgjBu+xsZHFaImJ+yJXuPjPEuIWoKMoiH9fDrJ0
C3KRaS8COePkaiH/NUjuIjyTXzhvJqmFbH730vABpcKi01eQMMjtRkPlWIEqUHoG
QbU6uberp2QAQA==
-----END CERTIFICATE-----`)
		clientCAKey1 = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAuxbyNToBQ/W31anrE5MBiGOsQ8aLE/6IKP1xJfleRKGlU1g1
6bAKqkM0560oAC0VAySlyjWKx+4Hzvps1j79sLlgKtimvOWNL4RgBv794P9qFRGd
dfLT3J8GsZMy+vq2LwEvkOqUOqEe7ex6QgxJ52twKzXR0NySCXiWCGhFKgJp3f8s
uo5hM8MPtUsom6ngmBLcnWzoDG8F55ECH3hq9M/7gw/g8Eh7CurQI4MD6t/3F8aU
6tFhTlkmcg8UIzq4QgDGlgmX20q7I3beRnUEm2V6cdiJHh+i5cTTlLRTmnD7Pda8
7I1GxVDYFWJk6xWeYPrBWs8G5lEbY0wJ8TOMAQIDAQABAoIBAHZMrBq78tDmLrgM
GXjnG7ECVYsFoCukZrSEjWdVpyX+kGuC+5QonJXMqUdVVlXGK+Mw6SRTds201Xsr
Hmbarc9xaD2vgL8w53WEXrQNyLrcxldMLCTIxu5aIAFo8nOA1HIkbc9UhSYNe2E2
hpf87T5H0UWBYoqO7kjO1w+53wIQL8gSCysHfO/72LwHhob1E89lyUN4bemr++eU
IgwuUxvCdiKr3in5nvbRwhLNO+K7TipKZgIj5J0SUqtiLZZ4QLNvnGbGzgoyRzoU
OgQ02qAZ8oJW0P9xal9OhWWSVRESo6D+HWMJM6Y3GdPt36oFqSnrpDh9n9L9Bf0R
SS0VXYECgYEA4DAwNPlPdiNbg8GHBouBTWW2dBGhhWvyWzZOs7Q97JW/Cs1B1ruM
42+1/ZNyNdr+buWqhDGr1QtM4UEK1nBkRHuV6kqZw8z/hKhC3r0D2AhP01yI4sGF
Bm3QFlmQJTYz9wOPFJDINkgCG2KH60p+PXBIeULA5MtYEC6hMZNe1mMCgYEA1aMf
Tlu4DIZ3Trh1ow+XtJPbwwcjcdXmMfwU+jQr3pSz6ySxXuCSBgJ9z8RbcELwDmNg
9MW8u+XMH6VSw8X6Fv1Fy7+npObz7UMW0Ij0cW/FFJ9vKOSYYET/YpFh5D0/QsWi
zLmg8iYQEjo4DlXVh8mfz0ishm0H6dVwGDp0X0sCgYAF5379hitfkyLP34Ls2zO2
lB0wBV7ZorQpTs7X0MFov7DeWfWH8DyPqNuEKCPz4yacSRQqkxxRahDGRe5BI4ig
fRi/qONP0tBP8BaCwzucrutbR66bOjmEp9O5Iva25CyOLtvP0NhVBaR4kCnAOqAE
gjaGawmlfO1+z5uTMKxovQKBgQDNJGVEZhhWlqxr//6eBLQFJ1IIdYtYnS/9YXV3
SK+zfRFDQ6m6VGSDttK+tmujYfOHrXAFuvbfautWm/bcnPfoKW5jFvdRBqDGfPyk
ZE5tuwkBI5OnLdMP5lFhgf8BHrrnUEZi1gExZNFb32HCijOPv1FgxwU70+icZmLM
MR1b/wKBgHyhTEIz3YDAG7O/y3U6JWGnxqlr8i8+FobZWMbVSGDtgRZpDcDGyhFb
AIOz/jD6sCJ6KPr1L6mJ5w4mDX1UmjCKy3Kz4xfqxPEbMvPDTL+9TWFSlAuNtHGC
lIwEl8tStnO9u1JUK4w1e+lC37zI2v5k4WMQmJcolUEMwmZjnCR/
-----END RSA PRIVATE KEY-----`)

		clientCACert2Name = "new-minikube-ca"
		clientCACert2     = []byte(`-----BEGIN CERTIFICATE-----
MIIDBDCCAeygAwIBAgIUXutuW//tcCBAR2BKjz1N9xNosNwwDQYJKoZIhvcNAQEL
BQAwGjEYMBYGA1UEAxMPbmV3LW1pbmlrdWJlLWNhMB4XDTIyMDQxMjA3MDcwMFoX
DTI3MDQxMTA3MDcwMFowGjEYMBYGA1UEAxMPbmV3LW1pbmlrdWJlLWNhMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsiOi5NGvtPtJLD4FfMUne1KcgtKs
o91WdriOJWF6mfWiB2fnbMS8EaKaU4AMXyrQpn6neZTDXeH5DOXhiQqvczRr5B4u
/SD+OXLhdrzNaIpYc7DNhbT41DdG0F+ZiNQao0rQJrvw7pcR6D+CzqMmLk34Q4VU
h0e2nXSqNS4S/0coKUomL1eMSHpMqJVGTQhlWHDU7xMyOZ2t5TleHBI+OhfXAMyV
iEcaZUeengV73RoX+ycAYb5tjZOwk0GlolQxYl4rnjro2c2i5ezK8F+xdfkbCL/D
NfUGg01JBfCc1Yb3DtOpTaQcnnwFyJjQX8aPMTtDbT/4JZsHujZdRKU9yQIDAQAB
o0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQU
+8T3UU5pFUA8jFwy4pLioRxg1IgwDQYJKoZIhvcNAQELBQADggEBAELa5wEx7CAX
y98v2iDAQ4uXNIrVZFp3zAgL1Rvtivf85Vz6+aQMSflJG8Ftk205PbUPhcvMrdFi
NdC9mGZ1K+QoyAIP+cU6zDVhzH73Vjlg/JyMa8kRFaqJMAnExGKNuzu4Ox742XKH
ej+WbykRbnB3t/Fvw4WrA0ZhQip/314SOyF71xDHGfBQrJYItGEB7kIriTOUL0Or
Eh1pkuxLBmO/iz4iAMaaG5JuVlPtDEYLX1kBx/aPh9sjgw28AWvlA1L/HawmXLsR
Yg+zBuGRGSu1IfIIwjshOGmsz+jaTM0SEZ5AtbmOl1gvGSgj8Ylod+Qb7gXBxBO8
yUsW6+ksN+o=
-----END CERTIFICATE-----`)
		clientCAKey2 = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAsiOi5NGvtPtJLD4FfMUne1KcgtKso91WdriOJWF6mfWiB2fn
bMS8EaKaU4AMXyrQpn6neZTDXeH5DOXhiQqvczRr5B4u/SD+OXLhdrzNaIpYc7DN
hbT41DdG0F+ZiNQao0rQJrvw7pcR6D+CzqMmLk34Q4VUh0e2nXSqNS4S/0coKUom
L1eMSHpMqJVGTQhlWHDU7xMyOZ2t5TleHBI+OhfXAMyViEcaZUeengV73RoX+ycA
Yb5tjZOwk0GlolQxYl4rnjro2c2i5ezK8F+xdfkbCL/DNfUGg01JBfCc1Yb3DtOp
TaQcnnwFyJjQX8aPMTtDbT/4JZsHujZdRKU9yQIDAQABAoIBAQCacqVHyLmTq478
qeVuES2zEaQbFPeTt1LA6jBsHoECvWI3E5IlzsjUbWtqXAnd9SwkPomLszxTyJl6
4lDR1Y7azqeAh97rntBsFLuAjB93tQMNg0wd0hMvQ6HFBi4C4QsbasDf5HD3G8nt
2CrcZ72xxe4q9I2eIMIm8ECmjQTxiFiVf89TRz5Y+63IniId9Gh7WKmDR59sS31I
aEVRVRS9934tdkKx3TJd4Hmb1SNusvnx8wiTfi12nVjgVtYLLzPkd18I58wNvyj/
BE4iyiM4AqzQBqgEjc8Hw6YeR3Mwu6zyA0u7g3pXHhO4JL/eOpxWY6DAVlOt+WWC
ZkhGxs0lAoGBAN8GgPKVOX9x75CPv44ATfbZ5g7qmT5wrhHIlcF/1B1Q0xvZsrmn
2Hax96EINk93osWaiKAWoVIt0mHuoE2k5TK1cazI+DatyuXgU+3ngxoI7SPK95w2
EcXTKkFGgz5/WU2XWgYRdDy2gzb3XTlygPael+pjWYb5bRQjw6hALwQHAoGBAMx6
MWcX9FmeHvjBjXRyxz4xehqv8iXnMKIghAfCTD0zQ4OTGher5mVVCcWncKB8s/c7
5mIaKfTaoGgfVeGlrBeGLSeDWoHQMdWP1ZBMchNKpzZ9OXU2QmYrkUzFPJGTUSJe
sKLLYD2R+vwWGra508rJBKQKMnmIf7MLacB6lVuvAoGAJ/HoQoqLo9HqUIAOlQZk
8GOSmvVVwSM5aiH9AI0+lomVZhWVtz7ivE+fxI3N/Gm3E6Fb+yBSgH+IgNXWjFGO
Y4iv9XyBSHnUL1wAbEnc51rV7mU5+BaPFFl/5fUVKKpyej0zeIbDxOQDmGKxpcpm
YsWA/BATRuOBr+u/7XChex0CgYBdGG0RsPhRLQqQ2x6aG//WsxQSvnSTCTU9O2yh
U7b+Ti644uqISH13OUZftSI0D1Koh58Wny7nCfrqLQoe2B0IANDiIo28eJuXzgq/
ze5KFj0XM+BLG08T0VYwC8TNyrKv4UiudcX1glcxGqdC9kwVEXyJaxMb/ieVzuZw
+d6yhQKBgQCyR66MFetyEffnbxHng3WIG4MzJj7Bn5IewQiKe6yWgrS4xMxoUywx
xdiQxdLMPqh48D9u+bwt+roq66lt1kcF0mvIUgEYXhaPj/9moG8cfgmbmF9tsm08
bW4nbZLxXHQ4e+OOPeBUXUP9V0QcE4XixdvQuslfVxjn0Ja82gdzeA==
-----END RSA PRIVATE KEY-----`)
	)

	const (
		name      = "test"
		userName  = "foo"
		namespace = "baz"
	)

	BeforeEach(func() {
		shootState = &gardencore.ShootState{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: gardencore.ShootStateSpec{
				Gardener: []gardencore.GardenerResourceData{
					{
						Name: "ca-hugo",
						Type: "secret",
						Labels: map[string]string{
							"name":             "ca",
							"managed-by":       "secrets-manager",
							"manager-identity": "gardenlet",
							"issued-at-time":   "0",
						},
						Data: runtime.RawExtension{
							Raw: []byte(`{"ca.crt":"` + utils.EncodeBase64(clusterCACert1) + `"}`),
						},
					},
					{
						Name: "ca-client-foo",
						Type: "secret",
						Labels: map[string]string{
							"name":             "ca-client",
							"managed-by":       "secrets-manager",
							"manager-identity": "gardenlet",
							"issued-at-time":   "0",
						},
						Data: runtime.RawExtension{
							Raw: []byte(`{"ca.crt":"` + utils.EncodeBase64(clientCACert1) + `","ca.key":"` + utils.EncodeBase64(clientCAKey1) + `"}`),
						},
					},
				},
			},
		}

		caClusterSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name + ".ca-cluster", Namespace: namespace},
			Data: map[string][]byte{
				"ca.crt": clusterCACert1,
			},
		}
		caClientSecret = &gardencore.InternalSecret{
			ObjectMeta: metav1.ObjectMeta{Name: name + ".ca-client", Namespace: namespace},
			Data: map[string][]byte{
				"ca.crt": clientCACert1,
				"ca.key": clientCAKey1,
			},
		}

		createValidation = func(ctx context.Context, obj runtime.Object) error { return nil }
		shoot = &gardencore.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Status: gardencore.ShootStatus{
				AdvertisedAddresses: []gardencore.ShootAdvertisedAddress{
					{
						Name: "external",
						URL:  "https://foo.bar.external:9443",
					},
					{
						Name: "internal",
						URL:  "https://foo.bar.internal:9443",
					},
				},
			},
		}

		shootGetter = &fakeGetter{obj: shoot}
		shootStateGetter = &fakeGetter{obj: shootState}

		secretLister = &fakeSecretLister{obj: caClusterSecret}
		internalSecretLister = &fakeInternalSecretLister{obj: caClientSecret}

		obj = &authenticationapi.AdminKubeconfigRequest{
			Spec: authenticationapi.AdminKubeconfigRequestSpec{
				ExpirationSeconds: int64(time.Minute.Seconds() * 11),
			},
		}

		akcREST = &AdminKubeconfigREST{
			secretLister:         secretLister,
			internalSecretLister: internalSecretLister,
			shootStorage:         shootGetter,
			shootStateStorage:    shootStateGetter,
		}

		ctx = request.WithUser(context.Background(), &user.DefaultInfo{
			Name: userName,
		})

		DeferCleanup(test.WithVar(&secretsutils.Clock, testclock.NewFakeClock(time.Unix(10, 0))))
	})

	Context("request fails", func() {
		var (
			actual runtime.Object
			err    error
		)

		AfterEach(func() {
			actual, err = akcREST.Create(ctx, name, obj, createValidation, nil)

			Expect(err).To(HaveOccurred())
			Expect(actual).To(BeNil())
		})

		Context("retrieving CAs from secrets", func() {
			It("returns an error if create validation fails", func() {
				createValidation = func(ctx context.Context, obj runtime.Object) error {
					return errors.New("some error")
				}
			})

			It("returns an error if validation fails", func() {
				obj.Spec.ExpirationSeconds = -1
			})

			It("returns an error if there is no user in the context", func() {
				ctx = context.TODO()
			})

			It("returns an error if it cannot get the ca-client secret", func() {
				internalSecretLister.err = errors.New("fake")
			})

			It("returns an error if the ca-client secret is missing the public key", func() {
				delete(caClientSecret.Data, "ca.crt")
			})

			It("returns an error if the ca-client secret is missing the private key", func() {
				delete(caClientSecret.Data, "ca.key")
			})

			It("returns an error if it cannot get the ca-cluster secret", func() {
				secretLister.err = errors.New("fake")
			})

			It("returns an error if the ca-cluster secret is missing the public key", func() {
				delete(caClusterSecret.Data, "ca.crt")
			})

			It("returns an error if the ca-cluster secret doesn't exist", func() {
				secretLister.err = apierrors.NewNotFound(gardencore.Resource("internalsecrets"), caClusterSecret.Name)
			})

			It("returns an error if it cannot get the shoot", func() {
				shootGetter.err = errors.New("can't get shoot")
			})

			It("returns an error if it cannot convert the object to a shoot", func() {
				shootGetter.obj = &corev1.Pod{}
			})

			It("returns an error if there are no advertised addresses in shoot status", func() {
				shoot.Status.AdvertisedAddresses = nil
			})
		})

		Context("falling back to the shoot state", func() {
			BeforeEach(func() {
				internalSecretLister.err = apierrors.NewNotFound(gardencore.Resource("internalsecrets"), caClientSecret.Name)
			})

			It("returns an error if it cannot get the shoot state", func() {
				shootStateGetter.err = errors.New("can't get shoot state")
			})

			It("returns an error if it cannot convert the object to a shoot state", func() {
				shootStateGetter.obj = &corev1.Pod{}
			})

			It("returns an error if the cluster certificate authority is not yet provisioned", func() {
				shootState.Spec.Gardener = append(shootState.Spec.Gardener[:0], shootState.Spec.Gardener[1:]...)
			})

			It("returns an error if the cluster certificate authority contains no certificate", func() {
				shootState.Spec.Gardener[0].Data.Raw = []byte("{}")
			})

			It("returns an error if the client certificate authority is not yet provisioned", func() {
				shootState.Spec.Gardener = append(shootState.Spec.Gardener[:1], shootState.Spec.Gardener[2:]...)
			})

			It("returns an error if the client certificate authority contains no certificate", func() {
				shootState.Spec.Gardener[1].Data.Raw = []byte("{}")
			})

			It("returns an error if the issued-at-time label on a CA cert secret is missing", func() {
				shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencore.GardenerResourceData{
					Name: "ca-2",
					Type: "secret",
					Labels: map[string]string{
						"name":             "ca",
						"managed-by":       "secrets-manager",
						"manager-identity": "gardenlet",
					},
				})
			})
		})
	})

	Context("request succeeds", func() {
		Context("retrieving CAs from secrets", func() {
			It("should successfully issue admin kubeconfig", func() {
				actual, err := akcREST.Create(ctx, name, obj, nil, nil)

				Expect(err).ToNot(HaveOccurred())
				Expect(actual).ToNot(BeNil())
				Expect(actual).To(BeAssignableToTypeOf(&authenticationapi.AdminKubeconfigRequest{}))

				akcr := actual.(*authenticationapi.AdminKubeconfigRequest)

				Expect(akcr.Status.ExpirationTimestamp.Time).To(Equal(time.Unix(10, 0).Add(time.Minute * 11)))

				config := &clientcmdv1.Config{}
				Expect(runtime.DecodeInto(clientcmdlatest.Codec, akcr.Status.Kubeconfig, config)).To(Succeed())

				Expect(config.Clusters).To(ConsistOf(
					clientcmdv1.NamedCluster{
						Name: "baz--test-external",
						Cluster: clientcmdv1.Cluster{
							Server:                   "https://foo.bar.external:9443",
							CertificateAuthorityData: clusterCACert1,
						},
					},
					clientcmdv1.NamedCluster{
						Name: "baz--test-internal",
						Cluster: clientcmdv1.Cluster{
							Server:                   "https://foo.bar.internal:9443",
							CertificateAuthorityData: clusterCACert1,
						},
					},
				))

				Expect(config.Contexts).To(ConsistOf(
					clientcmdv1.NamedContext{
						Name: "baz--test-external",
						Context: clientcmdv1.Context{
							Cluster:  "baz--test-external",
							AuthInfo: "baz--test-external",
						},
					},
					clientcmdv1.NamedContext{
						Name: "baz--test-internal",
						Context: clientcmdv1.Context{
							Cluster:  "baz--test-internal",
							AuthInfo: "baz--test-external",
						},
					},
				))
				Expect(config.CurrentContext).To(Equal("baz--test-external"))

				Expect(config.AuthInfos).To(HaveLen(1))
				Expect(config.AuthInfos[0].Name).To(Equal("baz--test-external"))
				Expect(config.AuthInfos[0].AuthInfo.ClientCertificateData).ToNot(BeEmpty())
				Expect(config.AuthInfos[0].AuthInfo.ClientKeyData).ToNot(BeEmpty())

				certPem, _ := pem.Decode(config.AuthInfos[0].AuthInfo.ClientCertificateData)
				cert, err := x509.ParseCertificate(certPem.Bytes)
				Expect(err).ToNot(HaveOccurred())

				Expect(cert.Subject.CommonName).To(Equal(userName))
				Expect(cert.Subject.Organization).To(ConsistOf("system:masters"))
				Expect(cert.NotAfter.Unix()).To(Equal(akcr.Status.ExpirationTimestamp.Time.Unix())) // certificates do not have nano seconds in them
				Expect(cert.NotBefore.UTC()).To(Equal(time.Unix(10, 0).UTC()))
				Expect(cert.Issuer.CommonName).To(Equal(clientCACert1Name))
			})
		})

		Context("falling back to the shoot state", func() {
			BeforeEach(func() {
				internalSecretLister.err = apierrors.NewNotFound(gardencore.Resource("internalsecrets"), caClientSecret.Name)
			})

			DescribeTable("request succeeds",
				func(expectedIssuerName string, expectedClusterCABundle []byte, prepEnv func()) {
					if prepEnv != nil {
						prepEnv()
					}

					actual, err := akcREST.Create(ctx, name, obj, nil, nil)

					Expect(err).ToNot(HaveOccurred())
					Expect(actual).ToNot(BeNil())
					Expect(actual).To(BeAssignableToTypeOf(&authenticationapi.AdminKubeconfigRequest{}))

					akcr := actual.(*authenticationapi.AdminKubeconfigRequest)

					Expect(akcr.Status.ExpirationTimestamp.Time).To(Equal(time.Unix(10, 0).Add(time.Minute * 11)))

					config := &clientcmdv1.Config{}
					Expect(runtime.DecodeInto(clientcmdlatest.Codec, akcr.Status.Kubeconfig, config)).To(Succeed())

					Expect(config.Clusters).To(ConsistOf(
						clientcmdv1.NamedCluster{
							Name: "baz--test-external",
							Cluster: clientcmdv1.Cluster{
								Server:                   "https://foo.bar.external:9443",
								CertificateAuthorityData: expectedClusterCABundle,
							},
						},
						clientcmdv1.NamedCluster{
							Name: "baz--test-internal",
							Cluster: clientcmdv1.Cluster{
								Server:                   "https://foo.bar.internal:9443",
								CertificateAuthorityData: expectedClusterCABundle,
							},
						},
					))

					Expect(config.Contexts).To(ConsistOf(
						clientcmdv1.NamedContext{
							Name: "baz--test-external",
							Context: clientcmdv1.Context{
								Cluster:  "baz--test-external",
								AuthInfo: "baz--test-external",
							},
						},
						clientcmdv1.NamedContext{
							Name: "baz--test-internal",
							Context: clientcmdv1.Context{
								Cluster:  "baz--test-internal",
								AuthInfo: "baz--test-external",
							},
						},
					))
					Expect(config.CurrentContext).To(Equal("baz--test-external"))

					Expect(config.AuthInfos).To(HaveLen(1))
					Expect(config.AuthInfos[0].Name).To(Equal("baz--test-external"))
					Expect(config.AuthInfos[0].AuthInfo.ClientCertificateData).ToNot(BeEmpty())
					Expect(config.AuthInfos[0].AuthInfo.ClientKeyData).ToNot(BeEmpty())

					certPem, _ := pem.Decode(config.AuthInfos[0].AuthInfo.ClientCertificateData)
					cert, err := x509.ParseCertificate(certPem.Bytes)
					Expect(err).ToNot(HaveOccurred())

					Expect(cert.Subject.CommonName).To(Equal(userName))
					Expect(cert.Subject.Organization).To(ConsistOf("system:masters"))
					Expect(cert.NotAfter.Unix()).To(Equal(akcr.Status.ExpirationTimestamp.Time.Unix())) // certificates do not have nano seconds in them
					Expect(cert.NotBefore.UTC()).To(Equal(time.Unix(10, 0).UTC()))
					Expect(cert.Issuer.CommonName).To(Equal(expectedIssuerName))
				},

				Entry("one client CA, one cluster CA", clientCACert1Name, clusterCACert1, nil),

				Entry("one client CA, multiple cluster CA", clientCACert1Name, append(clusterCACert2, clusterCACert1...), func() {
					shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencore.GardenerResourceData{
						Name: "ca-cluster-2",
						Type: "secret",
						Labels: map[string]string{
							"name":             "ca",
							"managed-by":       "secrets-manager",
							"manager-identity": "gardenlet",
							"issued-at-time":   "1",
						},
						Data: runtime.RawExtension{
							Raw: []byte(`{"ca.crt":"` + utils.EncodeBase64(clusterCACert2) + `"}`),
						},
					})
				}),

				Entry("multiple client CA (in case of rotation), one cluster CA", clientCACert2Name, clusterCACert1, func() {
					shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencore.GardenerResourceData{
						Name: "ca-client-bar",
						Type: "secret",
						Labels: map[string]string{
							"name":             "ca-client",
							"managed-by":       "secrets-manager",
							"manager-identity": "gardenlet",
							"issued-at-time":   "1",
						},
						Data: runtime.RawExtension{
							Raw: []byte(`{"ca.crt":"` + utils.EncodeBase64(clientCACert2) + `","ca.key":"` + utils.EncodeBase64(clientCAKey2) + `"}`),
						},
					})
				}),

				Entry("multiple client CA (in case of rotation), multiple cluster CA", clientCACert2Name, append(clusterCACert2, clusterCACert1...), func() {
					shootState.Spec.Gardener = append(shootState.Spec.Gardener,
						gardencore.GardenerResourceData{
							Name: "ca-client-bar",
							Type: "secret",
							Labels: map[string]string{
								"name":             "ca-client",
								"managed-by":       "secrets-manager",
								"manager-identity": "gardenlet",
								"issued-at-time":   "1",
							},
							Data: runtime.RawExtension{
								Raw: []byte(`{"ca.crt":"` + utils.EncodeBase64(clientCACert2) + `","ca.key":"` + utils.EncodeBase64(clientCAKey2) + `"}`),
							},
						},
						gardencore.GardenerResourceData{
							Name: "ca-cluster-2",
							Type: "secret",
							Labels: map[string]string{
								"name":             "ca",
								"managed-by":       "secrets-manager",
								"manager-identity": "gardenlet",
								"issued-at-time":   "1",
							},
							Data: runtime.RawExtension{
								Raw: []byte(`{"ca.crt":"` + utils.EncodeBase64(clusterCACert2) + `"}`),
							},
						},
					)
				}),

				Entry("no client CA, one cluster CA", clientCACert1Name, clusterCACert1, func() {
					shootState.Spec.Gardener[0].Labels["name"] = "ca"
				}),
			)
		})
	})
})

type fakeGetter struct {
	obj runtime.Object
	err error
}

func (f *fakeGetter) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return f.obj, f.err
}

type fakeSecretLister struct {
	kubecorev1listers.SecretLister
	obj *corev1.Secret
	err error
}

func (f fakeSecretLister) Secrets(string) kubecorev1listers.SecretNamespaceLister {
	return f
}

func (f fakeSecretLister) Get(name string) (*corev1.Secret, error) {
	return f.obj, f.err
}

type fakeInternalSecretLister struct {
	gardencorelisters.InternalSecretLister
	obj *gardencore.InternalSecret
	err error
}

func (f fakeInternalSecretLister) InternalSecrets(string) gardencorelisters.InternalSecretNamespaceLister {
	return f
}

func (f fakeInternalSecretLister) Get(name string) (*gardencore.InternalSecret, error) {
	return f.obj, f.err
}
