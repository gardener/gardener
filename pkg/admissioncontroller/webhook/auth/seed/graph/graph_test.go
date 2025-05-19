// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("graph", func() {
	var (
		ctx = context.TODO()

		fakeClient                            client.Client
		fakeInformerSeed                      *controllertest.FakeInformer
		fakeInformerShoot                     *controllertest.FakeInformer
		fakeInformerProject                   *controllertest.FakeInformer
		fakeInformerBackupBucket              *controllertest.FakeInformer
		fakeInformerBackupEntry               *controllertest.FakeInformer
		fakeInformerBastion                   *controllertest.FakeInformer
		fakeInformerSecretBinding             *controllertest.FakeInformer
		fakeInformerCredentialsBinding        *controllertest.FakeInformer
		fakeInformerControllerInstallation    *controllertest.FakeInformer
		fakeInformerManagedSeed               *controllertest.FakeInformer
		fakeInformerGardenlet                 *controllertest.FakeInformer
		fakeInformerCertificateSigningRequest *controllertest.FakeInformer
		fakeInformerServiceAccount            *controllertest.FakeInformer
		fakeInformers                         *informertest.FakeInformers

		log   logr.Logger
		graph *graph

		seed1                                     *gardencorev1beta1.Seed
		seed1BackupSecretCredentialsRef           = corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "seed1secret2", Name: "seed1secret2"}
		seed1BackupWorkloadIdentityCredentialsRef = corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Namespace: "seed1workloadidentity1", Name: "seed1workloadidentity1"}
		seed1DNSProviderSecretRef                 = corev1.SecretReference{Namespace: "seed1secret3", Name: "seed1secret3"}
		seed1SecretResourceRef                    = autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "resource1"}
		seed1LeaseNamespace                       = "gardener-system-seed-lease"

		shootIssuerNamespace = "gardener-system-shoot-issuer"

		shoot1                           *gardencorev1beta1.Shoot
		shoot1DNSProvider1               = gardencorev1beta1.DNSProvider{SecretName: ptr.To("dnssecret1")}
		shoot1DNSProvider2               = gardencorev1beta1.DNSProvider{SecretName: ptr.To("dnssecret2")}
		shoot1AuditPolicyConfigMapRef    = corev1.ObjectReference{Name: "auditpolicy1"}
		shoot1AuthnConfigConfigMapName   = "authentication-config"
		shoot1AuthzConfigConfigMapName   = "authorization-config"
		shoot1AuthzKubeconfigSecretName  = "authorization-config-authorizer-kubeconfig"
		shoot1Resource1                  = autoscalingv1.CrossVersionObjectReference{APIVersion: "foo", Kind: "bar", Name: "resource1"}
		shoot1Resource2                  = autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "resource2"}
		shoot1Resource3                  = autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "resource3"}
		shoot1SecretNameKubeconfig       string
		shoot1SecretNameCACluster        string
		shoot1SecretNameSSHKeypair       string
		shoot1SecretNameOldSSHKeypair    string
		shoot1SecretNameMonitoring       string
		shoot1SecretNameManagedIssuer    string
		shoot1InternalSecretNameCAClient string
		shoot1ConfigMapNameCACluster     string
		shoot1ConfigMapNameCAKubelet     string

		namespace1 *corev1.Namespace
		project1   *gardencorev1beta1.Project

		backupBucket1                   *gardencorev1beta1.BackupBucket
		backupBucket1SecretRef          = corev1.SecretReference{Namespace: "baz", Name: "foo"}
		backupBucket1GeneratedSecretRef = corev1.SecretReference{Namespace: "generated", Name: "secret"}

		backupEntry1 *gardencorev1beta1.BackupEntry

		bastion1 *operationsv1alpha1.Bastion

		secretBinding1          *gardencorev1beta1.SecretBinding
		secretBinding1SecretRef = corev1.SecretReference{Namespace: "foobar", Name: "bazfoo"}

		controllerInstallation1 *gardencorev1beta1.ControllerInstallation

		credentialsBinding1   *securityv1alpha1.CredentialsBinding
		credentialsBindingRef = corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "foobar", Name: "bazfoo"}

		seedConfig1 *gardenletconfigv1alpha1.SeedConfig
		seedConfig2 *gardenletconfigv1alpha1.SeedConfig

		managedSeed1                  *seedmanagementv1alpha1.ManagedSeed
		managedSeedBootstrapMode      = seedmanagementv1alpha1.BootstrapToken
		bootstrapTokenNamespace       = "kube-system"
		managedSeedBootstrapTokenName = "bootstrap-token-78f9fc"
		backupSecretCredentialsRef    = corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "backupsecret1ns", Name: "backupsecret1name"}

		gardenlet1                  *seedmanagementv1alpha1.Gardenlet
		gardenletBootstrapTokenName = "bootstrap-token-2a9d62"

		seedNameInCSR = "myseed"
		csr1          *certificatesv1.CertificateSigningRequest

		serviceAccount1Secret1 = "sa1secret1"
		serviceAccount1Secret2 = "sa1secret2"
		serviceAccount1        *corev1.ServiceAccount
	)

	BeforeEach(func() {
		scheme := kubernetes.GardenScheme
		Expect(metav1.AddMetaToScheme(scheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeInformerSeed = &controllertest.FakeInformer{}
		fakeInformerShoot = &controllertest.FakeInformer{}
		fakeInformerProject = &controllertest.FakeInformer{}
		fakeInformerBackupBucket = &controllertest.FakeInformer{}
		fakeInformerBackupEntry = &controllertest.FakeInformer{}
		fakeInformerBastion = &controllertest.FakeInformer{}
		fakeInformerSecretBinding = &controllertest.FakeInformer{}
		fakeInformerCredentialsBinding = &controllertest.FakeInformer{}
		fakeInformerControllerInstallation = &controllertest.FakeInformer{}
		fakeInformerManagedSeed = &controllertest.FakeInformer{}
		fakeInformerGardenlet = &controllertest.FakeInformer{}
		fakeInformerCertificateSigningRequest = &controllertest.FakeInformer{}
		fakeInformerServiceAccount = &controllertest.FakeInformer{}

		fakeInformers = &informertest.FakeInformers{
			Scheme: scheme,
			InformersByGVK: map[schema.GroupVersionKind]toolscache.SharedIndexInformer{
				gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"):                   fakeInformerSeed,
				gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"):                  fakeInformerShoot,
				gardencorev1beta1.SchemeGroupVersion.WithKind("Project"):                fakeInformerProject,
				gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"):           fakeInformerBackupBucket,
				gardencorev1beta1.SchemeGroupVersion.WithKind("BackupEntry"):            fakeInformerBackupEntry,
				operationsv1alpha1.SchemeGroupVersion.WithKind("Bastion"):               fakeInformerBastion,
				gardencorev1beta1.SchemeGroupVersion.WithKind("SecretBinding"):          fakeInformerSecretBinding,
				gardencorev1beta1.SchemeGroupVersion.WithKind("ControllerInstallation"): fakeInformerControllerInstallation,
				securityv1alpha1.SchemeGroupVersion.WithKind("CredentialsBinding"):      fakeInformerCredentialsBinding,
				seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed"):       fakeInformerManagedSeed,
				seedmanagementv1alpha1.SchemeGroupVersion.WithKind("Gardenlet"):         fakeInformerGardenlet,
				certificatesv1.SchemeGroupVersion.WithKind("CertificateSigningRequest"): fakeInformerCertificateSigningRequest,
				corev1.SchemeGroupVersion.WithKind("ServiceAccount"):                    fakeInformerServiceAccount,
			},
		}

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		graph = New(log, fakeClient)
		Expect(graph.Setup(ctx, fakeInformers)).To(Succeed())

		seed1 = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: "seed1"},
			Spec: gardencorev1beta1.SeedSpec{
				Backup: &gardencorev1beta1.Backup{
					CredentialsRef: &seed1BackupSecretCredentialsRef,
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						SecretRef: seed1DNSProviderSecretRef,
					},
				},
				Resources: []gardencorev1beta1.NamedResourceReference{
					{ResourceRef: seed1SecretResourceRef},
				},
			},
		}

		namespace1 = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace1",
				Labels: map[string]string{
					"project.gardener.cloud/name": "project1",
				},
			},
		}
		Expect(fakeClient.Create(ctx, namespace1)).To(Succeed())

		shoot1 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot1",
				Namespace: namespace1.Name,
				UID:       "f486b21b-7cbe-4bde-9b83-bf8a55c7f075",
				Annotations: map[string]string{
					"authentication.gardener.cloud/issuer": "managed",
				},
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName: ptr.To("cloudprofile1"),
				CloudProfile: &gardencorev1beta1.CloudProfileReference{
					Name: "cloudprofile1",
					Kind: "CloudProfile",
				},
				DNS: &gardencorev1beta1.DNS{
					Providers: []gardencorev1beta1.DNSProvider{shoot1DNSProvider1, shoot1DNSProvider2},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AuditConfig: &gardencorev1beta1.AuditConfig{
							AuditPolicy: &gardencorev1beta1.AuditPolicy{
								ConfigMapRef: &shoot1AuditPolicyConfigMapRef,
							},
						},
						StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
							ConfigMapName: shoot1AuthnConfigConfigMapName,
						},
						StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
							ConfigMapName: shoot1AuthzConfigConfigMapName,
							Kubeconfigs:   []gardencorev1beta1.AuthorizerKubeconfigReference{{SecretName: shoot1AuthzKubeconfigSecretName}},
						},
					},
				},
				Resources:              []gardencorev1beta1.NamedResourceReference{{ResourceRef: shoot1Resource1}, {ResourceRef: shoot1Resource2}, {ResourceRef: shoot1Resource3}},
				SecretBindingName:      ptr.To("secretbinding1"),
				CredentialsBindingName: ptr.To("credentialsbinding1"),
				SeedName:               &seed1.Name,
			},
		}
		shoot1SecretNameKubeconfig = shoot1.Name + ".kubeconfig"
		shoot1SecretNameCACluster = shoot1.Name + ".ca-cluster"
		shoot1ConfigMapNameCAKubelet = shoot1.Name + ".ca-kubelet"
		shoot1SecretNameSSHKeypair = shoot1.Name + ".ssh-keypair"
		shoot1SecretNameOldSSHKeypair = shoot1.Name + ".ssh-keypair.old"
		shoot1SecretNameMonitoring = shoot1.Name + ".monitoring"
		shoot1InternalSecretNameCAClient = shoot1.Name + ".ca-client"
		shoot1ConfigMapNameCACluster = shoot1.Name + ".ca-cluster"

		project1 = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "project1"},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To(namespace1.Name),
			},
		}

		shoot1SecretNameManagedIssuer = fmt.Sprintf("%s--%s", project1.Name, shoot1.UID)

		backupBucket1 = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{Name: "backupbucket1"},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SecretRef: backupBucket1SecretRef,
				SeedName:  &seed1.Name,
			},
			Status: gardencorev1beta1.BackupBucketStatus{
				GeneratedSecretRef: &backupBucket1GeneratedSecretRef,
			},
		}

		backupEntry1 = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backupentry1",
				Namespace: "backupentry1namespace",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
					Name:       shoot1.Name,
				}},
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: backupBucket1.Name,
				SeedName:   &seed1.Name,
			},
		}

		bastion1 = &operationsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{Name: "bastion1", Namespace: "bastion1namespace"},
			Spec: operationsv1alpha1.BastionSpec{
				SeedName: &seed1.Name,
			},
		}

		secretBinding1 = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "secretbinding1", Namespace: "sb1namespace"},
			SecretRef:  secretBinding1SecretRef,
		}

		controllerInstallation1 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{Name: "controllerinstallation1"},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				DeploymentRef:   &corev1.ObjectReference{Name: "controllerdeployment1"},
				RegistrationRef: corev1.ObjectReference{Name: "controllerregistration1"},
				SeedRef:         corev1.ObjectReference{Name: seed1.Name},
			},
		}

		credentialsBinding1 = &securityv1alpha1.CredentialsBinding{
			ObjectMeta:     metav1.ObjectMeta{Name: "credentialsBinding1", Namespace: "cb1namespace"},
			CredentialsRef: credentialsBindingRef,
		}

		seedConfig1 = &gardenletconfigv1alpha1.SeedConfig{
			SeedTemplate: gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.Backup{
						CredentialsRef: &backupSecretCredentialsRef,
					},
				},
			},
		}

		seedConfig2 = &gardenletconfigv1alpha1.SeedConfig{
			SeedTemplate: gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.Backup{
						CredentialsRef: &backupSecretCredentialsRef,
					},
				},
			},
		}

		managedSeed1 = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{Name: "managedseed1", Namespace: "managedseednamespace"},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{Name: shoot1.Name},
				Gardenlet: seedmanagementv1alpha1.GardenletConfig{
					Bootstrap: &managedSeedBootstrapMode,
					Config: runtime.RawExtension{
						Object: &gardenletconfigv1alpha1.GardenletConfiguration{
							SeedConfig: seedConfig1,
						},
					},
				},
			},
		}

		gardenlet1 = &seedmanagementv1alpha1.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{Name: seed1.Name, Namespace: "gardenletnamespace"},
			Spec: seedmanagementv1alpha1.GardenletSpec{
				Config: runtime.RawExtension{
					Object: &gardenletconfigv1alpha1.GardenletConfiguration{
						SeedConfig: seedConfig2,
					},
				},
			},
		}

		csr1 = &certificatesv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{Name: "csr1"},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Request: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIClzCCAX8CAQAwUjEkMCIGA1UEChMbZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNl
