// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"time"

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
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	shootNamespace = "bar"

	externalDomain   = "foo.bar.external.example.com"
	externalProvider = "external-provider"
	externalZone     = "external-zone"

	priorExternalDomain   = "foo.bar.prior-external.example.com"
	priorExternalProvider = "prior-external-provider"
	priorExternalZone     = "prior-external-zone"

	internalDomain   = "foo.bar.internal.example.com"
	internalProvider = "internal-provider"
	internalZone     = "internal-zone"

	priorInternalDomain   = "foo.bar.prior-internal.example.com"
	priorInternalProvider = "prior-internal-provider"
	priorInternalZone     = "prior-internal-zone"

	address       = "1.2.3.4"
	ttl     int64 = 300
)

var _ = Describe("dnsrecord", func() {
	var (
		shootName             string
		controlPlaneNamespace string
		ctrl                  *gomock.Controller

		scheme *runtime.Scheme
		c      client.Client

		externalDNSRecord      *mockdnsrecord.MockInterface
		priorExternalDNSRecord *mockdnsrecord.MockInterface
		internalDNSRecord      *mockdnsrecord.MockInterface
		priorInternalDNSRecord *mockdnsrecord.MockInterface

		b *Botanist

		ctx                 = context.TODO()
		now                 = time.Now()
		testErr             = errors.New("test")
		seedClusterIdentity = "seed1"

		cleanup func()
	)

	BeforeEach(func() {
		shootName = "foo"
		controlPlaneNamespace = "shoot--foo--bar"
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(scheme).Build()

		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		priorExternalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		priorInternalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		cleanup = test.WithVar(&dnsrecord.TimeNow, func() time.Time { return now })
	})

	JustBeforeEach(func() {
		b = &Botanist{
			Operation: &operation.Operation{
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
						Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
							DNSEntryTTLSeconds: new(ttl),
						},
					},
				},
				Shoot: &shoot.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					ExternalClusterDomain: new(externalDomain),
					ExternalDomain: &gardenerutils.Domain{
						Domain:   externalDomain,
						Provider: externalProvider,
						Zone:     externalZone,
						Credentials: &corev1.Secret{
							Data: map[string][]byte{
								"external-foo": []byte("external-bar"),
							},
						},
					},
					InternalClusterDomain: new(internalDomain),
					InternalDomain: &gardenerutils.Domain{
						Domain:   internalDomain,
						Provider: internalProvider,
						Zone:     internalZone,
						Credentials: &corev1.Secret{
							Data: map[string][]byte{
								"internal-foo": []byte("internal-bar"),
							},
						},
					},
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							ExternalDNSRecord:      externalDNSRecord,
							PriorExternalDNSRecord: priorExternalDNSRecord,
							InternalDNSRecord:      internalDNSRecord,
							PriorInternalDNSRecord: priorInternalDNSRecord,
						},
					},
				},
				Garden: &garden.Garden{},
				Seed:   &seed.Seed{},
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
					Domain: new(externalDomain),
				},
			},
		})

		b.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				ClusterIdentity: &seedClusterIdentity,
			},
		})

		renderer := chartrenderer.NewWithServerVersion(&version.Info{})
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")
		b.SeedClientSet = fakekubernetes.NewClientSetBuilder().
			WithClient(c).
			WithChartApplier(chartApplier).
			Build()
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Describe("#DefaultExternalDNSRecord", func() {
		It("should create a component with correct values", func() {
			r := b.DefaultExternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			actual := r.GetValues()
			Expect(actual).To(DeepEqual(&dnsrecord.Values{
				Name:              b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
				SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
				Namespace:         controlPlaneNamespace,
				TTL:               new(ttl),
				Type:              externalProvider,
				Zone:              new(externalZone),
				DNSName:           "api." + externalDomain,
				RecordType:        extensionsv1alpha1.DNSRecordTypeA,
				Values:            []string{address},
				AnnotateOperation: false,
				IPStack:           "ipv4",
				Labels: map[string]string{
					"role":                "external",
					"gardener.cloud/role": "controlplane",
				},
			}))
		})

		DescribeTable("should set AnnotateOperation value to true",
			func(mutateShootFn func()) {
				mutateShootFn()

				c := b.DefaultExternalDNSRecord()

				Expect(c.GetValues().AnnotateOperation).To(BeTrue())
			},

			Entry("task annotation present", func() {
				shoot := b.Shoot.GetInfo()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordExternal")
				b.Shoot.SetInfo(shoot)
			}),
			Entry("restore phase", func() {
				shoot := b.Shoot.GetInfo()
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				b.Shoot.SetInfo(shoot)
			}),
		)

		It("should create a component that creates the DNSRecord and its secret on Deploy with static credentials", func() {
			shoot := b.Shoot.GetInfo()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordExternal")
			b.Shoot.SetInfo(shoot)

			r := b.DefaultExternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).To(Succeed())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: controlPlaneNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       controlPlaneNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					Labels: map[string]string{
						"role":                "external",
						"gardener.cloud/role": "controlplane",
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: externalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
						Namespace: controlPlaneNamespace,
					},
					Zone:       new(externalZone),
					Name:       "api." + externalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        new(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: controlPlaneNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       controlPlaneNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
			}))
		})

		It("should create a component that creates the DNSRecord and its secret on Deploy with WorkloadIdentity credentials", func() {
			shoot := b.Shoot.GetInfo()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordExternal")

			b.Shoot.ExternalDomain.Credentials = &securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wi-dns-external",
					Namespace: "garden",
				},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"dns"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "test",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"foo":"bar"}`),
						},
					},
				},
			}

			r := b.DefaultExternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).To(Succeed())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: controlPlaneNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       controlPlaneNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					Labels: map[string]string{
						"role":                "external",
						"gardener.cloud/role": "controlplane",
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: externalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
						Namespace: controlPlaneNamespace,
					},
					Zone:       new(externalZone),
					Name:       "api." + externalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        new(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName, Namespace: controlPlaneNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordExternalName,
					Namespace:       controlPlaneNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"` + shootName + `","namespace":"` + shootNamespace + `","uid":""}`,
						"workloadidentity.security.gardener.cloud/name":           "wi-dns-external",
						"workloadidentity.security.gardener.cloud/namespace":      "garden",
					},
					Labels: map[string]string{
						"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
						"workloadidentity.security.gardener.cloud/provider": "test",
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"config": []byte(`{"foo":"bar"}`),
				},
			}))
		})
	})

	Describe("#DefaultPriorExternalDNSRecord", func() {
		Context("domain migration in progress", func() {
			JustBeforeEach(func() {
				b.Shoot.PriorExternalClusterDomain = new(priorExternalDomain)
				b.Shoot.PriorExternalDomain = &gardenerutils.Domain{
					Domain:   priorExternalDomain,
					Provider: priorExternalProvider,
					Zone:     priorExternalZone,
					Credentials: &corev1.Secret{
						Data: map[string][]byte{
							"prior-external-foo": []byte("prior-external-bar"),
						},
					},
				}
			})

			It("should create a component with correct values", func() {
				r := b.DefaultPriorExternalDNSRecord()
				r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				r.SetValues([]string{address})

				Expect(r.GetValues()).To(DeepEqual(&dnsrecord.Values{
					Name:              b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordPriorExternalName,
					SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordPriorExternalName,
					Namespace:         controlPlaneNamespace,
					TTL:               new(ttl),
					Type:              priorExternalProvider,
					Zone:              new(priorExternalZone),
					DNSName:           "api." + priorExternalDomain,
					RecordType:        extensionsv1alpha1.DNSRecordTypeA,
					Values:            []string{address},
					AnnotateOperation: false,
					IPStack:           "ipv4",
					Labels: map[string]string{
						"role":                "prior-external",
						"gardener.cloud/role": "controlplane",
					},
				}))
			})
		})

		Context("no domain migration", func() {
			It("should create a component without domain information", func() {
				values := b.DefaultPriorExternalDNSRecord().GetValues()

				Expect(values.DNSName).To(BeEmpty())
				Expect(values.Type).To(BeEmpty())
				Expect(values.Zone).To(BeNil())
			})
		})
	})

	Describe("#DefaultInternalDNSRecord", func() {
		It("should create a component with correct values", func() {
			c := b.DefaultInternalDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			actual := c.GetValues()
			Expect(actual).To(DeepEqual(&dnsrecord.Values{
				Name:              b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordInternalName,
				SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordInternalName,
				Namespace:         controlPlaneNamespace,
				TTL:               new(ttl),
				Type:              internalProvider,
				Zone:              new(internalZone),
				DNSName:           "api." + internalDomain,
				RecordType:        extensionsv1alpha1.DNSRecordTypeA,
				Values:            []string{address},
				AnnotateOperation: false,
				IPStack:           "ipv4",
				Labels: map[string]string{
					"role":                "internal",
					"gardener.cloud/role": "controlplane",
				},
			}))
		})

		DescribeTable("should set AnnotateOperation value to true",
			func(mutateShootFn func()) {
				mutateShootFn()

				c := b.DefaultInternalDNSRecord()

				Expect(c.GetValues().AnnotateOperation).To(BeTrue())
			},

			Entry("shoot deletion", func() {
				shoot := b.Shoot.GetInfo()
				shoot.DeletionTimestamp = &metav1.Time{}
				b.Shoot.SetInfo(shoot)
			}),
			Entry("task annotation present", func() {
				shoot := b.Shoot.GetInfo()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordInternal")
				b.Shoot.SetInfo(shoot)
			}),
			Entry("restore phase", func() {
				shoot := b.Shoot.GetInfo()
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				b.Shoot.SetInfo(shoot)
			}),
		)

		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			shoot := b.Shoot.GetInfo()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/tasks", "deployDNSRecordInternal")
			b.Shoot.SetInfo(shoot)

			r := b.DefaultInternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + v1beta1constants.DNSRecordInternalName, Namespace: controlPlaneNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + v1beta1constants.DNSRecordInternalName,
					Namespace:       controlPlaneNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					Labels: map[string]string{
						"role":                "internal",
						"gardener.cloud/role": "controlplane",
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: internalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName,
						Namespace: controlPlaneNamespace,
					},
					Zone:       new(internalZone),
					Name:       "api." + internalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        new(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName, Namespace: controlPlaneNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName,
					Namespace:       controlPlaneNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"internal-foo": []byte("internal-bar"),
				},
			}))
		})

		Context("self-hosted shoot", func() {
			var exposure *gardencorev1beta1.Exposure

			BeforeEach(func() {
				exposure = &gardencorev1beta1.Exposure{DNS: &gardencorev1beta1.DNSExposure{}}
			})

			JustBeforeEach(func() {
				b.Shoot.InternalDomain = nil
				b.Shoot.InternalClusterDomain = nil

				shoot := b.Shoot.GetInfo()
				shoot.Spec.CredentialsBindingName = new("credentials")
				shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
					Name:         "control-plane",
					ControlPlane: &gardencorev1beta1.WorkerControlPlane{Exposure: exposure},
				}}
				b.Shoot.SetInfo(shoot)
			})

			It("should use the external domain's provider and credentials if the control plane is exposed", func() {
				r := b.DefaultInternalDNSRecord()
				r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				r.SetValues([]string{address})

				Expect(r.GetValues()).To(DeepEqual(&dnsrecord.Values{
					Name:              b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordInternalName,
					SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordInternalName,
					Namespace:         controlPlaneNamespace,
					TTL:               new(ttl),
					Type:              externalProvider,
					Zone:              new(externalZone),
					DNSName:           "api.internal." + externalDomain,
					RecordType:        extensionsv1alpha1.DNSRecordTypeA,
					Values:            []string{address},
					AnnotateOperation: false,
					IPStack:           "ipv4",
					Labels: map[string]string{
						"role":                "internal",
						"gardener.cloud/role": "controlplane",
					},
				}))

				Expect(r.Deploy(ctx)).To(Succeed())

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName, Namespace: controlPlaneNamespace}, secret)).To(Succeed())
				Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + v1beta1constants.DNSRecordInternalName,
						Namespace:       controlPlaneNamespace,
						ResourceVersion: "1",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"external-foo": []byte("external-bar"),
					},
				}))
			})

			When("the control plane is not exposed", func() {
				BeforeEach(func() {
					exposure = nil
				})

				It("should not configure the internal DNSRecord", func() {
					values := b.DefaultInternalDNSRecord().GetValues()
					Expect(values.Type).To(BeEmpty())
					Expect(values.DNSName).To(BeEmpty())
				})
			})
		})
	})

	Describe("#DefaultPriorInternalDNSRecord", func() {
		Context("domain migration in progress", func() {
			JustBeforeEach(func() {
				b.Shoot.PriorInternalClusterDomain = new(priorInternalDomain)
				b.Shoot.PriorInternalDomain = &gardenerutils.Domain{
					Domain:   priorInternalDomain,
					Provider: priorInternalProvider,
					Zone:     priorInternalZone,
					Credentials: &corev1.Secret{
						Data: map[string][]byte{
							"prior-internal-foo": []byte("prior-internal-bar"),
						},
					},
				}
			})

			It("should create a component with correct values", func() {
				r := b.DefaultPriorInternalDNSRecord()
				r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				r.SetValues([]string{address})

				Expect(r.GetValues()).To(DeepEqual(&dnsrecord.Values{
					Name:              b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordPriorInternalName,
					SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordPriorInternalName,
					Namespace:         controlPlaneNamespace,
					TTL:               new(ttl),
					Type:              priorInternalProvider,
					Zone:              new(priorInternalZone),
					DNSName:           "api." + priorInternalDomain,
					RecordType:        extensionsv1alpha1.DNSRecordTypeA,
					Values:            []string{address},
					AnnotateOperation: false,
					IPStack:           "ipv4",
					Labels: map[string]string{
						"role":                "prior-internal",
						"gardener.cloud/role": "controlplane",
					},
				}))
			})

			It("should reconcile only on change or error and annotate the operation during the shoot deletion", func() {
				shoot := b.Shoot.GetInfo()
				shoot.DeletionTimestamp = &metav1.Time{}
				b.Shoot.SetInfo(shoot)

				values := b.DefaultPriorInternalDNSRecord().GetValues()

				Expect(values.ReconcileOnlyOnChangeOrError).To(BeTrue())
				Expect(values.AnnotateOperation).To(BeTrue())
			})
		})

		Context("no domain migration", func() {
			It("should create a component without domain information", func() {
				values := b.DefaultPriorInternalDNSRecord().GetValues()

				Expect(values.DNSName).To(BeEmpty())
				Expect(values.Type).To(BeEmpty())
				Expect(values.Zone).To(BeNil())
			})
		})
	})

	Describe("#DeployOrDestroyExternalDNSRecord", func() {
		Context("domain migration in progress", func() {
			createExternalDNSRecord := func(name string) {
				Expect(c.Create(ctx, &extensionsv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: shootName + "-external", Namespace: controlPlaneNamespace},
					Spec: extensionsv1alpha1.DNSRecordSpec{
						Name:       name,
						RecordType: extensionsv1alpha1.DNSRecordTypeA,
						Values:     []string{address},
					},
				})).To(Succeed())
			}

			JustBeforeEach(func() {
				b.Shoot.PriorExternalClusterDomain = new(priorExternalDomain)
				b.Shoot.PriorExternalDomain = &gardenerutils.Domain{Domain: priorExternalDomain, Provider: priorExternalProvider}
			})

			It("should destroy the record of the old domain, keeping its DNS entry, before it deploys the record of the new domain", func() {
				createExternalDNSRecord("api." + priorExternalDomain)

				gomock.InOrder(
					externalDNSRecord.EXPECT().Migrate(ctx),
					externalDNSRecord.EXPECT().WaitMigrate(ctx),
					externalDNSRecord.EXPECT().Deploy(ctx),
					externalDNSRecord.EXPECT().Wait(ctx),
				)

				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: shootName + "-external"}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
			})

			It("should only deploy if the record already has the new domain", func() {
				createExternalDNSRecord("api." + externalDomain)

				externalDNSRecord.EXPECT().Deploy(ctx)
				externalDNSRecord.EXPECT().Wait(ctx)

				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should only deploy if there is no record yet", func() {
				externalDNSRecord.EXPECT().Deploy(ctx)
				externalDNSRecord.EXPECT().Wait(ctx)

				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should fail if the migration of the old record fails", func() {
				createExternalDNSRecord("api." + priorExternalDomain)

				externalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)

				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("deploy", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Deploy(ctx)
				externalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				externalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("restore", func() {
			shootState := &gardencorev1beta1.ShootState{}

			JustBeforeEach(func() {
				b.Shoot.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
			})

			It("should call Restore and Wait and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Restore(ctx, shootState)
				externalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				externalDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (Shoot DNS is set to nil)", func() {
			JustBeforeEach(func() {
				b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: nil,
					},
				})
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Destroy(ctx)
				externalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				externalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DeployOrDestroyInternalDNSRecord", func() {
		Context("domain migration in progress", func() {
			createInternalDNSRecord := func(name string) {
				Expect(c.Create(ctx, &extensionsv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: shootName + "-internal", Namespace: controlPlaneNamespace},
					Spec: extensionsv1alpha1.DNSRecordSpec{
						Name:       name,
						RecordType: extensionsv1alpha1.DNSRecordTypeA,
						Values:     []string{address},
					},
				})).To(Succeed())
			}

			JustBeforeEach(func() {
				b.Shoot.PriorInternalClusterDomain = new(priorInternalDomain)
				b.Shoot.PriorInternalDomain = &gardenerutils.Domain{Domain: priorInternalDomain, Provider: priorInternalProvider}
			})

			It("should destroy the record of the old domain, keeping its DNS entry, before it deploys the record of the new domain", func() {
				createInternalDNSRecord("api." + priorInternalDomain)

				gomock.InOrder(
					internalDNSRecord.EXPECT().Migrate(ctx),
					internalDNSRecord.EXPECT().WaitMigrate(ctx),
					internalDNSRecord.EXPECT().Deploy(ctx),
					internalDNSRecord.EXPECT().Wait(ctx),
				)

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: shootName + "-internal"}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
			})

			It("should destroy the record of the old domain, keeping its DNS entry, if the internal domain is disabled", func() {
				createInternalDNSRecord("api." + priorInternalDomain)
				b.Shoot.InternalClusterDomain = nil
				b.Shoot.InternalDomain = nil

				gomock.InOrder(
					internalDNSRecord.EXPECT().Migrate(ctx),
					internalDNSRecord.EXPECT().WaitMigrate(ctx),
				)

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: shootName + "-internal"}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
			})

			It("should do nothing if the internal domain is disabled and there is no record", func() {
				b.Shoot.InternalClusterDomain = nil
				b.Shoot.InternalDomain = nil

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should delete the record of the old domain without migration and with its credentials during the shoot deletion", func() {
				createInternalDNSRecord("api." + priorInternalDomain)
				shoot := b.Shoot.GetInfo()
				shoot.DeletionTimestamp = &metav1.Time{}
				b.Shoot.SetInfo(shoot)
				b.Shoot.PriorInternalDomain.Credentials = &corev1.Secret{Data: map[string][]byte{"prior-internal-foo": []byte("prior-internal-bar")}}

				gomock.InOrder(
					internalDNSRecord.EXPECT().Deploy(ctx),
					internalDNSRecord.EXPECT().Wait(ctx),
				)

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: shootName + "-internal"}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: DNSRecordSecretPrefix + "-" + shootName + "-internal"}, secret)).To(Succeed())
				Expect(secret.Data).To(Equal(map[string][]byte{"prior-internal-foo": []byte("prior-internal-bar")}))
			})

			It("should only deploy if the record already has the new domain", func() {
				createInternalDNSRecord("api." + internalDomain)

				internalDNSRecord.EXPECT().Deploy(ctx)
				internalDNSRecord.EXPECT().Wait(ctx)

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should only deploy if there is no record yet", func() {
				internalDNSRecord.EXPECT().Deploy(ctx)
				internalDNSRecord.EXPECT().Wait(ctx)

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should fail if the migration of the old record fails", func() {
				createInternalDNSRecord("api." + priorInternalDomain)

				internalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)

				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("deploy", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Deploy(ctx)
				internalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				internalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("restore", func() {
			shootState := &gardencorev1beta1.ShootState{}

			JustBeforeEach(func() {
				b.Shoot.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
			})

			It("should call Restore and Wait and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Restore(ctx, shootState)
				internalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				internalDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (Shoot InternalDomain is set to nil)", func() {
			JustBeforeEach(func() {
				b.Shoot.InternalDomain = nil
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Destroy(ctx)
				internalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				internalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DeployOrDestroyPriorExternalDNSRecord", func() {
		Context("deploy", func() {
			JustBeforeEach(func() {
				b.Shoot.PriorExternalClusterDomain = new(priorExternalDomain)
				b.Shoot.PriorExternalDomain = &gardenerutils.Domain{Domain: priorExternalDomain, Provider: priorExternalProvider}
			})

			It("should call Deploy and Wait and succeed if they succeeded", func() {
				priorExternalDNSRecord.EXPECT().Deploy(ctx)
				priorExternalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyPriorExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				priorExternalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyPriorExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (no domain migration)", func() {
			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				priorExternalDNSRecord.EXPECT().Destroy(ctx)
				priorExternalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyPriorExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				priorExternalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyPriorExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DeployOrDestroyPriorInternalDNSRecord", func() {
		Context("deploy", func() {
			JustBeforeEach(func() {
				b.Shoot.PriorInternalClusterDomain = new(priorInternalDomain)
				b.Shoot.PriorInternalDomain = &gardenerutils.Domain{Domain: priorInternalDomain, Provider: priorInternalProvider}
			})

			It("should call Deploy and Wait and succeed if they succeeded", func() {
				priorInternalDNSRecord.EXPECT().Deploy(ctx)
				priorInternalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyPriorInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				priorInternalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyPriorInternalDNSRecord(ctx)).To(MatchError(testErr))
			})

			It("should call Restore and Wait in the restore phase", func() {
				shootState := &gardencorev1beta1.ShootState{}
				b.Shoot.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore},
				}

				priorInternalDNSRecord.EXPECT().Restore(ctx, shootState)
				priorInternalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyPriorInternalDNSRecord(ctx)).To(Succeed())
			})
		})

		Context("destroy (no domain migration)", func() {
			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				priorInternalDNSRecord.EXPECT().Destroy(ctx)
				priorInternalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyPriorInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				priorInternalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyPriorInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DestroyPriorInternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			priorInternalDNSRecord.EXPECT().Destroy(ctx)
			priorInternalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyPriorInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			priorInternalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyPriorInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigratePriorInternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			priorInternalDNSRecord.EXPECT().Migrate(ctx)
			priorInternalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigratePriorInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			priorInternalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigratePriorInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#DestroyPriorExternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			priorExternalDNSRecord.EXPECT().Destroy(ctx)
			priorExternalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyPriorExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			priorExternalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyPriorExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigratePriorExternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			priorExternalDNSRecord.EXPECT().Migrate(ctx)
			priorExternalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigratePriorExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			priorExternalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigratePriorExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#DestroyExternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			externalDNSRecord.EXPECT().Destroy(ctx)
			externalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			externalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
		})

		Context("domain migration in progress", func() {
			createExternalDNSRecord := func(name string) {
				Expect(c.Create(ctx, &extensionsv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: shootName + "-external", Namespace: controlPlaneNamespace},
					Spec:       extensionsv1alpha1.DNSRecordSpec{Name: name},
				})).To(Succeed())
			}

			JustBeforeEach(func() {
				b.Shoot.PriorExternalClusterDomain = new(priorExternalDomain)
				b.Shoot.PriorExternalDomain = &gardenerutils.Domain{
					Domain:      priorExternalDomain,
					Provider:    priorExternalProvider,
					Credentials: &corev1.Secret{Data: map[string][]byte{"prior-external-foo": []byte("prior-external-bar")}},
				}
			})

			It("should destroy the record of the prior domain with the credentials of the prior domain", func() {
				createExternalDNSRecord("api." + priorExternalDomain)

				Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: shootName + "-external"}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: DNSRecordSecretPrefix + "-" + shootName + "-external"}, secret)).To(Succeed())
				Expect(secret.Data).To(Equal(map[string][]byte{"prior-external-foo": []byte("prior-external-bar")}))
			})

			It("should destroy the record of the new domain with the component of the new domain", func() {
				createExternalDNSRecord("api." + externalDomain)

				externalDNSRecord.EXPECT().Destroy(ctx)
				externalDNSRecord.EXPECT().WaitCleanup(ctx)

				Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should destroy with the component of the new domain if there is no record", func() {
				externalDNSRecord.EXPECT().Destroy(ctx)
				externalDNSRecord.EXPECT().WaitCleanup(ctx)

				Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())
			})
		})
	})

	Describe("#DestroyInternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			internalDNSRecord.EXPECT().Destroy(ctx)
			internalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			internalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
		})

		Context("domain migration in progress", func() {
			createInternalDNSRecord := func(name string) {
				Expect(c.Create(ctx, &extensionsv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: shootName + "-internal", Namespace: controlPlaneNamespace},
					Spec:       extensionsv1alpha1.DNSRecordSpec{Name: name},
				})).To(Succeed())
			}

			JustBeforeEach(func() {
				b.Shoot.PriorInternalClusterDomain = new(priorInternalDomain)
				b.Shoot.PriorInternalDomain = &gardenerutils.Domain{
					Domain:      priorInternalDomain,
					Provider:    priorInternalProvider,
					Credentials: &corev1.Secret{Data: map[string][]byte{"prior-internal-foo": []byte("prior-internal-bar")}},
				}
			})

			It("should destroy the record of the prior domain with the credentials of the prior domain", func() {
				createInternalDNSRecord("api." + priorInternalDomain)

				Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: shootName + "-internal"}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: DNSRecordSecretPrefix + "-" + shootName + "-internal"}, secret)).To(Succeed())
				Expect(secret.Data).To(Equal(map[string][]byte{"prior-internal-foo": []byte("prior-internal-bar")}))
			})

			It("should destroy the record of the new domain with the component of the new domain", func() {
				createInternalDNSRecord("api." + internalDomain)

				internalDNSRecord.EXPECT().Destroy(ctx)
				internalDNSRecord.EXPECT().WaitCleanup(ctx)

				Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should destroy with the component of the new domain if there is no record", func() {
				internalDNSRecord.EXPECT().Destroy(ctx)
				internalDNSRecord.EXPECT().WaitCleanup(ctx)

				Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())
			})
		})
	})

	Describe("#MigrateExternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			externalDNSRecord.EXPECT().Migrate(ctx)
			externalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			externalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateInternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			internalDNSRecord.EXPECT().Migrate(ctx)
			internalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			internalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#WaitForLiveMigrationDNSCacheSync", func() {
		It("should return nil once the DNS cache sync wait duration has elapsed", func() {
			fakeClock := testclock.NewFakeClock(time.Now())
			botanist := &Botanist{Operation: &operation.Operation{Clock: fakeClock, Logger: logr.Discard()}}

			errCh := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				errCh <- botanist.WaitForLiveMigrationDNSCacheSync(ctx)
			}()

			Eventually(fakeClock.HasWaiters).Should(BeTrue())
			fakeClock.Step(2 * time.Minute)
			Eventually(errCh).Should(Receive(BeNil()))
		})

		It("should return ctx.Err() when the context is cancelled before the timer fires", func() {
			fakeClock := testclock.NewFakeClock(time.Now())
			botanist := &Botanist{Operation: &operation.Operation{Clock: fakeClock, Logger: logr.Discard()}}

			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			Expect(botanist.WaitForLiveMigrationDNSCacheSync(cancelCtx)).To(MatchError(context.Canceled))
		})
	})
})
