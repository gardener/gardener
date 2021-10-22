// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package tokenrequestor

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/scheme"
	corev1fake "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconciler", func() {
	Describe("#Reconcile", func() {
		var (
			ctx = context.TODO()

			logger     logr.Logger
			fakeClock  clock.Clock
			fakeJitter func(time.Duration, float64) time.Duration

			sourceClient, targetClient client.Client
			coreV1Client               *corev1fake.FakeCoreV1

			ctrl *reconciler

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

					cAction, _ := action.(testing.CreateAction)
					tr, _ := cAction.GetObject().(*authenticationv1.TokenRequest)

					return true, &authenticationv1.TokenRequest{
						Status: authenticationv1.TokenRequestStatus{
							Token:               token,
							ExpirationTimestamp: metav1.Time{Time: fakeNow.Add(time.Duration(*tr.Spec.ExpirationSeconds) * time.Second)},
						},
					}, nil
				})
			}
		)

		BeforeEach(func() {
			logger = log.Log.WithName("test")
			fakeNow = time.Date(2021, 10, 4, 10, 0, 0, 0, time.UTC)
			fakeClock = clock.NewFakeClock(fakeNow)
			fakeJitter = func(d time.Duration, _ float64) time.Duration { return d }

			sourceClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			targetClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			coreV1Client = &corev1fake.FakeCoreV1{Fake: &testing.Fake{}}

			ctrl = &reconciler{
				clock:              fakeClock,
				jitter:             fakeJitter,
				targetClient:       targetClient,
				targetCoreV1Client: coreV1Client,
			}

			Expect(ctrl.InjectLogger(logger)).To(Succeed())
			Expect(ctrl.InjectClient(sourceClient)).To(Succeed())

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

		It("should generate a new service account, a new token and requeue", func() {
			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(MatchError(ContainSubstring("not found")))

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
			Expect(serviceAccount.AutomountServiceAccountToken).To(PointTo(BeFalse()))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(layout)))
		})

		It("should requeue because renew timestamp has not been reached", func() {
			delay := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(delay).Format(layout))

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: delay}))
		})

		It("should issue a new token since the renew timestamp is in the past", func() {
			expiredSince := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(-expiredSince).Format(layout))

			token = "new-token"
			fakeCreateServiceAccountToken()

			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Create(ctx, serviceAccount)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(layout)))
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
			Expect(secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(layout)))
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

		It("should set a finalizer on the secret", func() {
			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())

			_, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Finalizers).To(ConsistOf("resources.gardener.cloud/tokenrequestor-controller"))
		})

		It("should remove the finalizer from the secret after deleting the ServiceAccount", func() {
			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(MatchError(ContainSubstring("not found")))

			_, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(sourceClient.Delete(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(MatchError(ContainSubstring("not found")))
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(MatchError(ContainSubstring("not found")))
		})

		It("should ignore the ServiceAccount on deletion if skip-deletion annotation is set", func() {
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/skip-deletion", "true")

			fakeCreateServiceAccountToken()
			Expect(sourceClient.Create(ctx, secret)).To(Succeed())
			_, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())

			Expect(sourceClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(sourceClient.Delete(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(MatchError(ContainSubstring("not found")))
			Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())
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
				metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", "invalid-format")
				Expect(sourceClient.Create(ctx, secret)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).To(MatchError(ContainSubstring("could not parse renew timestamp")))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
