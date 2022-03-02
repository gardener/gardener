// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Secrets", func() {
	var (
		ctx           = context.TODO()
		namespace     = "shoot--foo--bar"
		caSecretNames = []string{
			"ca",
			"ca-etcd",
			"ca-front-proxy",
			"ca-kubelet",
			"ca-metrics-server",
			"ca-vpn",
		}

		fakeClient         client.Client
		fakeSeedInterface  kubernetes.Interface
		fakeSecretsManager secretsmanager.Interface
		botanist           *Botanist
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		fakeSeedInterface = fakeclientset.NewClientSetBuilder().WithClient(fakeClient).Build()
		fakeSecretsManager = fake.New(fakeClient, namespace)
		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sSeedClient:  fakeSeedInterface,
				SecretsManager: fakeSecretsManager,
				Shoot: &shootpkg.Shoot{
					SeedNamespace: namespace,
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	Describe("#InitializeSecretsManagement", func() {
		Context("when shoot is not in restoration phase", func() {
			It("should get or generate the certificate authorities", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				for _, name := range caSecretNames {
					secret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, kutil.Key(namespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}
			})
		})

		Context("when shoot is in restoration phase", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore all secrets from the shootstate", func() {
				botanist.SetShootState(&gardencorev1alpha1.ShootState{
					Spec: gardencorev1alpha1.ShootStateSpec{
						Gardener: []gardencorev1alpha1.GardenerResourceData{
							{
								Name:   "ca",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager"},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "ca")},
							},
							{
								Name:   "ca-etcd",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager"},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "ca-etcd")},
							},
							{
								Name:   "non-ca-secret",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager"},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "non-ca-secret")},
							},
							{
								Name: "secret-without-labels",
								Type: "secret",
							},
							{
								Name: "some-other-data",
								Type: "not-a-secret",
							},
						},
					},
				})

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				By("verifying existing CA secrets got restored")
				for _, name := range caSecretNames[:1] {
					secret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, kutil.Key(namespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, Equal(map[string][]byte{"data-for": []byte(secret.Name)}))
				}

				By("verifying missing CA secrets got generated")
				for _, name := range caSecretNames[2:] {
					secret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, kutil.Key(namespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				By("verifying non-CA secrets got restored")
				secret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, kutil.Key(namespace, "non-ca-secret"), secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager"}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("verifying unrelated data not to be restored")
				Expect(fakeClient.Get(ctx, kutil.Key(namespace, "secret-without-labels"), &corev1.Secret{})).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, kutil.Key(namespace, "some-other-data"), &corev1.Secret{})).To(BeNotFoundError())
			})
		})
	})
})

func verifyCASecret(name string, secret *corev1.Secret, dataMatcher gomegatypes.GomegaMatcher) {
	Expect(secret.Immutable).To(PointTo(BeTrue()))
	Expect(secret.Labels).To(And(
		HaveKeyWithValue("name", name),
		HaveKeyWithValue("managed-by", "secrets-manager"),
		HaveKeyWithValue("persist", "true"),
		HaveKeyWithValue("rotation-strategy", "keepold"),
		HaveKey("checksum-of-config"),
		HaveKey("last-rotation-started-time"),
	))

	if dataMatcher != nil {
		Expect(secret.Data).To(dataMatcher)
	}
}

func rawData(key, value string) []byte {
	return []byte(`{"` + key + `":"` + utils.EncodeBase64([]byte(value)) + `"}`)
}
