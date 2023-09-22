// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NginxIngress", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		shootKubernetesVersion, _ := semver.NewVersion("1.22.1")
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{KubernetesVersion: shootKubernetesVersion}
		policy := corev1.ServiceExternalTrafficPolicyTypeCluster
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.22.1",
				},
				Addons: &gardencorev1beta1.Addons{
					NginxIngress: &gardencorev1beta1.NginxIngress{
						Config:                   map[string]string{},
						LoadBalancerSourceRanges: []string{},
						ExternalTrafficPolicy:    &policy,
					},
				},
			},
		})
		botanist.Garden = &garden.Garden{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultNginxIngress", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
		})

		It("should successfully create a nginxingress interface", func() {
			kubernetesClient.EXPECT().Client()

			nginxIngress, err := botanist.DefaultNginxIngress()
			Expect(nginxIngress).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	var (
		shootName     = "testShoot"
		seedNamespace = "shoot--foo--bar"

		scheme *runtime.Scheme
		client client.Client

		ingressDNSRecord *mockdnsrecord.MockInterface

		b *Botanist

		ctx     = context.TODO()
		now     = time.Now()
		testErr = fmt.Errorf("test")
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()

		ingressDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		b = &Botanist{
			Operation: &operation.Operation{
				Config: &config.GardenletConfiguration{
					Controllers: &config.GardenletControllerConfiguration{
						Shoot: &config.ShootControllerConfiguration{
							DNSEntryTTLSeconds: pointer.Int64(ttl),
						},
					},
				},
				Shoot: &shootpkg.Shoot{
					SeedNamespace:         seedNamespace,
					ExternalClusterDomain: pointer.String(externalDomain),
					ExternalDomain: &gardenerutils.Domain{
						Domain:   externalDomain,
						Provider: externalProvider,
						Zone:     externalZone,
						SecretData: map[string][]byte{
							"external-foo": []byte("external-bar"),
						},
					},
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{
							IngressDNSRecord: ingressDNSRecord,
						},
					},
				},
				Garden: &garden.Garden{},
				Logger: logr.Discard(),
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				DNS: &gardencorev1beta1.DNS{
					Domain: pointer.String(externalDomain),
				},
				Addons: &gardencorev1beta1.Addons{
					NginxIngress: &gardencorev1beta1.NginxIngress{
						Addon: gardencorev1beta1.Addon{
							Enabled: true,
						},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				ClusterIdentity: pointer.String("shoot-cluster-identity"),
			},
		})

		renderer := chartrenderer.NewWithServerVersion(&version.Info{})
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(client, mapper))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")

		b.SeedClientSet = kubernetesfake.NewClientSetBuilder().
			WithClient(client).
			WithChartApplier(chartApplier).
			Build()

		DeferCleanup(test.WithVar(&dnsrecord.TimeNow, func() time.Time { return now }))
	})

	Describe("#SetNginxIngressAddress", func() {
		It("does nothing when nginx is disabled", func() {
			b.Shoot.GetInfo().Spec.Addons.NginxIngress.Enabled = false

			ingressDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA).Times(0)
			ingressDNSRecord.EXPECT().SetValues([]string{address}).Times(0)

			b.SetNginxIngressAddress(address)
		})

		It("sets an ingress DNSRecord", func() {
			ingressDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			ingressDNSRecord.EXPECT().SetValues([]string{address})

			b.SetNginxIngressAddress(address)
		})
	})

	Describe("#DefaultIngressDNSRecord", func() {
		It("should create a component with correct values when nginx-ingress addon is enabled", func() {
			c := b.DefaultIngressDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			actual := c.GetValues()
			Expect(actual).To(DeepEqual(&dnsrecord.Values{
				Name:       b.Shoot.GetInfo().Name + "-ingress",
				SecretName: DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
				Namespace:  seedNamespace,
				TTL:        pointer.Int64(ttl),
				Type:       externalProvider,
				Zone:       pointer.String(externalZone),
				SecretData: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
				DNSName:           "*.ingress." + externalDomain,
				RecordType:        extensionsv1alpha1.DNSRecordTypeA,
				Values:            []string{address},
				AnnotateOperation: false,
			}))
		})

		DescribeTable("should set AnnotateOperation value to true",
			func(mutateShootFn func()) {
				mutateShootFn()

				c := b.DefaultIngressDNSRecord()

				Expect(c.GetValues().AnnotateOperation).To(BeTrue())
			},

			Entry("task annotation present", func() {
				shoot := b.Shoot.GetInfo()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordIngress")
				b.Shoot.SetInfo(shoot)
			}),
			Entry("restore phase", func() {
				shoot := b.Shoot.GetInfo()
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				b.Shoot.SetInfo(shoot)
			}),
		)

		It("should create a component with correct values when nginx-ingress addon is disabled", func() {
			b.Shoot.GetInfo().Spec.Addons.NginxIngress.Enabled = false

			c := b.DefaultIngressDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			actual := c.GetValues()
			Expect(actual).To(DeepEqual(&dnsrecord.Values{
				Name:       b.Shoot.GetInfo().Name + "-ingress",
				SecretName: DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
				Namespace:  seedNamespace,
				TTL:        pointer.Int64(ttl),
				Type:       externalProvider,
				Zone:       pointer.String(externalZone),
				SecretData: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
				DNSName:           "*.ingress." + externalDomain,
				RecordType:        extensionsv1alpha1.DNSRecordTypeA,
				Values:            []string{address},
				AnnotateOperation: false,
			}))
		})

		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			shoot := b.Shoot.GetInfo()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordIngress")
			b.Shoot.SetInfo(shoot)

			c := b.DefaultIngressDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			Expect(c.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := client.Get(ctx, types.NamespacedName{Name: shootName + "-ingress", Namespace: seedNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-ingress",
					Namespace:       seedNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: externalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
						Namespace: seedNamespace,
					},
					Zone:       pointer.String(externalZone),
					Name:       "*.ingress." + externalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        pointer.Int64(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
			}))
		})
	})

	Describe("#DeployOrDestroyIngressDNSRecord", func() {
		Context("deploy", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				ingressDNSRecord.EXPECT().Deploy(ctx)
				ingressDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				ingressDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("restore", func() {
			var shootState = &gardencorev1beta1.ShootState{}

			BeforeEach(func() {
				b.Shoot.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
			})

			It("should call Restore and Wait and succeed if they succeeded", func() {
				ingressDNSRecord.EXPECT().Restore(ctx, shootState)
				ingressDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				ingressDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (Addon disabled)", func() {
			BeforeEach(func() {
				b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Addons: nil,
					},
				})
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				ingressDNSRecord.EXPECT().Destroy(ctx)
				ingressDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				ingressDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DestroyIngressDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			ingressDNSRecord.EXPECT().Destroy(ctx)
			ingressDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyIngressDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			ingressDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateIngressDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			ingressDNSRecord.EXPECT().Migrate(ctx)
			ingressDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateIngressDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			ingressDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateIngressDNSRecord(ctx)).To(MatchError(testErr))
		})
	})
})
