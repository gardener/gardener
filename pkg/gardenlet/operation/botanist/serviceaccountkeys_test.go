// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("ServiceAccountKeys", func() {
	var (
		ctx = context.TODO()

		shootName           = "foo"
		shootNamespace      = "bar"
		gardenClient        client.Client
		shootClient         client.Client
		shootClientSet      kubernetes.Interface
		fakeShootRESTClient rest.Interface
		botanist            *Botanist

		expectedSecret *corev1.Secret
	)

	BeforeEach(func() {
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeShootRESTClient = &fakerest.RESTClient{
			NegotiatedSerializer: scheme.Codecs,
			Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/.well-known/openid-configuration":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("well-known")))}, nil
				case "/openid/v1/jwks":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("jwks")))}, nil
				default:
					return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(&bytes.Buffer{})}, nil
				}
			}),
		}

		shootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).WithRESTClient(fakeShootRESTClient).Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient:   gardenClient,
				ShootClientSet: shootClientSet,
				Shoot:          &shootpkg.Shoot{},
				Garden: &garden.Garden{
					Project: &gardencorev1beta1.Project{
						ObjectMeta: metav1.ObjectMeta{
							Name: "project-name",
						},
					},
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
				UID:       "uid",
			},
		})
		expectedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "project-name--uid",
				Namespace:       "gardener-system-shoot-issuer",
				ResourceVersion: "1",
				Labels: map[string]string{
					"shoot.gardener.cloud/namespace":            "bar",
					"authentication.gardener.cloud/public-keys": "serviceaccount",
					"project.gardener.cloud/name":               "project-name",
					"shoot.gardener.cloud/name":                 "foo",
				},
			},
			Data: map[string][]byte{
				"jwks":          []byte("jwks"),
				"openid-config": []byte("well-known"),
			},
		}
	})

	Describe("#SyncPublicServiceAccountKeys", func() {
		It("should sync public info", func() {
			Expect(botanist.SyncPublicServiceAccountKeys(ctx)).To(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "project-name--uid",
					Namespace: "gardener-system-shoot-issuer",
				},
			}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret).To(Equal(expectedSecret))
		})

		It("should overwrite the public info", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "project-name--uid",
					Namespace: "gardener-system-shoot-issuer",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			}
			Expect(gardenClient.Create(ctx, secret)).To(Succeed())
			Expect(botanist.SyncPublicServiceAccountKeys(ctx)).To(Succeed())
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			expectedSecret.ResourceVersion = "2"
			Expect(secret).To(Equal(expectedSecret))
		})

		Context("error responses", func() {
			It("should return bad request", func() {
				fakeShootRESTClient = &fakerest.RESTClient{
					NegotiatedSerializer: scheme.Codecs,
					Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
						switch req.URL.Path {
						case "/.well-known/openid-configuration":
							return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewReader([]byte("bad")))}, nil
						default:
							return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(&bytes.Buffer{})}, nil
						}
					}),
				}
				shootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).WithRESTClient(fakeShootRESTClient).Build()
				botanist.ShootClientSet = shootClientSet

				err := botanist.SyncPublicServiceAccountKeys(ctx)
				var statusError *apierrors.StatusError
				Expect(errors.As(err, &statusError)).To(BeTrue())
				Expect(statusError.Status().Code).To(Equal(int32(400)))
				Expect(statusError.Status().Details.Causes[0].Message).To(Equal("bad"))
			})

			It("should return internal server error", func() {
				fakeShootRESTClient = &fakerest.RESTClient{
					NegotiatedSerializer: scheme.Codecs,
					Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
						switch req.URL.Path {
						case "/.well-known/openid-configuration":
							return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("good")))}, nil
						case "/openid/v1/jwks":
							return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader([]byte("bad")))}, nil
						default:
							return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(&bytes.Buffer{})}, nil
						}
					}),
				}
				shootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).WithRESTClient(fakeShootRESTClient).Build()
				botanist.ShootClientSet = shootClientSet

				err := botanist.SyncPublicServiceAccountKeys(ctx)
				var statusError *apierrors.StatusError
				Expect(errors.As(err, &statusError)).To(BeTrue())
				Expect(statusError.Status().Code).To(Equal(int32(500)))
				Expect(statusError.Status().Details.Causes[0].Message).To(Equal("bad"))
			})
		})
	})

	Describe("#DeletePublicServiceAccountKeys", func() {
		It("should delete the public info", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "project-name--uid",
					Namespace: "gardener-system-shoot-issuer",
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}
			Expect(gardenClient.Create(ctx, secret)).To(Succeed())
			Expect(botanist.DeletePublicServiceAccountKeys(ctx)).To(Succeed())
			err := gardenClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})
})