ZWRzMSowKAYDVQQDEyFnYXJkZW5lci5jbG91ZDpzeXN0ZW06c2VlZDpteXNlZWQw
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCzNgJWhogJrCSzAhKKmHkJ
FuooKAbxpWRGDOe5DiB8jPdgCoRCkZYnF7D9x9cDzliljA9IeBad3P3E9oegtSV/
sXFJYqb+lRuhJQ5oo2eBC6WRg+Oxglp+n7o7xt0bO7JHS977mqNrqsJ1d1FnJHTB
MPHPxqoqkgIbdW4t219ckSA20aWzC3PU7I7+Z9OD+YfuuYgzkWG541XyBBKVSD2w
Ix2yGu6zrslqZ1eVBZ4IoxpWrQNmLSMFQVnABThyEUi0U1eVtW0vPNwSnBf0mufX
Z0PpqAIPVjr64Z4s3HHml2GSu64iOxaG5wwb9qIPcdyFaQCep/sFh7kq1KjNI1Ql
AgMBAAGgADANBgkqhkiG9w0BAQsFAAOCAQEAb+meLvm7dgHpzhu0XQ39w41FgpTv
S7p78ABFwzDNcP1NwfrEUft0T/rUwPiMlN9zve2rRicaZX5Z7Bol/newejsu8H5z
OdotvtKjE7zBCMzwnXZwO/0pA0cuUFcAy50DPcr35gdGjGlzV9ogO+HPKPTieS3n
TRVg+MWlcLqCjALr9Y4N39DOzf4/SJts8AZJJ+lyyxnY3XIPXx7SdADwNWC8BX0U
OK8CwMwN3iiBQ4redVeMK7LU1unV899q/PWB+NXFcKVr+Grm/Kom5VxuhXSzcHEp
yO57qEcJqG1cB7iSchFuCSTuDBbZlN0fXgn4YjiWZyb4l3BDp3rm4iJImA==
-----END CERTIFICATE REQUEST-----`),
				Usages: []certificatesv1.KeyUsage{
					certificatesv1.UsageKeyEncipherment,
					certificatesv1.UsageDigitalSignature,
					certificatesv1.UsageClientAuth,
				},
			},
		}

		serviceAccount1 = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Namespace: "sa1ns", Name: gardenletbootstraputil.ServiceAccountNamePrefix + "sa1"},
			Secrets: []corev1.ObjectReference{
				{Name: serviceAccount1Secret1},
				{Name: serviceAccount1Secret2},
			},
		}
	})

	Describe("#HasVertex", func() {
		It("should return false", func() {
			Expect(graph.HasVertex(VertexTypeSeed, seed1.Namespace, seed1.Name)).To(BeFalse())
		})

		It("should return true", func() {
			fakeInformerSeed.Add(seed1)
			Expect(graph.HasVertex(VertexTypeSeed, seed1.Namespace, seed1.Name)).To(BeTrue())
		})
	})

	It("should behave as expected for gardencorev1beta1.Seed", func() {
		By("Add")
		fakeInformerSeed.Add(seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(7))
		Expect(graph.graph.Edges().Len()).To(Equal(6))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		seed1Copy := seed1.DeepCopy()
		seed1.Spec.Provider.Type = "providertype"
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(7))
		Expect(graph.graph.Edges().Len()).To(Equal(6))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("Update (backup credentials ref to workloadidentity)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.Backup.CredentialsRef = &seed1BackupWorkloadIdentityCredentialsRef
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(7))
		Expect(graph.graph.Edges().Len()).To(Equal(6))
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seed1BackupWorkloadIdentityCredentialsRef.Namespace, seed1BackupWorkloadIdentityCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("Update (remove backup secret ref)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.Backup = nil
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(6))
		Expect(graph.graph.Edges().Len()).To(Equal(5))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seed1BackupWorkloadIdentityCredentialsRef.Namespace, seed1BackupWorkloadIdentityCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("Update (remove DNS provider secret ref)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.DNS.Provider = nil
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seed1BackupWorkloadIdentityCredentialsRef.Namespace, seed1BackupWorkloadIdentityCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("Update (all secret refs)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.Backup = &gardencorev1beta1.Backup{CredentialsRef: &seed1BackupSecretCredentialsRef}
		seed1.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{SecretRef: seed1DNSProviderSecretRef}
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(7))
		Expect(graph.graph.Edges().Len()).To(Equal(6))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seed1BackupWorkloadIdentityCredentialsRef.Namespace, seed1BackupWorkloadIdentityCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerSeed.Delete(seed1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seed1BackupWorkloadIdentityCredentialsRef.Namespace, seed1BackupWorkloadIdentityCredentialsRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
	})

	It("should behave as expected for gardencorev1beta1.Shoot", func() {
		By("Add")
		fakeInformerShoot.Add(shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(24))
		Expect(graph.graph.Edges().Len()).To(Equal(23))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", *shoot1.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Add (with namespaced cloud profile)")
		shoot1Copy := shoot1.DeepCopy()
		shoot1Copy.Spec.CloudProfileName = nil
		shoot1Copy.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
			Kind: "NamespacedCloudProfile",
			Name: "namespaced-profile-1",
		}
		fakeInformerShoot.Add(shoot1Copy)
		Expect(graph.graph.Nodes().Len()).To(Equal(24))
		Expect(graph.graph.Edges().Len()).To(Equal(23))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", *shoot1.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, shoot1Copy.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Add (secret binding is nil)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1Copy.Spec.SecretBindingName = nil
		fakeInformerShoot.Add(shoot1Copy)
		Expect(graph.graph.Nodes().Len()).To(Equal(23))
		Expect(graph.graph.Edges().Len()).To(Equal(22))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", *shoot1.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Add (credentials binding is nil)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1Copy.Spec.CredentialsBindingName = nil
		fakeInformerShoot.Add(shoot1Copy)
		Expect(graph.graph.Nodes().Len()).To(Equal(23))
		Expect(graph.graph.Edges().Len()).To(Equal(22))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", *shoot1.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (cloud profile name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{Name: "foo", Kind: "CloudProfile"}
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(24))
		Expect(graph.graph.Edges().Len()).To(Equal(23))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", *shoot1Copy.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, *shoot1.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (namespaced cloud profile)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1Copy.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{Name: "namespaced-profile", Kind: "NamespacedCloudProfile"}
		fakeInformerShoot.Update(shoot1, shoot1Copy)
		Expect(graph.graph.Nodes().Len()).To(Equal(24))
		Expect(graph.graph.Edges().Len()).To(Equal(23))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", *shoot1Copy.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1Copy.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, *shoot1Copy.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, shoot1Copy.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (secret binding name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.SecretBindingName = ptr.To("bar")
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(24))
		Expect(graph.graph.Edges().Len()).To(Equal(23))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1Copy.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (credentials binding name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.CredentialsBindingName = ptr.To("bar")
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(24))
		Expect(graph.graph.Edges().Len()).To(Equal(23))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1Copy.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (audit policy config map name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(23))
		Expect(graph.graph.Edges().Len()).To(Equal(22))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (structured authentication config map name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(22))
		Expect(graph.graph.Edges().Len()).To(Equal(21))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (structured authorization kubeconfig secrets)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(21))
		Expect(graph.graph.Edges().Len()).To(Equal(20))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (structured authorization config map name and kubeconfig secrets)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(20))
		Expect(graph.graph.Edges().Len()).To(Equal(19))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (dns provider secrets)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.DNS = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(18))
		Expect(graph.graph.Edges().Len()).To(Equal(17))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (resources)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.Resources = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(16))
		Expect(graph.graph.Edges().Len()).To(Equal(15))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (no seed name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.SeedName = nil
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(15))
		Expect(graph.graph.Edges().Len()).To(Equal(14))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (new seed name)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Spec.SeedName = ptr.To("newseed")
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(16))
		Expect(graph.graph.Edges().Len()).To(Equal(15))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", "newseed")).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Update (new seed name in status)")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Status.SeedName = ptr.To("seed-in-status")
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(17))
		Expect(graph.graph.Edges().Len()).To(Equal(16))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", "newseed")).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", "seed-in-status")).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Remove managed issuer annotation")
		shoot1Copy = shoot1.DeepCopy()
		shoot1.Annotations = map[string]string{}
		fakeInformerShoot.Update(shoot1Copy, shoot1)
		Expect(graph.graph.Nodes().Len()).To(Equal(16))
		Expect(graph.graph.Edges().Len()).To(Equal(15))
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", "newseed")).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", "seed-in-status")).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerShoot.Delete(shoot1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeNamespacedCloudProfile, shoot1.Namespace, shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", "newseed")).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
	})

	It("should restore edges on recreating SecretBinding and CredentialsBinding for gardenercorev1beta1.Shoot", func() {
		By("Add")
		fakeInformerShoot.Add(shoot1)
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())

		By("Intermediate deletion of secret and credentials binding")
		credentialsBinding := &securityv1alpha1.CredentialsBinding{ObjectMeta: metav1.ObjectMeta{
			Name:      *shoot1.Spec.CredentialsBindingName,
			Namespace: shoot1.Namespace,
		}}
		secretBinding := &gardencorev1beta1.SecretBinding{ObjectMeta: metav1.ObjectMeta{
			Name:      *shoot1.Spec.SecretBindingName,
			Namespace: shoot1.Namespace,
		}}
		fakeInformerCredentialsBinding.Delete(credentialsBinding)
		fakeInformerSecretBinding.Delete(secretBinding)
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeFalse())
		fakeInformerCredentialsBinding.Add(credentialsBinding)
		fakeInformerSecretBinding.Add(secretBinding)
		fakeInformerShoot.Update(shoot1, shoot1)
		Expect(graph.HasPathFrom(VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name)).To(BeTrue())
	})

	It("should behave as expected for gardencorev1beta1.Project", func() {
		By("Add")
		fakeInformerProject.Add(project1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace)).To(BeTrue())

		By("Update (irrelevant change)")
		project1Copy := project1.DeepCopy()
		project1.Spec.Purpose = ptr.To("purpose")
		fakeInformerProject.Update(project1Copy, project1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace)).To(BeTrue())

		By("Update (namespace)")
		project1Copy = project1.DeepCopy()
		project1.Spec.Namespace = ptr.To("newnamespace")
		fakeInformerProject.Update(project1Copy, project1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1Copy.Spec.Namespace)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace)).To(BeTrue())

		By("Delete")
		fakeInformerProject.Delete(project1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1Copy.Spec.Namespace)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace)).To(BeFalse())
	})

	It("should behave as expected for gardencorev1beta1.BackupBucket", func() {
		By("Add")
		fakeInformerBackupBucket.Add(backupBucket1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1SecretRef.Namespace, backupBucket1SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName)).To(BeTrue())

		By("Update (irrelevant change)")
		backupBucket1Copy := backupBucket1.DeepCopy()
		backupBucket1.Spec.Provider.Type = "provider-type"
		fakeInformerBackupBucket.Update(backupBucket1Copy, backupBucket1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1SecretRef.Namespace, backupBucket1SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName)).To(BeTrue())

		By("Update (seed name)")
		backupBucket1Copy = backupBucket1.DeepCopy()
		backupBucket1.Spec.SeedName = ptr.To("newbbseed")
		fakeInformerBackupBucket.Update(backupBucket1Copy, backupBucket1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1SecretRef.Namespace, backupBucket1SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1Copy.Spec.SeedName)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName)).To(BeTrue())

		By("Update (secret ref)")
		backupBucket1Copy = backupBucket1.DeepCopy()
		backupBucket1.Spec.SecretRef = corev1.SecretReference{Namespace: "newsecretrefnamespace", Name: "newsecretrefname"}
		fakeInformerBackupBucket.Update(backupBucket1Copy, backupBucket1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1SecretRef.Namespace, backupBucket1SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1.Spec.SecretRef.Namespace, backupBucket1.Spec.SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName)).To(BeTrue())

		By("Update (generated secret ref)")
		backupBucket1Copy = backupBucket1.DeepCopy()
		backupBucket1.Status.GeneratedSecretRef = &corev1.SecretReference{Namespace: "newgeneratedsecretrefnamespace", Name: "newgeneratedsecretrefname"}
		fakeInformerBackupBucket.Update(backupBucket1Copy, backupBucket1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1.Spec.SecretRef.Namespace, backupBucket1.Spec.SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1.Status.GeneratedSecretRef.Namespace, backupBucket1.Status.GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName)).To(BeTrue())

		By("Delete")
		fakeInformerBackupBucket.Delete(backupBucket1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1.Spec.SecretRef.Namespace, backupBucket1.Spec.SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupBucket1.Status.GeneratedSecretRef.Namespace, backupBucket1.Status.GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName)).To(BeFalse())
	})

	It("should behave as expected for gardencorev1beta1.BackupEntry", func() {
		By("Add")
		fakeInformerBackupEntry.Add(backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeTrue())

		By("Update (irrelevant change)")
		backupEntry1Copy := backupEntry1.DeepCopy()
		backupEntry1.Labels = map[string]string{"foo": "bar"}
		fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeTrue())

		By("Update (seed name)")
		backupEntry1Copy = backupEntry1.DeepCopy()
		backupEntry1.Spec.SeedName = ptr.To("newbbseed")
		fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1Copy.Spec.SeedName)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeTrue())

		By("Update (new seed name in status)")
		backupEntry1Copy = backupEntry1.DeepCopy()
		backupEntry1.Status.SeedName = ptr.To("newbbseedinstatus")
		fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Status.SeedName)).To(BeTrue())

		By("Update (bucket name")
		backupEntry1Copy = backupEntry1.DeepCopy()
		backupEntry1.Spec.BucketName = "newbebucket"
		fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1Copy.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeTrue())

		By("Update (owner ref name")
		backupEntry1Copy = backupEntry1.DeepCopy()
		backupEntry1.OwnerReferences = nil
		fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeTrue())

		By("Delete")
		fakeInformerBackupEntry.Delete(backupEntry1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Status.SeedName)).To(BeFalse())
	})

	It("should behave as expected for operationsv1alpha1.Bastion", func() {
		By("Add")
		fakeInformerBastion.Add(bastion1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", *bastion1.Spec.SeedName)).To(BeTrue())

		By("Update (irrelevant change)")
		bastion1Copy := bastion1.DeepCopy()
		bastion1.Spec.SSHPublicKey = "foobar"
		fakeInformerBastion.Update(bastion1Copy, bastion1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", *bastion1.Spec.SeedName)).To(BeTrue())

		By("Update (seed name)")
		bastion1Copy = bastion1.DeepCopy()
		bastion1.Spec.SeedName = ptr.To("newseed")
		fakeInformerBastion.Update(bastion1Copy, bastion1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", *bastion1Copy.Spec.SeedName)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", *bastion1.Spec.SeedName)).To(BeTrue())

		By("Delete")
		fakeInformerBastion.Delete(bastion1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", *bastion1.Spec.SeedName)).To(BeFalse())
	})

	It("should behave as expected for gardencorev1beta1.SecretBinding", func() {
		By("Add")
		fakeInformerSecretBinding.Add(secretBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		secretBinding1Copy := secretBinding1.DeepCopy()
		secretBinding1.Quotas = []corev1.ObjectReference{{}, {}, {}}
		fakeInformerSecretBinding.Update(secretBinding1Copy, secretBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name)).To(BeTrue())

		By("Update (secretref)")
		secretBinding1Copy = secretBinding1.DeepCopy()
		secretBinding1.SecretRef = corev1.SecretReference{Namespace: "new-sb-secret-namespace", Name: "new-sb-secret-name"}
		fakeInformerSecretBinding.Update(secretBinding1Copy, secretBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, secretBinding1Copy.SecretRef.Namespace, secretBinding1Copy.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerSecretBinding.Delete(secretBinding1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name)).To(BeFalse())
	})

	It("should behave as expected for securityv1alpha1.CredentialsBinding referencing a Secret", func() {
		By("Add")
		fakeInformerCredentialsBinding.Add(credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		credentialsBinding1Copy := credentialsBinding1.DeepCopy()
		credentialsBinding1.Quotas = []corev1.ObjectReference{{}, {}, {}}
		fakeInformerCredentialsBinding.Update(credentialsBinding1Copy, credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeTrue())

		By("Update (credentialsref)")
		credentialsBinding1Copy = credentialsBinding1.DeepCopy()
		credentialsBinding1.CredentialsRef = corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "new-cb-secret-namespace", Name: "new-cb-secret-name"}
		fakeInformerCredentialsBinding.Update(credentialsBinding1Copy, credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, credentialsBinding1Copy.CredentialsRef.Namespace, credentialsBinding1Copy.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerCredentialsBinding.Delete(credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeFalse())
	})

	It("should behave as expected for securityv1alpha1.CredentialsBinding referencing a WorkloadIdentity", func() {
		credentialsBinding1.CredentialsRef.APIVersion = securityv1alpha1.SchemeGroupVersion.String()
		credentialsBinding1.CredentialsRef.Kind = "WorkloadIdentity"
		By("Add")
		fakeInformerCredentialsBinding.Add(credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		credentialsBinding1Copy := credentialsBinding1.DeepCopy()
		credentialsBinding1.Quotas = []corev1.ObjectReference{{}, {}, {}}
		fakeInformerCredentialsBinding.Update(credentialsBinding1Copy, credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeTrue())

		By("Update (credentialsref)")
		credentialsBinding1Copy = credentialsBinding1.DeepCopy()
		credentialsBinding1.CredentialsRef = corev1.ObjectReference{APIVersion: securityv1alpha1.SchemeGroupVersion.String(), Kind: "WorkloadIdentity", Namespace: "new-cb-secret-namespace", Name: "new-cb-secret-name"}
		fakeInformerCredentialsBinding.Update(credentialsBinding1Copy, credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, credentialsBinding1Copy.CredentialsRef.Namespace, credentialsBinding1Copy.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerCredentialsBinding.Delete(credentialsBinding1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, credentialsBinding1.CredentialsRef.Namespace, credentialsBinding1.CredentialsRef.Name, VertexTypeCredentialsBinding, credentialsBinding1.Namespace, credentialsBinding1.Name)).To(BeFalse())
	})

	It("should behave as expected for gardencorev1beta1.ControllerInstallation", func() {
		By("Add")
		fakeInformerControllerInstallation.Add(controllerInstallation1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		controllerInstallation1Copy := controllerInstallation1.DeepCopy()
		controllerInstallation1.Spec.RegistrationRef.ResourceVersion = "123"
		fakeInformerControllerInstallation.Update(controllerInstallation1Copy, controllerInstallation1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name)).To(BeTrue())

		By("Update (controller registration name)")
		controllerInstallation1Copy = controllerInstallation1.DeepCopy()
		controllerInstallation1.Spec.RegistrationRef.Name = "newreg"
		fakeInformerControllerInstallation.Update(controllerInstallation1Copy, controllerInstallation1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1Copy.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name)).To(BeTrue())

		By("Update (controller deployment name)")
		controllerInstallation1Copy = controllerInstallation1.DeepCopy()
		controllerInstallation1.Spec.DeploymentRef.Name = "newdeploy"
		fakeInformerControllerInstallation.Update(controllerInstallation1Copy, controllerInstallation1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1Copy.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name)).To(BeTrue())

		By("Update (seed name)")
		controllerInstallation1Copy = controllerInstallation1.DeepCopy()
		controllerInstallation1.Spec.SeedRef.Name = "newseed"
		fakeInformerControllerInstallation.Update(controllerInstallation1Copy, controllerInstallation1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1Copy.Spec.SeedRef.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name)).To(BeTrue())

		By("Delete")
		fakeInformerControllerInstallation.Delete(controllerInstallation1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name)).To(BeFalse())
	})

	It("should behave as expected for seedmanagementv1alpha1.ManagedSeed", func() {
		By("Add")
		fakeInformerManagedSeed.Add(managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		managedSeed1Copy := managedSeed1.DeepCopy()
		seedConfig1.Labels = map[string]string{"new": "labels"}
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())

		By("Update (shoot name)")
		managedSeed1Copy = managedSeed1.DeepCopy()
		managedSeed1.Spec.Shoot.Name = "newshoot"
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1Copy.Spec.Shoot.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())

		By("Update (backup credentials ref to new secret), seed exists")
		seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: managedSeed1.Name}}
		Expect(fakeClient.Create(ctx, seed)).To(Succeed())
		managedSeed1Copy = managedSeed1.DeepCopy()
		seedConfig1.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "new2", Name: "newaswell2"}
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())

		By("Update (backup credentials ref to workloadidentity), seed exists")
		seed = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: managedSeed1.Name}}
		Expect(fakeClient.Delete(ctx, seed)).To(Succeed())
		Expect(fakeClient.Create(ctx, seed)).To(Succeed())
		managedSeed1Copy = managedSeed1.DeepCopy()
		seedConfig1.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Namespace: "new2", Name: "newaswell2"}
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())

		By("Update (annotation), seed exists but with expired client cert")
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: managedSeed1.Name},
			Status:     gardencorev1beta1.SeedStatus{ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)}},
		}
		Expect(fakeClient.Delete(ctx, seed)).To(Succeed())
		Expect(fakeClient.Create(ctx, seed)).To(Succeed())
		managedSeed1Copy = managedSeed1.DeepCopy()
		managedSeed1.Annotations = map[string]string{"gardener.cloud/operation": "reconcile"}
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())

		By("Update (annotation), seed exists with non-expired client cert")
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: managedSeed1.Name},
			Status:     gardencorev1beta1.SeedStatus{ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(time.Hour)}},
		}
		Expect(fakeClient.Delete(ctx, seed)).To(Succeed())
		Expect(fakeClient.Create(ctx, seed)).To(Succeed())
		managedSeed1Copy = managedSeed1.DeepCopy()
		managedSeed1.Annotations = map[string]string{"gardener.cloud/operation": "reconcile-again"}
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())

		By("Update (renew-kubeconfig annotation), seed exists")
		managedSeed1Copy = managedSeed1.DeepCopy()
		managedSeed1.Annotations = map[string]string{"gardener.cloud/operation": "renew-kubeconfig"}
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(5))
		Expect(graph.graph.Edges().Len()).To(Equal(4))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())

		By("Update (bootstrap mode), seed does not exist")
		Expect(fakeClient.Delete(ctx, seed)).To(Succeed())
		managedSeed1Copy = managedSeed1.DeepCopy()
		newBootstrapMode := seedmanagementv1alpha1.BootstrapServiceAccount
		managedSeed1.Spec.Gardenlet.Bootstrap = &newBootstrapMode
		fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(6))
		Expect(graph.graph.Edges().Len()).To(Equal(5))
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeServiceAccount, managedSeed1.Namespace, "gardenlet-bootstrap-"+managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeClusterRoleBinding, "", "gardener.cloud:system:seed-bootstrapper:"+managedSeed1.Namespace+":gardenlet-bootstrap-"+managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerManagedSeed.Delete(managedSeed1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig1.Spec.Backup.CredentialsRef.Namespace, seedConfig1.Spec.Backup.CredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name)).To(BeFalse())
	})

	It("should behave as expected for seedmanagementv1alpha1.Gardenlet", func() {
		By("Add")
		fakeInformerGardenlet.Add(gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		gardenlet1Copy := gardenlet1.DeepCopy()
		seedConfig2.Labels = map[string]string{"new": "labels"}
		fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())

		By("Update (backup credentials ref to new secret)")
		gardenlet1Copy = gardenlet1.DeepCopy()
		seedConfig2.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "newcredentialsnamespaces", Name: "newcredentials"}
		fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seedConfig2.Spec.Backup.CredentialsRef.Namespace, seedConfig2.Spec.Backup.CredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())

		By("Update (backup credentials ref to workloadidentity)")
		gardenlet1Copy = gardenlet1.DeepCopy()
		seedConfig2.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Namespace: "newcredentialsnamespaces", Name: "newcredentials"}
		fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig2.Spec.Backup.CredentialsRef.Namespace, seedConfig2.Spec.Backup.CredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seedConfig2.Spec.Backup.CredentialsRef.Namespace, seedConfig2.Spec.Backup.CredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeFalse())

		By("Update (annotation), seed exists with expired client cert")
		seed := &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: gardenlet1.Name},
			Status:     gardencorev1beta1.SeedStatus{ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)}},
		}
		Expect(fakeClient.Create(ctx, seed)).To(Succeed())
		gardenlet1Copy = gardenlet1.DeepCopy()
		gardenlet1.Annotations = map[string]string{"gardener.cloud/operation": "reconcile"}
		fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig2.Spec.Backup.CredentialsRef.Namespace, seedConfig2.Spec.Backup.CredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, gardenletBootstrapTokenName, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())

		By("Update (annotation), seed exists with non-expired client cert")
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: gardenlet1.Name},
			Status:     gardencorev1beta1.SeedStatus{ClientCertificateExpirationTimestamp: &metav1.Time{Time: time.Now().Add(time.Hour)}},
		}
		Expect(fakeClient.Delete(ctx, seed)).To(Succeed())
		Expect(fakeClient.Create(ctx, seed)).To(Succeed())
		gardenlet1Copy = gardenlet1.DeepCopy()
		gardenlet1.Annotations = map[string]string{"gardener.cloud/operation": "reconcile-again"}
		fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig2.Spec.Backup.CredentialsRef.Namespace, seedConfig2.Spec.Backup.CredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, gardenletBootstrapTokenName, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeFalse())

		By("Update (renew-kubeconfig annotation)")
		gardenlet1Copy = gardenlet1.DeepCopy()
		gardenlet1.Annotations = map[string]string{"gardener.cloud/operation": "renew-kubeconfig"}
		fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(Equal(4))
		Expect(graph.graph.Edges().Len()).To(Equal(3))
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeWorkloadIdentity, seedConfig2.Spec.Backup.CredentialsRef.Namespace, seedConfig2.Spec.Backup.CredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, gardenletBootstrapTokenName, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerGardenlet.Delete(gardenlet1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, bootstrapTokenNamespace, gardenletBootstrapTokenName, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name)).To(BeFalse())
	})

	It("should behave as expected for certificatesv1.CertificateSigningRequest", func() {
		By("Add")
		fakeInformerCertificateSigningRequest.Add(csr1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeCertificateSigningRequest, "", csr1.Name, VertexTypeSeed, "", seedNameInCSR)).To(BeTrue())

		By("Delete")
		fakeInformerCertificateSigningRequest.Delete(csr1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeCertificateSigningRequest, "", csr1.Name, VertexTypeSeed, "", seedNameInCSR)).To(BeFalse())

		By("Add unrelated")
		csr1.Spec.Usages = nil
		fakeInformerCertificateSigningRequest.Add(csr1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeCertificateSigningRequest, "", csr1.Name, VertexTypeSeed, "", seedNameInCSR)).To(BeFalse())
	})

	It("should behave as expected for corev1.ServiceAccount", func() {
		By("Add")
		fakeInformerServiceAccount.Add(serviceAccount1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeTrue())

		By("Update (irrelevant change)")
		serviceAccount1Copy := serviceAccount1.DeepCopy()
		serviceAccount1.Labels = map[string]string{"foo": "bar"}
		fakeInformerServiceAccount.Update(serviceAccount1Copy, serviceAccount1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeTrue())

		By("Update (secrets)")
		serviceAccount1Copy = serviceAccount1.DeepCopy()
		serviceAccount1.Secrets = []corev1.ObjectReference{{Name: "newsasecret"}}
		fakeInformerServiceAccount.Update(serviceAccount1Copy, serviceAccount1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1.Secrets[0].Name, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeTrue())

		By("Delete")
		fakeInformerServiceAccount.Delete(serviceAccount1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name)).To(BeFalse())
	})

	It("should behave as expected with more objects modified in parallel", func() {
		var (
			nodes, edges int
			paths        = make(map[VertexType][]pathExpectation)
			wg           sync.WaitGroup
			lock         sync.Mutex
		)

		By("Create objects")
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerSeed.Add(seed1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+7, edges+6
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerShoot.Add(shoot1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+22, edges+23
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerProject.Add(project1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+2, edges+1
			paths[VertexTypeProject] = append(paths[VertexTypeProject], pathExpectation{VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerBackupBucket.Add(backupBucket1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+3, edges+3
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1SecretRef.Namespace, backupBucket1SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeTrue()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeTrue()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerBackupEntry.Add(backupEntry1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+2, edges+3
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, BeTrue()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerBastion.Add(bastion1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+1, edges+1
			paths[VertexTypeBastion] = append(paths[VertexTypeBastion], pathExpectation{VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerSecretBinding.Add(secretBinding1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+2, edges+1
			paths[VertexTypeSecretBinding] = append(paths[VertexTypeSecretBinding], pathExpectation{VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerControllerInstallation.Add(controllerInstallation1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+3, edges+3
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeTrue()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeTrue()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerManagedSeed.Add(managedSeed1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+5, edges+4
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerGardenlet.Add(gardenlet1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+1, edges+2
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerCertificateSigningRequest.Add(csr1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+2, edges+1
			paths[VertexTypeCertificateSigningRequest] = append(paths[VertexTypeCertificateSigningRequest], pathExpectation{VertexTypeCertificateSigningRequest, "", csr1.Name, VertexTypeSeed, "", seedNameInCSR, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerServiceAccount.Add(serviceAccount1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+3, edges+2
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeTrue()})
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeTrue()})
		}()
		wg.Wait()
		Expect(graph.graph.Nodes().Len()).To(Equal(nodes))
		Expect(graph.graph.Edges().Len()).To(Equal(edges))
		expectPaths(graph, edges, paths)

		By("Update some objects (1)")
		paths = make(map[VertexType][]pathExpectation)
		wg.Add(1)
		go func() {
			defer wg.Done()
			seed1Copy := seed1.DeepCopy()
			seed1.Spec.Provider.Type = "providertype"
			fakeInformerSeed.Update(seed1Copy, seed1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			shoot1Copy := shoot1.DeepCopy()
			shoot1.Spec.CloudProfile.Name = "foo"
			fakeInformerShoot.Update(shoot1Copy, shoot1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCloudProfile, "", shoot1Copy.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			project1Copy := project1.DeepCopy()
			project1.Spec.Namespace = ptr.To("newnamespace")
			fakeInformerProject.Update(project1Copy, project1)
			lock.Lock()
			defer lock.Unlock()
			nodes = nodes + 1
			paths[VertexTypeProject] = append(paths[VertexTypeProject], pathExpectation{VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1Copy.Spec.Namespace, BeFalse()})
			paths[VertexTypeProject] = append(paths[VertexTypeProject], pathExpectation{VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			backupBucket1Copy := backupBucket1.DeepCopy()
			backupBucket1.Spec.SecretRef = corev1.SecretReference{Namespace: "newsecretrefnamespace", Name: "newsecretrefname"}
			fakeInformerBackupBucket.Update(backupBucket1Copy, backupBucket1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1Copy.Spec.SecretRef.Namespace, backupBucket1Copy.Spec.SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeFalse()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeTrue()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1.Spec.SecretRef.Namespace, backupBucket1.Spec.SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeTrue()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			backupEntry1Copy := backupEntry1.DeepCopy()
			backupEntry1.Spec.SeedName = ptr.To("newbbseed")
			fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
			lock.Lock()
			defer lock.Unlock()
			nodes = nodes + 1
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, BeTrue()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1Copy.Spec.SeedName, BeFalse()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			bastion1Copy := bastion1.DeepCopy()
			bastion1.Spec.SSHPublicKey = "new-key"
			fakeInformerBastion.Update(bastion1Copy, bastion1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBastion] = append(paths[VertexTypeBastion], pathExpectation{VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			secretBinding1Copy := secretBinding1.DeepCopy()
			secretBinding1.Quotas = []corev1.ObjectReference{{}, {}, {}}
			fakeInformerSecretBinding.Update(secretBinding1Copy, secretBinding1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeSecretBinding] = append(paths[VertexTypeSecretBinding], pathExpectation{VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			controllerInstallation1Copy := controllerInstallation1.DeepCopy()
			controllerInstallation1.Spec.RegistrationRef.Name = "newreg"
			fakeInformerControllerInstallation.Update(controllerInstallation1Copy, controllerInstallation1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerRegistration, "", controllerInstallation1Copy.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeFalse()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeTrue()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeTrue()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			managedSeed1Copy := managedSeed1.DeepCopy()
			managedSeed1.Spec.Shoot.Name = "newshoot"
			fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1Copy.Spec.Shoot.Name, BeFalse()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			gardenlet1Copy := gardenlet1.DeepCopy()
			seedConfig2.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "newsecretnamespace", Name: "newsecretname"}
			fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
			lock.Lock()
			defer lock.Unlock()
			nodes = nodes + 1
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeSecret, "newsecretnamespace", "newsecretname", VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, BeTrue()})
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerCertificateSigningRequest.Delete(csr1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes-2, edges-1
			paths[VertexTypeCertificateSigningRequest] = append(paths[VertexTypeCertificateSigningRequest], pathExpectation{VertexTypeCertificateSigningRequest, csr1.Namespace, csr1.Name, VertexTypeSeed, "", seedNameInCSR, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			serviceAccount1Copy := serviceAccount1.DeepCopy()
			serviceAccount1.Secrets = []corev1.ObjectReference{{Name: "newsasecret"}}
			fakeInformerServiceAccount.Update(serviceAccount1Copy, serviceAccount1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes-1, edges-1
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeFalse()})
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeFalse()})
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1.Secrets[0].Name, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeTrue()})
		}()
		wg.Wait()
		Expect(graph.graph.Nodes().Len()).To(Equal(nodes), "node count")
		Expect(graph.graph.Edges().Len()).To(Equal(edges), "edge count")
		expectPaths(graph, edges, paths)

		By("Update some objects (2)")
		paths = make(map[VertexType][]pathExpectation)
		wg.Add(1)
		go func() {
			defer wg.Done()
			seed1Copy := seed1.DeepCopy()
			seed1.Spec.Backup = nil
			seed1.Spec.Resources = append(seed1.Spec.Resources, gardencorev1beta1.NamedResourceReference{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "resource2"}})
			fakeInformerSeed.Update(seed1Copy, seed1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, "garden", seed1SecretResourceRef.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeConfigMap, "garden", "resource2", VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			shoot1Copy := shoot1.DeepCopy()
			shoot1.Spec.Kubernetes.KubeAPIServer = nil
			fakeInformerShoot.Update(shoot1Copy, shoot1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes-4, edges-4
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCloudProfile, "", shoot1.Spec.CloudProfile.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shootIssuerNamespace, shoot1SecretNameManagedIssuer, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCAKubelet, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			project1Copy := project1.DeepCopy()
			project1.Spec.Purpose = ptr.To("purpose")
			fakeInformerProject.Update(project1Copy, project1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeProject] = append(paths[VertexTypeProject], pathExpectation{VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			backupBucket1Copy := backupBucket1.DeepCopy()
			backupBucket1.Spec.SeedName = ptr.To("newbbseed")
			fakeInformerBackupBucket.Update(backupBucket1Copy, backupBucket1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1.Spec.SecretRef.Namespace, backupBucket1.Spec.SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeTrue()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeTrue()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1Copy.Spec.SeedName, BeFalse()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			backupEntry1Copy := backupEntry1.DeepCopy()
			backupEntry1.Spec.BucketName = "newbebucket"
			fakeInformerBackupEntry.Update(backupEntry1Copy, backupEntry1)
			lock.Lock()
			defer lock.Unlock()
			nodes = nodes + 1
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupBucket, "", backupEntry1Copy.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, BeFalse()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, BeTrue()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name, BeTrue()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			bastion1Copy := bastion1.DeepCopy()
			bastion1.Spec.SSHPublicKey = "another-new-key"
			fakeInformerBastion.Update(bastion1Copy, bastion1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBastion] = append(paths[VertexTypeBastion], pathExpectation{VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			secretBinding1Copy := secretBinding1.DeepCopy()
			secretBinding1.SecretRef = corev1.SecretReference{Namespace: "new-sb-secret-namespace", Name: "new-sb-secret-name"}
			fakeInformerSecretBinding.Update(secretBinding1Copy, secretBinding1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeSecretBinding] = append(paths[VertexTypeSecretBinding], pathExpectation{VertexTypeSecret, secretBinding1Copy.SecretRef.Namespace, secretBinding1Copy.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name, BeFalse()})
			paths[VertexTypeSecretBinding] = append(paths[VertexTypeSecretBinding], pathExpectation{VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			controllerInstallation1Copy := controllerInstallation1.DeepCopy()
			controllerInstallation1.Spec.RegistrationRef.ResourceVersion = "123"
			fakeInformerControllerInstallation.Update(controllerInstallation1Copy, controllerInstallation1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerDeployment, "", controllerInstallation1.Spec.DeploymentRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeTrue()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeTrue()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			managedSeed1Copy := managedSeed1.DeepCopy()
			seedConfig1.Labels = map[string]string{"new": "labels"}
			fakeInformerManagedSeed.Update(managedSeed1Copy, managedSeed1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			gardenlet1Copy := gardenlet1.DeepCopy()
			seedConfig2.Labels = map[string]string{"new": "labels"}
			fakeInformerGardenlet.Update(gardenlet1Copy, gardenlet1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name, BeTrue()})
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeSecret, "newsecretnamespace", "newsecretname", VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerCertificateSigningRequest.Add(csr1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes+2, edges+1
			paths[VertexTypeCertificateSigningRequest] = append(paths[VertexTypeCertificateSigningRequest], pathExpectation{VertexTypeCertificateSigningRequest, "", csr1.Name, VertexTypeSeed, "", seedNameInCSR, BeTrue()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			serviceAccount1Copy := serviceAccount1.DeepCopy()
			serviceAccount1.Secrets = []corev1.ObjectReference{{Name: "newsasecret2"}}
			fakeInformerServiceAccount.Update(serviceAccount1Copy, serviceAccount1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeFalse()})
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeFalse()})
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1.Secrets[0].Name, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeTrue()})
		}()
		wg.Wait()
		Expect(graph.graph.Nodes().Len()).To(Equal(nodes), "node count")
		Expect(graph.graph.Edges().Len()).To(Equal(edges), "edge count")
		expectPaths(graph, edges, paths)

		By("Delete all objects")
		paths = make(map[VertexType][]pathExpectation)
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerSeed.Delete(seed1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1BackupSecretCredentialsRef.Namespace, seed1BackupSecretCredentialsRef.Name, VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeSecret, seed1DNSProviderSecretRef.Namespace, seed1DNSProviderSecretRef.Name, VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed1.Name), VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeConfigMap, "kube-system", "cluster-identity", VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeSeed] = append(paths[VertexTypeSeed], pathExpectation{VertexTypeLease, seed1LeaseNamespace, seed1.Name, VertexTypeSeed, "", seed1.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerShoot.Delete(shoot1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeNamespace, "", shoot1.Namespace, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCloudProfile, "", *shoot1.Spec.CloudProfileName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecretBinding, shoot1.Namespace, *shoot1.Spec.SecretBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeCredentialsBinding, shoot1.Namespace, *shoot1.Spec.CredentialsBindingName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuditPolicyConfigMapRef.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthnConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1AuthzConfigConfigMapName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1AuthzKubeconfigSecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider1.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, *shoot1DNSProvider2.SecretName, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1Resource2.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1Resource3.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameKubeconfig, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameOldSSHKeypair, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeSecret, shoot1.Namespace, shoot1SecretNameMonitoring, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeInternalSecret, shoot1.Namespace, shoot1InternalSecretNameCAClient, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeConfigMap, shoot1.Namespace, shoot1ConfigMapNameCACluster, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShoot, shoot1.Namespace, shoot1.Name, VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeShoot] = append(paths[VertexTypeShoot], pathExpectation{VertexTypeShootState, shoot1.Namespace, shoot1.Name, VertexTypeShoot, shoot1.Namespace, shoot1.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerProject.Delete(project1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeProject] = append(paths[VertexTypeProject], pathExpectation{VertexTypeProject, "", project1.Name, VertexTypeNamespace, "", *project1.Spec.Namespace, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerBackupBucket.Delete(backupBucket1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1SecretRef.Namespace, backupBucket1SecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeFalse()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeSecret, backupBucket1GeneratedSecretRef.Namespace, backupBucket1GeneratedSecretRef.Name, VertexTypeBackupBucket, "", backupBucket1.Name, BeFalse()})
			paths[VertexTypeBackupBucket] = append(paths[VertexTypeBackupBucket], pathExpectation{VertexTypeBackupBucket, "", backupBucket1.Name, VertexTypeSeed, "", *backupBucket1.Spec.SeedName, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerBackupEntry.Delete(backupEntry1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupBucket, "", backupEntry1.Spec.BucketName, VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, BeFalse()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeShoot, backupEntry1.Namespace, shoot1.Name, BeFalse()})
			paths[VertexTypeBackupEntry] = append(paths[VertexTypeBackupEntry], pathExpectation{VertexTypeBackupEntry, backupEntry1.Namespace, backupEntry1.Name, VertexTypeSeed, "", *backupEntry1.Spec.SeedName, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerBastion.Delete(bastion1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeBastion] = append(paths[VertexTypeBastion], pathExpectation{VertexTypeBastion, bastion1.Namespace, bastion1.Name, VertexTypeSeed, "", *bastion1.Spec.SeedName, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerSecretBinding.Delete(secretBinding1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeSecretBinding] = append(paths[VertexTypeSecretBinding], pathExpectation{VertexTypeSecret, secretBinding1.SecretRef.Namespace, secretBinding1.SecretRef.Name, VertexTypeSecretBinding, secretBinding1.Namespace, secretBinding1.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerControllerInstallation.Delete(controllerInstallation1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerRegistration, "", controllerInstallation1.Spec.RegistrationRef.Name, VertexTypeControllerInstallation, "", controllerInstallation1.Name, BeFalse()})
			paths[VertexTypeControllerInstallation] = append(paths[VertexTypeControllerInstallation], pathExpectation{VertexTypeControllerInstallation, "", controllerInstallation1.Name, VertexTypeSeed, "", controllerInstallation1.Spec.SeedRef.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerManagedSeed.Delete(managedSeed1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSeed, "", managedSeed1.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeFalse()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, VertexTypeShoot, managedSeed1.Namespace, managedSeed1.Spec.Shoot.Name, BeFalse()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeFalse()})
			paths[VertexTypeManagedSeed] = append(paths[VertexTypeManagedSeed], pathExpectation{VertexTypeSecret, bootstrapTokenNamespace, managedSeedBootstrapTokenName, VertexTypeManagedSeed, managedSeed1.Namespace, managedSeed1.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerGardenlet.Delete(gardenlet1)
			lock.Lock()
			defer lock.Unlock()
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, VertexTypeSeed, "", seed1.Name, BeFalse()})
			paths[VertexTypeGardenlet] = append(paths[VertexTypeGardenlet], pathExpectation{VertexTypeSecret, backupSecretCredentialsRef.Namespace, backupSecretCredentialsRef.Name, VertexTypeGardenlet, gardenlet1.Namespace, gardenlet1.Name, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerCertificateSigningRequest.Delete(csr1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes-2, edges-1
			paths[VertexTypeCertificateSigningRequest] = append(paths[VertexTypeCertificateSigningRequest], pathExpectation{VertexTypeCertificateSigningRequest, csr1.Namespace, csr1.Name, VertexTypeSeed, "", seedNameInCSR, BeFalse()})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			fakeInformerServiceAccount.Delete(serviceAccount1)
			lock.Lock()
			defer lock.Unlock()
			nodes, edges = nodes-3, edges-2
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret1, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeFalse()})
			paths[VertexTypeServiceAccount] = append(paths[VertexTypeServiceAccount], pathExpectation{VertexTypeSecret, serviceAccount1.Namespace, serviceAccount1Secret2, VertexTypeServiceAccount, serviceAccount1.Namespace, serviceAccount1.Name, BeFalse()})
		}()
		wg.Wait()
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		expectPaths(graph, 0, paths)
	})
})

type pathExpectation struct {
	fromType      VertexType
	fromNamespace string
	fromName      string
	toType        VertexType
	toNamespace   string
	toName        string
	matcher       gomegatypes.GomegaMatcher
}

func expectPaths(graph *graph, edges int, paths map[VertexType][]pathExpectation) {
	var pathsCount int

	for vertexType, expectation := range paths {
		By(fmt.Sprintf("validating path expectations for %s (%d expected paths)", vertexTypes[vertexType], len(expectation)))
		for _, p := range expectation {
			switch p.matcher.(type) {
			case *matchers.BeTrueMatcher:
				pathsCount++
				By(fmt.Sprintf("expect edge from %s to %s", newVertex(p.fromType, p.fromNamespace, p.fromName, 0), newVertex(p.toType, p.toNamespace, p.toName, 0)))
			}

			ExpectWithOffset(1, graph.HasPathFrom(p.fromType, p.fromNamespace, p.fromName, p.toType, p.toNamespace, p.toName)).To(p.matcher, fmt.Sprintf("path expectation from %s:%s/%s to %s:%s/%s", vertexTypes[p.fromType], p.fromNamespace, p.fromName, vertexTypes[p.toType], p.toNamespace, p.toName))
		}
	}

	edgeIterator := graph.graph.Edges()
	for edgeIterator.Next() {
		if edge := edgeIterator.Edge(); edge != nil {
			By(fmt.Sprintf("found edge from %s to %s", edge.From(), edge.To()))
		}
	}

	ExpectWithOffset(1, pathsCount).To(BeNumerically(">=", edges), "paths equals edges")
}
