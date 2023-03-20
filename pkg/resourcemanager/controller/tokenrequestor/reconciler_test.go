// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tokenrequestor_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	corev1fake "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/testing"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/tokenrequestor"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	Describe("#Reconcile", func() {
		var (
			ctx = context.TODO()

			fakeClock  clock.Clock
			fakeJitter func(time.Duration, float64) time.Duration

			sourceClient, targetClient client.Client
			coreV1Client               *corev1fake.FakeCoreV1

			ctrl reconcile.Reconciler

			secret         *corev1.Secret
			serviceAccount *corev1.ServiceAccount
			request        reconcile.Request

			secretName              string
			serviceAccountName      string
			serviceAccountNamespace string
			expectedRenewDuration   time.Duration
			token                   string
			fakeNow                 time.Time

			fakeCreateServiceAccountToken = func() {
				coreV1Client.AddReactor("create", "serviceaccounts", func(action testing.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "token" {
						return false, nil, fmt.Errorf("subresource should be 'token'")
					}

					cAction, ok := action.(testing.CreateAction)
					if !ok {
						return false, nil, fmt.Errorf("could not convert action (type %T) to type testing.CreateAction", cAction)
					}

					tokenRequest, ok := cAction.GetObject().(*authenticationv1.TokenRequest)
					if !ok {
						return false, nil, fmt.Errorf("could not convert object (type %T) to type *authenticationv1.TokenRequest", cAction.GetObject())
					}

					return true, &authenticationv1.TokenRequest{
						Status: authenticationv1.TokenRequestStatus{
							Token:               token,
							ExpirationTimestamp: metav1.Time{Time: fakeNow.Add(time.Duration(*tokenRequest.Spec.ExpirationSeconds) * time.Second)},
						},
					}, nil
				})
			}
		)

		BeforeEach(func() {
			fakeNow = time.Date(2021, 10, 4, 10, 0, 0, 0, time.UTC)
			fakeClock = testclock.NewFakeClock(fakeNow)
			fakeJitter = func(d time.Duration, _ float64) time.Duration { return d }

			sourceClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			targetClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			coreV1Client = &corev1fake.FakeCoreV1{Fake: &testing.Fake{}}

			ctrl = &Reconciler{
				SourceClient:       sourceClient,
				TargetClient:       targetClient,
				TargetCoreV1Client: coreV1Client,
				Clock:              fakeClock,
				JitterFunc:         fakeJitter,
			}

			secretName = "kube-scheduler"
			serviceAccountName = "kube-scheduler-serviceaccount"
			serviceAccountNamespace = "kube-system"
			// If no token-expiration-duration is set then the default of 12 hours is used
			expectedRenewDuration = 12 * time.Hour * 80 / 100
			token = "foo"

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						"serviceaccount.resources.gardener.cloud/name":      serviceAccountName,
						"serviceaccount.resources.gardener.cloud/namespace": serviceAccountNamespace,
					},
					Labels: map[string]string{
						"resources.gardener.cloud/purpose": "token-requestor",
					},
				},
			}
			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: serviceAccountNamespace,
				},
			}
			request = reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			}}
		})

		It("should create a new service account, generate a new token and requeue", func() {
			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should create a new service account, generate a new token for the kubeconfig and requeue", func() {
			secret.Data = map[string][]byte{"kubeconfig": newKubeconfigRaw("")}

			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).NotTo(HaveKey("token"))
			Expect(secret.Data).To(HaveKeyWithValue("kubeconfig", newKubeconfigRaw(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should create a new service account and a new target secret, generate a new token and requeue", func() {
			targetSecretName, targetSecretNamespace := "foo", "bar"
			secret.Annotations["token-requestor.resources.gardener.cloud/target-secret-name"] = targetSecretName
			secret.Annotations["token-requestor.resources.gardener.cloud/target-secret-namespace"] = targetSecretNamespace

			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).NotTo(HaveKey("token"))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			targetSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetSecretName,
					Namespace: targetSecretNamespace,
				},
			}

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(targetSecret), targetSecret)).To(Succeed())
			Expect(targetSecret.Data).To(HaveKeyWithValue("token", []byte(token)))
		})

		It("should create a new service account, generate a new token and requeue, and create the target secret in the next reconciliation", func() {
			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			targetSecretName, targetSecretNamespace := "foo", "bar"
			secret.Annotations["token-requestor.resources.gardener.cloud/target-secret-name"] = targetSecretName
			secret.Annotations["token-requestor.resources.gardener.cloud/target-secret-namespace"] = targetSecretNamespace
			Expect(sourceClient.Update(ctx, secret)).To(Succeed())

			result, err = ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			targetSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetSecretName,
					Namespace: targetSecretNamespace,
				},
			}

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(targetSecret), targetSecret)).To(Succeed())
			Expect(targetSecret.Data).To(HaveKeyWithValue("token", []byte(token)))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).NotTo(HaveKey("token"))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		Context("token missing but renew timestamp present", func() {
			BeforeEach(func() {
				metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(time.Hour).Format(time.RFC3339))
			})

			It("should create a new service account, generate a new token and requeue", func() {
				fakeCreateServiceAccountToken()
				Expect(sourceClient.Create(ctx, secret)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
				Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

				Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
				Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
			})

			It("should create a new service account, generate a new token for the kubeconfig and requeue", func() {
				secret.Data = map[string][]byte{"kubeconfig": newKubeconfigRaw("")}

				fakeCreateServiceAccountToken()
				Expect(sourceClient.Create(ctx, secret)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
				Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

				Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				Expect(secret.Data).NotTo(HaveKey("token"))
				Expect(secret.Data).To(HaveKeyWithValue("kubeconfig", newKubeconfigRaw(token)))
				Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
			})

			It("should requeue when token is present in the target cluster", func() {
				targetSecretName, targetSecretNamespace := "foo", "bar"
				secret.Annotations["token-requestor.resources.gardener.cloud/target-secret-name"] = targetSecretName
				secret.Annotations["token-requestor.resources.gardener.cloud/target-secret-namespace"] = targetSecretNamespace

				targetSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetSecretName,
						Namespace: targetSecretNamespace,
					},
					Data: map[string][]byte{"token": []byte("token")},
				}

				Expect(targetClient.Create(ctx, targetSecret)).To(Succeed())
				Expect(sourceClient.Create(ctx, secret)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: time.Hour}))
			})
		})

		It("should fail when the provided kubeconfig cannot be decoded", func() {
			secret.Data = map[string][]byte{"kubeconfig": []byte("some non-decodeable stuff")}

			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(BeNotFoundError())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal string into Go value of type")))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should requeue because renew timestamp has not been reached (token case)", func() {
			secret.Data = map[string][]byte{"token": []byte("some-token")}

			delay := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(delay).Format(time.RFC3339))

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: delay}))
		})

		It("should requeue because renew timestamp has not been reached (kubeconfig case)", func() {
			secret.Data = map[string][]byte{"kubeconfig": newKubeconfigRaw("some-token")}

			delay := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(delay).Format(time.RFC3339))

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: delay}))
		})

		It("should issue a new token since the renew timestamp is in the past", func() {
			expiredSince := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(-expiredSince).Format(time.RFC3339))

			token = "new-token"
			fakeCreateServiceAccountToken()

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Create(ctx, serviceAccount)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should reconcile the service account settings", func() {
			serviceAccount.AutomountServiceAccountToken = pointer.Bool(true)

			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Create(ctx, serviceAccount)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))
		})

		It("should do nothing if the secret does not have the purpose label", func() {
			Expect(targetClient.Create(ctx, serviceAccount)).To(Succeed())
			secret.Labels = nil
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Consistently(func() error {
				return targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
			}).Should(Succeed())
		})

		It("should use the provided token expiration duration", func() {
			expirationDuration := 10 * time.Minute
			expectedRenewDuration = 8 * time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-expiration-duration", expirationDuration.String())
			fakeCreateServiceAccountToken()

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should always renew the token after 24h", func() {
			expirationDuration := 100 * time.Hour
			expectedRenewDuration = 24 * time.Hour * 80 / 100
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-expiration-duration", expirationDuration.String())
			fakeCreateServiceAccountToken()

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))
		})

		Context("error", func() {
			It("provided token expiration duration cannot be parsed", func() {
				metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-expiration-duration", "unparseable")

				Expect(sourceClient.Create(ctx, secret)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).To(MatchError(ContainSubstring("invalid duration")))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("renew timestamp has invalid format", func() {
				secret.Data = map[string][]byte{"token": []byte("some-token")}

				metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", "invalid-format")
				Expect(sourceClient.Create(ctx, secret)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).To(MatchError(ContainSubstring("could not parse renew timestamp")))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})

func newKubeconfigRaw(token string) []byte {
	return []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: some-server-url
  name: shoot--foo--bar
contexts:
- context:
    cluster: shoot--foo--bar
    user: shoot--foo--bar-token
  name: shoot--foo--bar
current-context: shoot--foo--bar
kind: Config
preferences: {}
users:
- name: shoot--foo--bar-token
  user:
    token: ` + token + `
`)
}
