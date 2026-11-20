// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"context"
	"encoding/json"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
				Config: &gardenletconfigv1alpha1.TokenRequestorWorkloadIdentityControllerConfiguration{
					TokenExpirationDuration: &metav1.Duration{Duration: 6 * time.Hour},
				},
				GardenNamespace: "garden",
			}

			secretName = "cloudsecret"
			workloadIdentityName = "foo-cloud"
			workloadIdentityNamespace = "garden-foo"
			expectedRenewDuration = 6 * time.Hour * 50 / 100
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
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: expectedRenewDuration}))

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
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: expectedRenewDuration}))

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
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: delay}))
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
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: expectedRenewDuration}))

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
			expectedRenewDuration = 24 * time.Hour * 50 / 100
			fakeCreateWorkloadIdentityToken(new(int64(3600 * 100)))

			Expect(seedClient.Create(ctx, secret)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: expectedRenewDuration}))
		})

		Context("context object", func() {
			const (
				shootNamespace        = "garden-foo"
				shootName             = "bar"
				shootUID              = types.UID("4bdd6ae0-3d4e-4f8b-9a9b-1e6b2d3c4f5a")
				controlPlaneNamespace = "shoot--foo--bar"
				backupEntryName       = controlPlaneNamespace + "--" + string(shootUID)
			)

			var (
				coreGroupVersion = gardencorev1beta1.SchemeGroupVersion.String()

				setContextObject = func(namespace string, contextObject securityv1alpha1.ContextObject) {
					raw, err := json.Marshal(contextObject)
					Expect(err).NotTo(HaveOccurred())
					secret.Namespace = namespace
					secret.Annotations["workloadidentity.security.gardener.cloud/context-object"] = string(raw)
					request.Namespace = namespace
				}

				tokenRequested = func() bool {
					for _, action := range securityClient.Actions() {
						if action.GetVerb() == "create" && action.GetSubresource() == "token" {
							return true
						}
					}
					return false
				}
			)

			BeforeEach(func() {
				Expect(gardenClient.Create(ctx, &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: shootName, UID: shootUID},
					Status:     gardencorev1beta1.ShootStatus{TechnicalID: controlPlaneNamespace},
				})).To(Succeed())
				Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())
				fakeCreateWorkloadIdentityToken(nil)
			})

			DescribeTable("should request a token for context objects belonging to the secret's namespace",
				func(namespace string, contextObject securityv1alpha1.ContextObject) {
					setContextObject(namespace, contextObject)
					Expect(seedClient.Create(ctx, secret)).To(Succeed())

					result, err := ctrl.Reconcile(ctx, request)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{RequeueAfter: expectedRenewDuration}))
					Expect(tokenRequested()).To(BeTrue())
				},

				Entry("shoot in its control plane namespace", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Shoot", Namespace: new(shootNamespace), Name: shootName, UID: shootUID}),
				Entry("seed in the garden namespace", "garden", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Seed", Name: "seed"}),
				Entry("backupBucket in the garden namespace", "garden", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "BackupBucket", Name: "bucket"}),
				Entry("backupEntry in the garden namespace", "garden", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "BackupEntry", Namespace: new(shootNamespace), Name: backupEntryName}),
				Entry("backupEntry in the shoot's control plane namespace", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "BackupEntry", Namespace: new(shootNamespace), Name: backupEntryName}),
				Entry("source backupEntry in the shoot's control plane namespace", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "BackupEntry", Namespace: new(shootNamespace), Name: "source-" + backupEntryName}),
			)

			DescribeTable("should not request a token for context objects not belonging to the secret's namespace",
				func(namespace string, contextObject securityv1alpha1.ContextObject) {
					setContextObject(namespace, contextObject)
					Expect(seedClient.Create(ctx, secret)).To(Succeed())

					result, err := ctrl.Reconcile(ctx, request)
					Expect(err).To(MatchError(ContainSubstring("is not allowed for secrets in namespace")))
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(tokenRequested()).To(BeFalse())
				},

				Entry("shoot in another shoot's control plane namespace", "shoot--foo--other", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Shoot", Namespace: new(shootNamespace), Name: shootName, UID: shootUID}),
				Entry("shoot in the garden namespace", "garden", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Shoot", Namespace: new(shootNamespace), Name: shootName, UID: shootUID}),
				Entry("shoot with mismatching UID", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Shoot", Namespace: new(shootNamespace), Name: shootName, UID: "other"}),
				Entry("non-existing shoot", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Shoot", Namespace: new(shootNamespace), Name: "other", UID: shootUID}),
				Entry("shoot without namespace", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Shoot", Name: shootName, UID: shootUID}),
				Entry("seed in a shoot's control plane namespace", controlPlaneNamespace, securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "Seed", Name: "seed"}),
				Entry("backupBucket in an extension namespace", "extension-foo", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "BackupBucket", Name: "bucket"}),
				Entry("backupEntry in another shoot's control plane namespace", "shoot--foo--other", securityv1alpha1.ContextObject{APIVersion: coreGroupVersion, Kind: "BackupEntry", Namespace: new(shootNamespace), Name: backupEntryName}),
				Entry("unsupported kind", "garden", securityv1alpha1.ContextObject{APIVersion: "v1", Kind: "Namespace", Name: "garden"}),
			)
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
