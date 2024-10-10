// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement_test

import (
	"context"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/certmanagement"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CertManagement", func() {
	var (
		ctx = context.Background()

		namespace        = "some-namespace"
		image            = "some-image:some-tag"
		issuerSecretName = "issuer-secret"

		c       client.Client
		applier kubernetes.Applier
		values  Values

		consistOf func(...client.Object) types.GomegaMatcher
		contain   func(...client.Object) types.GomegaMatcher

		managedResourceIssuer       *resourcesv1alpha1.ManagedResource
		managedResourceIssuerSecret *corev1.Secret

		managedResourceDeployment       *resourcesv1alpha1.ManagedResource
		managedResourceDeploymentSecret *corev1.Secret
		serviceAccount                  *corev1.ServiceAccount
		clusterRole                     *rbacv1.ClusterRole
		clusterRoleBinding              *rbacv1.ClusterRoleBinding
		deployment                      *appsv1.Deployment
		role                            *rbacv1.Role
		roleBinding                     *rbacv1.RoleBinding

		issuer *certv1alpha1.Issuer

		newController = func(values Values) component.DeployWaiter {
			return New(c, values)
		}
		newDefaultIssuer = func(values Values) component.DeployWaiter {
			return NewIssuers(c, values)
		}

		checkIssuer     func()
		checkDeployment func(deploy *appsv1.Deployment)
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier = kubernetes.NewApplier(c, mapper)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
		contain = NewManagedResourceContainsObjectsMatcher(c)

		Expect(c.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "issuer-secret", Namespace: namespace},
			Type:       "Opaque",
			Data:       map[string][]byte{"privateKey": []byte("1234")},
		})).To(Succeed())

		values = Values{
			Image:     image,
			Namespace: namespace,
			DefaultIssuer: operatorv1alpha1.DefaultIssuer{
				ACME: &operatorv1alpha1.ACMEIssuer{
					Email:  "test@example.com",
					Server: "https://acme-v02.api.letsencrypt.org/directory",
					SecretRef: &corev1.LocalObjectReference{
						Name: issuerSecretName,
					},
				},
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-controller-manager",
				Namespace: namespace,
			},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cert-controller-manager",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"services"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingresses"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"gateway.networking.k8s.io"},
					Resources: []string{"gateways", "httproutes"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"networking.istio.io"},
					Resources: []string{"gateways", "virtualservices"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"get", "list", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{"cert.gardener.cloud"},
					Resources: []string{
						"issuers", "issuers/status",
						"certificates", "certificates/status",
						"certificaterevocations", "certificaterevocations/status",
					},
					Verbs: []string{"get", "list", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "update", "create", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cert-controller-manager",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cert-controller-manager",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "cert-controller-manager",
				Namespace: namespace,
			}},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-controller-manager",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"cert-controller-manager-controllers"},
					Verbs:         []string{"get", "watch", "update"},
				},
				{
					APIGroups: []string{"extensions.gardener.cloud"},
					Resources: []string{"dnsrecords"},
					Verbs:     []string{"get", "list", "update", "watch", "create", "delete", "patch"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      role.Name,
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/instance":                 "cert-management",
					"app.kubernetes.io/name":                     "cert-management",
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/instance": "cert-management",
						"app.kubernetes.io/name":     "cert-management",
					},
				},
				Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/instance":                     "cert-management",
							"app.kubernetes.io/name":                         "cert-management",
							"networking.gardener.cloud/to-dns":               "allowed",
							"networking.gardener.cloud/to-runtime-apiserver": "allowed",
							"networking.gardener.cloud/to-public-networks":   "allowed",
							"networking.gardener.cloud/to-private-networks":  "allowed",
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						Containers: []corev1.Container{{
							Name:            "cert-management",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--name=cert-controller-manager",
								"--dns-namespace=some-namespace",
								"--use-dnsrecords",
								"--issuer.issuer-namespace=some-namespace",
								"--server-port-http=8080",
								"--default-rsa-private-key-size=3072",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt32(8080),
										Scheme: "HTTP",
									},
								},
								InitialDelaySeconds: 30,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: 8080,
								Protocol:      corev1.ProtocolTCP,
							}},
						}},
					},
				},
			},
		}
	})

	issuer = &certv1alpha1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultIssuerName,
			Namespace: namespace,
		},
		Spec: certv1alpha1.IssuerSpec{
			ACME: &certv1alpha1.ACMESpec{
				Server: "https://acme-v02.api.letsencrypt.org/directory",
				Email:  "test@example.com",
				PrivateKeySecretRef: &corev1.SecretReference{
					Name:      "issuer-secret",
					Namespace: namespace,
				},
			},
		},
	}

	checkIssuer = func() {
		Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIssuer), managedResourceIssuer)).To(Succeed())
		expectedMrProvider := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cert-management-issuers",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To("seed"),
				SecretRefs: []corev1.LocalObjectReference{{
					Name: managedResourceIssuer.Spec.SecretRefs[0].Name,
				}},
				KeepObjects: ptr.To(false),
			},
		}
		utilruntime.Must(references.InjectAnnotations(expectedMrProvider))
		Expect(managedResourceIssuer).To(DeepEqual(expectedMrProvider))

		Expect(managedResourceIssuer).To(consistOf(issuer))
	}

	checkDeployment = func(deploy *appsv1.Deployment) {
		Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceDeployment), managedResourceDeployment)).To(Succeed())
		expectedMrDeployment := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cert-management-controller",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To("seed"),
				SecretRefs: []corev1.LocalObjectReference{{
					Name: managedResourceDeployment.Spec.SecretRefs[0].Name,
				}},
				KeepObjects: ptr.To(false),
			},
		}
		utilruntime.Must(references.InjectAnnotations(expectedMrDeployment))
		Expect(managedResourceDeployment).To(DeepEqual(expectedMrDeployment))

		managedResourceDeploymentSecret.Name = managedResourceDeployment.Spec.SecretRefs[0].Name
		Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceDeploymentSecret), managedResourceDeploymentSecret)).To(Succeed())
		Expect(managedResourceDeploymentSecret.Type).To(Equal(corev1.SecretTypeOpaque))
		Expect(managedResourceDeploymentSecret.Immutable).To(Equal(ptr.To(true)))
		Expect(managedResourceDeploymentSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

		expectedObjects := []client.Object{serviceAccount, clusterRole, clusterRoleBinding, role, roleBinding, deploy}
		Expect(managedResourceDeployment).To(contain(expectedObjects...))
	}

	JustBeforeEach(func() {
		managedResourceIssuer = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-management-issuers",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert-management",
				},
			},
		}
		managedResourceIssuerSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceIssuer.Name,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert-management",
				},
			},
		}
		managedResourceDeployment = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-management-controller",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert-management",
				},
			},
		}
		managedResourceDeploymentSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceDeployment.Name,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert-management",
				},
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy default issuer", func() {
			comp := newDefaultIssuer(values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIssuer), managedResourceIssuer)).To(BeNotFoundError())

			Expect(comp.Deploy(ctx)).To(Succeed())

			checkIssuer()
		})

		It("should successfully deploy controller", func() {
			comp := newController(values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceDeployment), managedResourceDeployment)).To(BeNotFoundError())

			Expect(comp.Deploy(ctx)).To(Succeed())

			checkDeployment(deployment)
		})

		It("should successfully deploy controller with caCertificates", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ca-certificates",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"bundle.crt": []byte("-----BEGIN CERTIFICATE-----\nXXX\n-----END CERTIFICATE-----"),
				},
			}
			Expect(c.Create(ctx, secret)).To(Succeed())
			values.DeployConfig = &operatorv1alpha1.CertManagementConfig{CACertificatesSecretRef: &corev1.LocalObjectReference{Name: "ca-certificates"}}
			comp := newController(values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceDeployment), managedResourceDeployment)).To(BeNotFoundError())

			Expect(comp.Deploy(ctx)).To(Succeed())

			deploy := *deployment
			container := &deploy.Spec.Template.Spec.Containers[0]
			container.Env = []corev1.EnvVar{
				{
					Name:  "LEGO_CA_SYSTEM_CERT_POOL",
					Value: "true",
				},
				{
					Name:  "LEGO_CA_CERTIFICATES",
					Value: "/var/run/cert-manager/certs/bundle.crt",
				},
			}
			container.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "ca-certificates",
					MountPath: "/var/run/cert-manager/certs",
					ReadOnly:  true,
				},
			}
			expectedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Name + "-79db81ac",
					Namespace: namespace,
					Labels: map[string]string{
						"resources.gardener.cloud/garbage-collectable-reference": "true",
					},
				},
				Immutable: ptr.To(true),
				Data:      secret.Data,
				Type:      secret.Type,
			}
			deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "ca-certificates",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: expectedSecret.Name,
						},
					},
				},
			}
			utilruntime.Must(references.InjectAnnotations(&deploy))

			checkDeployment(&deploy)

			Expect(managedResourceDeployment).To(contain(expectedSecret))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources of controller", func() {
			comp := newController(values)

			Expect(c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
			Expect(c.Create(ctx, managedResourceDeployment)).To(Succeed())
			Expect(c.Create(ctx, managedResourceDeploymentSecret)).To(Succeed())

			Expect(comp.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceDeployment), managedResourceDeployment)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceDeploymentSecret), managedResourceDeploymentSecret)).To(BeNotFoundError())
		})

		It("should successfully destroy all resources of default issuer", func() {
			comp := newDefaultIssuer(values)

			Expect(c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
			Expect(c.Create(ctx, managedResourceIssuer)).To(Succeed())
			Expect(c.Create(ctx, managedResourceIssuerSecret)).To(Succeed())

			Expect(comp.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIssuer), managedResourceIssuer)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIssuerSecret), managedResourceIssuerSecret)).To(BeNotFoundError())
		})

		It("should successfully destroy all resources of CRDs", func() {
			comp := NewCRDs(c, applier)

			Expect(comp.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Name: "certificaterevocations.cert.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "certificates.cert.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: "issuers.cert.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
		})
	})
})
