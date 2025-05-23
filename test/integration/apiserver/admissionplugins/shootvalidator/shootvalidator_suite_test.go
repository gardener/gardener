// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootvalidator_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestShootValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration APIServer AdmissionPlugins ShootValidator Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "shootvalidator-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	testNamespace     *corev1.Namespace
	cloudProfile      *gardencorev1beta1.CloudProfile
	seed              *gardencorev1beta1.Seed
	testSecret        *corev1.Secret
	testSecretBinding *gardencorev1beta1.SecretBinding

	roleAdmin         *rbacv1.Role
	roleBindingAdmin  *rbacv1.RoleBinding
	roleMember        *rbacv1.Role
	roleBindingMember *rbacv1.RoleBinding
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,SeedValidator",
			},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test clients")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			GenerateName: "garden-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create Project")
	project := &gardencorev1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
		Spec: gardencorev1beta1.ProjectSpec{
			Namespace: &testNamespace.Name,
		},
	}
	Expect(testClient.Create(ctx, project)).To(Succeed())
	log.Info("Created Project for test", "project", client.ObjectKeyFromObject(project))

	DeferCleanup(func() {
		By("Delete Project")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())
	})

	By("Create member role and RoleBinding")
	roleMember = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "project-member",
			Namespace: testNamespace.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.gardener.cloud"},
				Resources: []string{
					"shoots",
				},
				Verbs: []string{
					"create",
					"delete",
					"get",
				},
			},
		},
	}
	roleBindingMember = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "project-member",
			Namespace: testNamespace.Name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleMember.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "project:member",
			},
		},
	}
	Expect(testClient.Create(ctx, roleMember)).To(Succeed())
	Expect(testClient.Create(ctx, roleBindingMember)).To(Succeed())

	DeferCleanup(func() {
		By("Delete member role and RoleBinding")
		Expect(testClient.Delete(ctx, roleMember)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, roleBindingMember)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create admin role and RoleBinding")
	roleAdmin = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "project-admin",
			Namespace: testNamespace.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.gardener.cloud"},
				Resources: []string{
					"shoots",
				},
				Verbs: []string{
					"create",
					"delete",
					"get",
				},
			},
			{
				APIGroups: []string{"core.gardener.cloud"},
				Resources: []string{
					"shoots/binding",
				},
				Verbs: []string{
					"update",
				},
			},
		},
	}
	roleBindingAdmin = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "project-admin",
			Namespace: testNamespace.Name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleAdmin.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "project:admin",
			},
		},
	}
	Expect(testClient.Create(ctx, roleAdmin)).To(Succeed())
	Expect(testClient.Create(ctx, roleBindingAdmin)).To(Succeed())

	DeferCleanup(func() {
		By("Delete admin role and RoleBinding")
		Expect(testClient.Delete(ctx, roleAdmin)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, roleBindingAdmin)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create CloudProfile")
	cloudProfile = &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
		},
		Spec: gardencorev1beta1.CloudProfileSpec{
			Kubernetes: gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.31.1"}},
			},
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "some-OS",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1"},
							CRI: []gardencorev1beta1.CRI{
								{
									Name: gardencorev1beta1.CRINameContainerD,
								},
							},
						},
					},
				},
			},
			MachineTypes: []gardencorev1beta1.MachineType{{Name: "large"}},
			Regions:      []gardencorev1beta1.Region{{Name: "region"}},
			Type:         "providerType",
		},
	}
	Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
	log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

	DeferCleanup(func() {
		By("Delete CloudProfile")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
	})

	By("Create SecretBinding")
	testSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    testNamespace.Name,
		},
	}
	Expect(testClient.Create(ctx, testSecret)).To(Succeed())
	log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(testSecret))

	DeferCleanup(func() {
		By("Delete Secret")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, testSecret))).To(Succeed())
	})

	testSecretBinding = &gardencorev1beta1.SecretBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    testNamespace.Name,
		},
		Provider: &gardencorev1beta1.SecretBindingProvider{
			Type: "providerType",
		},
		SecretRef: corev1.SecretReference{
			Name:      testSecret.Name,
			Namespace: testSecret.Namespace,
		},
	}
	Expect(testClient.Create(ctx, testSecretBinding)).To(Succeed())
	log.Info("Created SecretBinding for test", "secretBinding", client.ObjectKeyFromObject(testSecretBinding))

	DeferCleanup(func() {
		By("Delete SecretBinding")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, testSecretBinding))).To(Succeed())
	})

	By("Create Seed")
	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: "region",
				Type:   "providerType",
			},
			Ingress: &gardencorev1beta1.Ingress{
				Domain: "seed.example.com",
				Controller: gardencorev1beta1.IngressController{
					Kind: "nginx",
				},
			},
			DNS: gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{
					Type: "provider",
					SecretRef: corev1.SecretReference{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
				},
			},
			Settings: &gardencorev1beta1.SeedSettings{
				Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
			},
			Networks: gardencorev1beta1.SeedNetworks{
				Pods:     "10.0.0.0/16",
				Services: "10.1.0.0/16",
				Nodes:    ptr.To("10.2.0.0/16"),
				ShootDefaults: &gardencorev1beta1.ShootNetworks{
					Pods:     ptr.To("100.128.0.0/11"),
					Services: ptr.To("100.72.0.0/13"),
				},
			},
		},
	}
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

	DeferCleanup(func() {
		By("Delete Seed")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
	})
})
