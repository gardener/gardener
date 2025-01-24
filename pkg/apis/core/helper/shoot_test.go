// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("Helper", func() {
	var (
		trueVar  = true
		falseVar = false
	)

	DescribeTable("#HibernationIsEnabled",
		func(shoot *core.Shoot, hibernated bool) {
			Expect(HibernationIsEnabled(shoot)).To(Equal(hibernated))
		},
		Entry("no hibernation section", &core.Shoot{}, false),
		Entry("hibernation.enabled = false", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &falseVar},
			},
		}, false),
		Entry("hibernation.enabled = true", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &trueVar},
			},
		}, true),
	)

	DescribeTable("#IsShootInHibernation",
		func(shoot *core.Shoot, hibernated bool) {
			Expect(IsShootInHibernation(shoot)).To(Equal(hibernated))
		},
		Entry("no hibernation section and status.isHibernated is false", &core.Shoot{}, false),
		Entry("no hibernation section and status.isHibernated is true", &core.Shoot{
			Status: core.ShootStatus{IsHibernated: true},
		}, true),
		Entry("hibernation.enabled = false and status.isHibernated is false", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &falseVar},
			},
		}, false),
		Entry("hibernation.enabled = false and status.isHibernated is true", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &falseVar},
			},
			Status: core.ShootStatus{
				IsHibernated: true,
			},
		}, true),
		Entry("hibernation.enabled = true", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &trueVar},
			},
		}, true),
	)

	var (
		unmanagedType = core.DNSUnmanaged
		differentType = "foo"
	)

	DescribeTable("#ShootUsesUnmanagedDNS",
		func(dns *core.DNS, expectation bool) {
			shoot := &core.Shoot{
				Spec: core.ShootSpec{
					DNS: dns,
				},
			}
			Expect(ShootUsesUnmanagedDNS(shoot)).To(Equal(expectation))
		},

		Entry("no dns", nil, false),
		Entry("no dns providers", &core.DNS{}, false),
		Entry("dns providers but no type", &core.DNS{Providers: []core.DNSProvider{{}}}, false),
		Entry("dns providers but different type", &core.DNS{Providers: []core.DNSProvider{{Type: &differentType}}}, false),
		Entry("dns providers and unmanaged type", &core.DNS{Providers: []core.DNSProvider{{Type: &unmanagedType}}}, true),
	)

	DescribeTable("#ShootNeedsForceDeletion",
		func(shoot *core.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(ShootNeedsForceDeletion(shoot)).To(match)
		},

		Entry("shoot is nil",
			nil,
			BeFalse()),
		Entry("no force-delete annotation present",
			&core.Shoot{},
			BeFalse()),
		Entry("force-delete annotation present but value is false",
			&core.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationConfirmationForceDeletion: "0"}}},
			BeFalse()),
		Entry("force-delete annotation present and value is true",
			&core.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationConfirmationForceDeletion: "t"}}},
			BeTrue()),
	)

	Describe("#IsHAControlPlaneConfigured", func() {
		var shoot *core.Shoot

		BeforeEach(func() {
			shoot = &core.Shoot{}
		})

		It("return false when HighAvailability is not set", func() {
			shoot.Spec.ControlPlane = &core.ControlPlane{}
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeFalse())
		})

		It("return false when ControlPlane is not set", func() {
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeFalse())
		})

		It("should return true when HighAvailability is set", func() {
			shoot.Spec.ControlPlane = &core.ControlPlane{
				HighAvailability: &core.HighAvailability{},
			}
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeTrue())
		})
	})

	Describe("#IsMultiZonalShootControlPlane", func() {
		var shoot *core.Shoot

		BeforeEach(func() {
			shoot = &core.Shoot{}
		})

		It("should return false when shoot has no ControlPlane Spec", func() {
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return false when shoot has no HighAvailability Spec", func() {
			shoot.Spec.ControlPlane = &core.ControlPlane{}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return false when shoot defines failure tolerance type 'node'", func() {
			shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return true when shoot defines failure tolerance type 'zone'", func() {
			shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeTrue())
		})
	})

	Describe("#IsWorkerless", func() {
		var shoot *core.Shoot

		BeforeEach(func() {
			shoot = &core.Shoot{
				Spec: core.ShootSpec{
					Provider: core.Provider{
						Workers: []core.Worker{
							{
								Name: "worker",
							},
						},
					},
				},
			}
		})

		It("should return false when shoot has workers", func() {
			Expect(IsWorkerless(shoot)).To(BeFalse())
		})

		It("should return true when shoot has zero workers", func() {
			shoot.Spec.Provider.Workers = nil
			Expect(IsWorkerless(shoot)).To(BeTrue())
		})
	})

	DescribeTable("#ShootEnablesSSHAccess",
		func(workers []core.Worker, workersSettings *core.WorkersSettings, expectedResult bool) {
			shoot := &core.Shoot{
				Spec: core.ShootSpec{
					Provider: core.Provider{
						Workers:         workers,
						WorkersSettings: workersSettings,
					},
				},
			}
			Expect(ShootEnablesSSHAccess(shoot)).To(Equal(expectedResult))
		},

		Entry("should return false when shoot provider has zero workers", []core.Worker{}, nil, false),
		Entry("should return true when shoot provider has no WorkersSettings", []core.Worker{{Name: "worker"}}, nil, true),
		Entry("should return true when shoot worker settings has no SSHAccess", []core.Worker{{Name: "worker"}}, &core.WorkersSettings{}, true),
		Entry("should return true when shoot worker settings has SSHAccess set to true", []core.Worker{{Name: "worker"}}, &core.WorkersSettings{SSHAccess: &core.SSHAccess{Enabled: true}}, true),
		Entry("should return false when shoot worker settings has SSHAccess set to false", []core.Worker{{Name: "worker"}}, &core.WorkersSettings{SSHAccess: &core.SSHAccess{Enabled: false}}, false),
	)

	DescribeTable("#GetShootCARotationPhase",
		func(credentials *core.ShootCredentials, expectedPhase core.CredentialsRotationPhase) {
			Expect(GetShootCARotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, core.CredentialsRotationPhase("")),
		Entry("rotation nil", &core.ShootCredentials{}, core.CredentialsRotationPhase("")),
		Entry("ca nil", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{}}, core.CredentialsRotationPhase("")),
		Entry("phase empty", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{CertificateAuthorities: &core.CARotation{}}}, core.CredentialsRotationPhase("")),
		Entry("phase set", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{CertificateAuthorities: &core.CARotation{Phase: core.RotationCompleting}}}, core.RotationCompleting),
	)

	DescribeTable("#GetShootServiceAccountKeyRotationPhase",
		func(credentials *core.ShootCredentials, expectedPhase core.CredentialsRotationPhase) {
			Expect(GetShootServiceAccountKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, core.CredentialsRotationPhase("")),
		Entry("rotation nil", &core.ShootCredentials{}, core.CredentialsRotationPhase("")),
		Entry("serviceAccountKey nil", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{}}, core.CredentialsRotationPhase("")),
		Entry("phase empty", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{ServiceAccountKey: &core.ServiceAccountKeyRotation{}}}, core.CredentialsRotationPhase("")),
		Entry("phase set", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{ServiceAccountKey: &core.ServiceAccountKeyRotation{Phase: core.RotationCompleting}}}, core.RotationCompleting),
	)

	DescribeTable("#GetShootETCDEncryptionKeyRotationPhase",
		func(credentials *core.ShootCredentials, expectedPhase core.CredentialsRotationPhase) {
			Expect(GetShootETCDEncryptionKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, core.CredentialsRotationPhase("")),
		Entry("rotation nil", &core.ShootCredentials{}, core.CredentialsRotationPhase("")),
		Entry("etcdEncryptionKey nil", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{}}, core.CredentialsRotationPhase("")),
		Entry("phase empty", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{}}}, core.CredentialsRotationPhase("")),
		Entry("phase set", &core.ShootCredentials{Rotation: &core.ShootCredentialsRotation{ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{Phase: core.RotationCompleting}}}, core.RotationCompleting),
	)

	Describe("#GetAllZonesFromShoot", func() {
		It("should return an empty list because there are no zones", func() {
			Expect(sets.List(GetAllZonesFromShoot(&core.Shoot{}))).To(BeEmpty())
		})

		It("should return the expected list when there is only one pool", func() {
			Expect(sets.List(GetAllZonesFromShoot(&core.Shoot{
				Spec: core.ShootSpec{
					Provider: core.Provider{
						Workers: []core.Worker{
							{Zones: []string{"a", "b"}},
						},
					},
				},
			}))).To(ConsistOf("a", "b"))
		})

		It("should return the expected list when there are more than one pools", func() {
			Expect(sets.List(GetAllZonesFromShoot(&core.Shoot{
				Spec: core.ShootSpec{
					Provider: core.Provider{
						Workers: []core.Worker{
							{Zones: []string{"a", "c"}},
							{Zones: []string{"b", "d"}},
						},
					},
				},
			}))).To(ConsistOf("a", "b", "c", "d"))
		})
	})

	Describe("GetShootAuditPolicyConfigMapName", func() {
		test := func(description string, config *core.KubeAPIServerConfig, expectedName string) {
			It(description, Offset(1), func() {
				Expect(GetShootAuditPolicyConfigMapName(config)).To(Equal(expectedName))
			})
		}

		test("KubeAPIServerConfig = nil", nil, "")
		test("AuditConfig = nil", &core.KubeAPIServerConfig{}, "")
		test("AuditPolicy = nil", &core.KubeAPIServerConfig{
			AuditConfig: &core.AuditConfig{},
		}, "")
		test("ConfigMapRef = nil", &core.KubeAPIServerConfig{
			AuditConfig: &core.AuditConfig{
				AuditPolicy: &core.AuditPolicy{},
			},
		}, "")
		test("ConfigMapRef set", &core.KubeAPIServerConfig{
			AuditConfig: &core.AuditConfig{
				AuditPolicy: &core.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{Name: "foo"},
				},
			},
		}, "foo")
	})

	Describe("GetShootAuditPolicyConfigMapRef", func() {
		test := func(description string, config *core.KubeAPIServerConfig, expectedRef *corev1.ObjectReference) {
			It(description, Offset(1), func() {
				Expect(GetShootAuditPolicyConfigMapRef(config)).To(Equal(expectedRef))
			})
		}

		test("KubeAPIServerConfig = nil", nil, nil)
		test("AuditConfig = nil", &core.KubeAPIServerConfig{}, nil)
		test("AuditPolicy = nil", &core.KubeAPIServerConfig{
			AuditConfig: &core.AuditConfig{},
		}, nil)
		test("ConfigMapRef = nil", &core.KubeAPIServerConfig{
			AuditConfig: &core.AuditConfig{
				AuditPolicy: &core.AuditPolicy{},
			},
		}, nil)
		test("ConfigMapRef set", &core.KubeAPIServerConfig{
			AuditConfig: &core.AuditConfig{
				AuditPolicy: &core.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{Name: "foo"},
				},
			},
		}, &corev1.ObjectReference{Name: "foo"})
	})

	DescribeTable("#GetShootAuthenticationConfigurationConfigMapName",
		func(kubeAPIServerConfig *core.KubeAPIServerConfig, expectedName string) {
			authConfigName := GetShootAuthenticationConfigurationConfigMapName(kubeAPIServerConfig)
			Expect(authConfigName).To(Equal(expectedName))
		},

		Entry("KubeAPIServerConfig = nil", nil, ""),
		Entry("StructuredAuthentication = nil", &core.KubeAPIServerConfig{}, ""),
		Entry("ConfigMapName not set", &core.KubeAPIServerConfig{
			StructuredAuthentication: &core.StructuredAuthentication{},
		}, ""),
		Entry("ConfigMapName set", &core.KubeAPIServerConfig{
			StructuredAuthentication: &core.StructuredAuthentication{
				ConfigMapName: "foo",
			},
		}, "foo"),
	)

	DescribeTable("#GetShootAuthorizationConfigurationConfigMapName",
		func(kubeAPIServerConfig *core.KubeAPIServerConfig, expectedName string) {
			authConfigName := GetShootAuthorizationConfigurationConfigMapName(kubeAPIServerConfig)
			Expect(authConfigName).To(Equal(expectedName))
		},

		Entry("KubeAPIServerConfig = nil", nil, ""),
		Entry("StructuredAuthorization = nil", &core.KubeAPIServerConfig{}, ""),
		Entry("ConfigMapName not set", &core.KubeAPIServerConfig{
			StructuredAuthorization: &core.StructuredAuthorization{},
		}, ""),
		Entry("ConfigMapName set", &core.KubeAPIServerConfig{
			StructuredAuthorization: &core.StructuredAuthorization{
				ConfigMapName: "foo",
			},
		}, "foo"),
	)

	DescribeTable("#GetShootServiceAccountConfigIssuer",
		func(kubeAPIServerConfig *core.KubeAPIServerConfig, expectedIssuer *string) {
			Issuer := GetShootServiceAccountConfigIssuer(kubeAPIServerConfig)
			Expect(Issuer).To(Equal(expectedIssuer))
		},

		Entry("KubeAPIServerConfig = nil", nil, nil),
		Entry("ServiceAccountConfig = nil", &core.KubeAPIServerConfig{}, nil),
		Entry("Issuer not set", &core.KubeAPIServerConfig{
			ServiceAccountConfig: &core.ServiceAccountConfig{},
		}, nil),
		Entry("Issuer set", &core.KubeAPIServerConfig{
			ServiceAccountConfig: &core.ServiceAccountConfig{
				Issuer: ptr.To("foo"),
			},
		}, ptr.To("foo")),
	)

	DescribeTable("#GetShootServiceAccountConfigAcceptedIssuers",
		func(kubeAPIServerConfig *core.KubeAPIServerConfig, expectedAcceptedIssuers []string) {
			AcceptedIssuers := GetShootServiceAccountConfigAcceptedIssuers(kubeAPIServerConfig)
			Expect(AcceptedIssuers).To(Equal(expectedAcceptedIssuers))
		},

		Entry("KubeAPIServerConfig = nil", nil, nil),
		Entry("ServiceAccountConfig = nil", &core.KubeAPIServerConfig{}, nil),
		Entry("AcceptedIssuers not set", &core.KubeAPIServerConfig{
			ServiceAccountConfig: &core.ServiceAccountConfig{},
		}, nil),
		Entry("AcceptedIssuers set", &core.KubeAPIServerConfig{
			ServiceAccountConfig: &core.ServiceAccountConfig{
				AcceptedIssuers: []string{"foo", "bar"},
			},
		}, []string{"foo", "bar"}),
	)

	Describe("#HasManagedIssuer", func() {
		It("should return false when the shoot does not have managed issuer", func() {
			Expect(HasManagedIssuer(&core.Shoot{})).To(BeFalse())
		})

		It("should return true when the shoot has managed issuer", func() {
			shoot := &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"authentication.gardener.cloud/issuer": "managed"},
				},
			}
			Expect(HasManagedIssuer(shoot)).To(BeTrue())
		})
	})

	DescribeTable("#KubernetesDashboardEnabled",
		func(addons *core.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(KubernetesDashboardEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("kubernetesDashboard nil", &core.Addons{}, BeFalse()),
		Entry("kubernetesDashboard disabled", &core.Addons{KubernetesDashboard: &core.KubernetesDashboard{Addon: core.Addon{Enabled: false}}}, BeFalse()),
		Entry("kubernetesDashboard enabled", &core.Addons{KubernetesDashboard: &core.KubernetesDashboard{Addon: core.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#NginxIngressEnabled",
		func(addons *core.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(NginxIngressEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("nginxIngress nil", &core.Addons{}, BeFalse()),
		Entry("nginxIngress disabled", &core.Addons{NginxIngress: &core.NginxIngress{Addon: core.Addon{Enabled: false}}}, BeFalse()),
		Entry("nginxIngress enabled", &core.Addons{NginxIngress: &core.NginxIngress{Addon: core.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#FindPrimaryDNSProvider",
		func(providers []core.DNSProvider, matcher gomegatypes.GomegaMatcher) {
			Expect(FindPrimaryDNSProvider(providers)).To(matcher)
		},

		Entry("no providers", nil, BeNil()),
		Entry("one non primary provider", []core.DNSProvider{{Type: ptr.To("provider")}}, BeNil()),
		Entry("one primary provider", []core.DNSProvider{{Type: ptr.To("provider"),
			Primary: ptr.To(true)}}, Equal(&core.DNSProvider{Type: ptr.To("provider"), Primary: ptr.To(true)})),
		Entry("multiple w/ one primary provider", []core.DNSProvider{
			{
				Type: ptr.To("provider2"),
			},
			{
				Type:    ptr.To("provider1"),
				Primary: ptr.To(true),
			},
			{
				Type: ptr.To("provider3"),
			},
		}, Equal(&core.DNSProvider{Type: ptr.To("provider1"), Primary: ptr.To(true)})),
		Entry("multiple w/ multiple primary providers", []core.DNSProvider{
			{
				Type:    ptr.To("provider1"),
				Primary: ptr.To(true),
			},
			{
				Type:    ptr.To("provider2"),
				Primary: ptr.To(true),
			},
			{
				Type: ptr.To("provider3"),
			},
		}, Equal(&core.DNSProvider{Type: ptr.To("provider1"), Primary: ptr.To(true)})),
	)

	DescribeTable("#CalculateEffectiveKubernetesVersion",
		func(controlPlaneVersion *semver.Version, workerKubernetes *core.WorkerKubernetes, expectedRes *semver.Version) {
			res, err := CalculateEffectiveKubernetesVersion(controlPlaneVersion, workerKubernetes)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expectedRes))
		},

		Entry("workerKubernetes = nil", semver.MustParse("1.2.3"), nil, semver.MustParse("1.2.3")),
		Entry("workerKubernetes.version = nil", semver.MustParse("1.2.3"), &core.WorkerKubernetes{}, semver.MustParse("1.2.3")),
		Entry("workerKubernetes.version != nil", semver.MustParse("1.2.3"), &core.WorkerKubernetes{Version: ptr.To("4.5.6")}, semver.MustParse("4.5.6")),
	)

	DescribeTable("#SystemComponentsAllowed",
		func(worker *core.Worker, allowsSystemComponents bool) {
			Expect(SystemComponentsAllowed(worker)).To(Equal(allowsSystemComponents))
		},
		Entry("no systemComponents section", &core.Worker{}, true),
		Entry("systemComponents.allowed = false", &core.Worker{SystemComponents: &core.WorkerSystemComponents{Allow: false}}, false),
		Entry("systemComponents.allowed = true", &core.Worker{SystemComponents: &core.WorkerSystemComponents{Allow: true}}, true),
	)

	DescribeTable("#GetResourceByName",
		func(resources []core.NamedResourceReference, name string, expected *core.NamedResourceReference) {
			actual := GetResourceByName(resources, name)
			Expect(actual).To(Equal(expected))
		},

		Entry("resources is nil", nil, "foo", nil),
		Entry("resources doesn't contain a resource with the given name",
			[]core.NamedResourceReference{
				{Name: "bar", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "bar"}},
				{Name: "baz", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "ConfigMap", Name: "baz"}},
			},
			"foo",
			nil,
		),
		Entry("resources contains a resource with the given name",
			[]core.NamedResourceReference{
				{Name: "bar", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "bar"}},
				{Name: "baz", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "ConfigMap", Name: "baz"}},
				{Name: "foo", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "foo"}},
			},
			"foo",
			&core.NamedResourceReference{Name: "foo", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "foo"}},
		),
	)

	DescribeTable("#AccessRestrictionsAreSupported",
		func(seedAccessRestrictions []core.AccessRestriction, shootAccessRestrictions []core.AccessRestrictionWithOptions, expectation bool) {
			Expect(AccessRestrictionsAreSupported(seedAccessRestrictions, shootAccessRestrictions)).To(Equal(expectation))
		},

		Entry("both have no access restrictions",
			nil,
			nil,
			true,
		),
		Entry("shoot has no access restrictions",
			[]core.AccessRestriction{{Name: "foo"}},
			nil,
			true,
		),
		Entry("seed has no access restrictions",
			nil,
			[]core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}},
			false,
		),
		Entry("both have access restrictions and they match",
			[]core.AccessRestriction{{Name: "foo"}},
			[]core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}},
			true,
		),
		Entry("both have access restrictions and they don't match",
			[]core.AccessRestriction{{Name: "bar"}},
			[]core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}},
			false,
		),
	)

	DescribeTable("#FindWorkerByName",
		func(workers []core.Worker, name string, expectedWorker *core.Worker) {
			Expect(FindWorkerByName(workers, name)).To(Equal(expectedWorker))
		},

		Entry("no workers", nil, "", nil),
		Entry("worker not found", []core.Worker{{Name: "foo"}}, "bar", nil),
		Entry("worker found", []core.Worker{{Name: "foo"}}, "foo", &core.Worker{Name: "foo"}),
	)
})
