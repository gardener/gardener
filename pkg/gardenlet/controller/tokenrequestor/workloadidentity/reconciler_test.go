// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/testing"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	securityfake "github.com/gardener/gardener/pkg/client/security/clientset/versioned/fake"
	"github.com/gardener/gardener/pkg/gardenlet/controller/tokenrequestor/workloadidentity"
)

var _ = Describe("Reconciler", func() {
	Describe("#Reconcile", func() {
		var (
			ctx = context.TODO()

			fakeClock  clock.Clock
			fakeJitter func(time.Duration, float64) time.Duration

			seedClient, gardenClient client.Client
			securityClient           *securityfake.Clientset

			ctrl *workloadidentity.Reconciler

			secret           *corev1.Secret
			workloadIdentity *securityv1alpha1.WorkloadIdentity
			request          reconcile.Request

			secretName                string
			workloadIdentityName      string
			workloadIdentityNamespace string
			expectedRenewDuration     time.Duration
			token                     string
			fakeNow                   time.Time

			fakeCreateWorkloadIdentityToken = func(tokenExpirationSeconds *int64) {
				securityClient.AddReactor("create", "workloadidentities", func(action testing.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "token" {
						return false, nil, errors.New("subresource should be 'token'")
					}

					cAction, ok := action.(testing.CreateAction)
					if !ok {
						return false, nil, fmt.Errorf("could not convert action (type %T) to type testing.CreateAction", cAction)
					}

					tokenRequest, ok := cAction.GetObject().(*securityv1alpha1.TokenRequest)
					if !ok {
						return false, nil, fmt.Errorf("could not convert object (type %T) to type *securityv1alpha1.TokenRequest", cAction.GetObject())
					}

					expirationSeconds := *tokenRequest.Spec.ExpirationSeconds
					if tokenExpirationSeconds != nil {
						expirationSeconds = *tokenExpirationSeconds
					}

					return true, &securityv1alpha1.TokenRequest{
						Status: securityv1alpha1.TokenRequestStatus{
							Token:               token,
							ExpirationTimestamp: metav1.Time{Time: fakeNow.Add(time.Duration(expirationSeconds) * time.Second)},
						},
					}, nil
				})
			}
		)

		BeforeEach(func() {
			fakeNow = time.Date(2021, 10, 4, 10, 0, 0, 0, time.UTC)
			fakeClock = testclock.NewFakeClock(fakeNow)
			fakeJitter = func(d time.Duration, _ float64) time.Duration { return d }

			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			securityClient = &securityfake.Clientset{Fake: testing.Fake{}}

			ctrl = &workloadidentity.Reconciler{
				SeedClient:           seedClient,
				GardenClient:         gardenClient,
				GardenSecurityClient: securityClient,
				Clock:                fakeClock,
				JitterFunc:           fakeJitter,
			}

			secretName = "cloudsecret"
			workloadIdentityName = "foo-cloud"
			workloadIdentityNamespace = "garden-foo"
			expectedRenewDuration = 6 * time.Hour * 80 / 100
			token = "foo"

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						"workloadidentity.security.gardener.cloud/name":      workloadIdentityName,
						"workloadidentity.security.gardener.cloud/namespace": workloadIdentityNamespace,
					},
					Labels: map[string]string{
						"security.gardener.cloud/purpose": "workload-identity-token-requestor",
					},
				},
			}
			workloadIdentity = &securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workloadIdentityName,
					Namespace: workloadIdentityNamespace,
				},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"target-audience"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "foocloud",
					},
				},
			}
			request = reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			}}
		})

		It("should generate a new token and requeue", func() {
			fakeCreateWorkloadIdentityToken(nil)
			Expect(seedClient.Create(ctx, secret)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should generate a new token and requeue when token is missing but renew timestamp is present", func() {
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "workloadidentity.security.gardener.cloud/token-renew-timestamp", fakeNow.Add(time.Hour).Format(time.RFC3339))
			fakeCreateWorkloadIdentityToken(nil)
			Expect(seedClient.Create(ctx, secret)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should requeue because renew timestamp has not been reached", func() {
			secret.Data = map[string][]byte{"token": []byte("some-token")}

			delay := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "workloadidentity.security.gardener.cloud/token-renew-timestamp", fakeNow.Add(delay).Format(time.RFC3339))

			Expect(seedClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: delay}))
		})

		It("should issue a new token since the renew timestamp is in the past", func() {
			expiredSince := time.Minute
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "workloadidentity.security.gardener.cloud/token-renew-timestamp", fakeNow.Add(-expiredSince).Format(time.RFC3339))

			token = "new-token"
			fakeCreateWorkloadIdentityToken(nil)

			Expect(seedClient.Create(ctx, secret)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue("token", []byte(token)))
			Expect(secret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/token-renew-timestamp", fakeNow.Add(expectedRenewDuration).Format(time.RFC3339)))
		})

		It("should do nothing if the secret does not have the purpose label", func() {
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())
			secret.Labels = nil
			Expect(seedClient.Create(ctx, secret)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Labels).To(BeEmpty())
			Expect(secret.Data).To(BeEmpty())
		})

		It("should always renew the token after 24h", func() {
			expectedRenewDuration = 24 * time.Hour * 80 / 100
			fakeCreateWorkloadIdentityToken(ptr.To[int64](3600 * 100))

			Expect(seedClient.Create(ctx, secret)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: expectedRenewDuration}))
		})

		Context("error", func() {
			It("provided context object cannot be parsed", func() {
				secret.Annotations["workloadidentity.security.gardener.cloud/context-object"] = "unparsable"

				Expect(seedClient.Create(ctx, secret)).To(Succeed())
				Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).To(MatchError(ContainSubstring("cannot parse context object")))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("renew timestamp has invalid format", func() {
				secret.Data = map[string][]byte{"token": []byte("some-token")}

				metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "workloadidentity.security.gardener.cloud/token-renew-timestamp", "invalid-format")
				Expect(seedClient.Create(ctx, secret)).To(Succeed())
				Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(err).To(MatchError(ContainSubstring("could not parse renew timestamp")))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
