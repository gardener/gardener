/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

	authenticationapi "github.com/gardener/gardener/pkg/apis/authentication"
	gardenercore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Admin Kubeconfig", func() {
	var (
		shootState       *gardenercore.ShootState
		shoot            *gardenercore.Shoot
		akcREST          *AdminKubeconfigREST
		shootStateGetter *fakeGetter
		shootGetter      *fakeGetter
		ctx              context.Context
		obj              *authenticationapi.AdminKubeconfigRequest
		caCert           []byte
		createValidation registryrest.ValidateObjectFunc
	)

	const (
		name      = "test"
		userName  = "foo"
		namespace = "baz"
	)
	BeforeEach(func() {
		caCert = []byte(`-----BEGIN CERTIFICATE-----
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
		caKey := []byte(`-----BEGIN RSA PRIVATE KEY-----
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

		caInfoData := secrets.CertificateInfoData{
			Certificate: caCert,
			PrivateKey:  caKey,
		}

		caRaw, err := caInfoData.Marshal()
		Expect(err).ToNot(HaveOccurred())

		shootState = &gardenercore.ShootState{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: gardenercore.ShootStateSpec{
				Gardener: []gardenercore.GardenerResourceData{
					{
						Name: "ca",
						Type: "certificate",
						Data: runtime.RawExtension{
							Raw: caRaw,
						},
					},
				},
			},
		}
		createValidation = func(ctx context.Context, obj runtime.Object) error { return nil }
		shoot = &gardenercore.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Status: gardenercore.ShootStatus{
				AdvertisedAddresses: []gardenercore.ShootAdvertisedAddress{
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
		obj = &authenticationapi.AdminKubeconfigRequest{
			Spec: authenticationapi.AdminKubeconfigRequestSpec{
				ExpirationSeconds: int64(time.Minute.Seconds() * 11),
			},
		}

		akcREST = &AdminKubeconfigREST{
			shootStorage:      shootGetter,
			shootStateStorage: shootStateGetter,
			now: func() time.Time {
				return time.Unix(10, 0)
			},
		}

		ctx = request.WithUser(context.Background(), &user.DefaultInfo{
			Name: userName,
		})
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

		It("returns error when create validation fails", func() {
			createValidation = func(ctx context.Context, obj runtime.Object) error {
				return errors.New("some error")
			}
		})

		It("returns error when validation fails", func() {
			obj.Spec.ExpirationSeconds = -1
		})

		It("returns error when there is no user in context", func() {
			ctx = context.TODO()
		})

		It("returns error when cannot get shoot state", func() {
			shootStateGetter.err = errors.New("can't get shoot state")
		})

		It("returns error when cannot convert to shoot state", func() {
			shootStateGetter.obj = &corev1.Pod{}
		})

		It("returns error when cannot get shoot", func() {
			shootGetter.err = errors.New("can't get shoot")
		})

		It("returns error when cannot convert to shoot", func() {
			shootGetter.obj = &corev1.Pod{}
		})

		It("returns error when there are no advertised addresses in shoot status", func() {
			shoot.Status.AdvertisedAddresses = nil
		})

		It("returns error when the certificate authority key is with bad type", func() {
			shootState.Spec.Gardener[0].Type = "bad type"
		})

		It("returns error when the certificate authority is not yet provisioned", func() {
			shootState.Spec.Gardener = nil
		})

		It("returns error when the certificate authority is not certificate info data", func() {
			shootState.Spec.Gardener[0].Type = "basicAuth"
		})

		It("returns error when the certificate authority contains no certificate", func() {
			caInfoData := secrets.CertificateInfoData{
				Certificate: []byte{1},
				PrivateKey:  []byte{2},
			}

			caRaw, err := caInfoData.Marshal()
			Expect(err).ToNot(HaveOccurred())

			shootState.Spec.Gardener[0].Data.Raw = caRaw
		})
	})

	It("returns kubeconfig successfully", func() {
		actual, err := akcREST.Create(ctx, name, obj, nil, nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(actual).ToNot(BeNil())
		Expect(actual).To(BeAssignableToTypeOf(&authenticationapi.AdminKubeconfigRequest{}))

		akcr := actual.(*authenticationapi.AdminKubeconfigRequest)

		Expect(akcr.Status.ExpirationTimestamp.Time).To(Equal(time.Unix(10, 0).Add(time.Minute * 11)))

		config := &configv1.Config{}
		Expect(runtime.DecodeInto(configlatest.Codec, akcr.Status.Kubeconfig, config)).To(Succeed())

		Expect(config.Clusters).To(ConsistOf(
			configv1.NamedCluster{
				Name: "baz--test-external",
				Cluster: configv1.Cluster{
					Server:                   "https://foo.bar.external:9443",
					CertificateAuthorityData: caCert,
				},
			},
			configv1.NamedCluster{
				Name: "baz--test-internal",
				Cluster: configv1.Cluster{
					Server:                   "https://foo.bar.internal:9443",
					CertificateAuthorityData: caCert,
				},
			},
		))

		Expect(config.Contexts).To(ConsistOf(
			configv1.NamedContext{
				Name: "baz--test-external",
				Context: configv1.Context{
					Cluster:  "baz--test-external",
					AuthInfo: "baz--test-external",
				},
			},
			configv1.NamedContext{
				Name: "baz--test-internal",
				Context: configv1.Context{
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
	})
})

type fakeGetter struct {
	obj runtime.Object
	err error
}

func (f *fakeGetter) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return f.obj, f.err
}
