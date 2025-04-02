// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plutono_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	comp "github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/plutono"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Plutono", func() {
	var (
		ctx = context.Background()

		managedResourceName     = "plutono"
		namespace               = "some-namespace"
		image                   = "some-image:some-tag"
		imageDashboardRefresher = "some-other-image:some-other-tag"

		c                 client.Client
		component         comp.DeployWaiter
		fakeSecretManager secretsmanager.Interface
		values            Values

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)

		values = Values{
			Image:                   image,
			ImageDashboardRefresher: imageDashboardRefresher,
			Replicas:                int32(1),
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "observability-ingress", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "observability-ingress-users", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		var (
			manifests []string

			plutonoConfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plutono-config-fd97f886",
					Namespace: namespace,
					Labels: map[string]string{
						"component": "plutono",
						"resources.gardener.cloud/garbage-collectable-reference": "true",
					},
				},
				Type:      corev1.SecretTypeOpaque,
				Immutable: ptr.To(true),
				Data: map[string][]byte{
					"plutono.ini": []byte(`[auth.basic]
enabled = true
[security]
admin_user = admin
admin_password = ________________________________`),
				},
			}

			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plutono",
					Namespace: namespace,
					Labels:    map[string]string{"component": "plutono"},
				},
				AutomountServiceAccountToken: ptr.To(false),
			}

			role = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plutono-dashboard-refresher",
					Namespace: namespace,
					Labels:    map[string]string{"component": "plutono"},
				},
				Rules: []rbacv1.PolicyRule{{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"get", "list", "watch"},
				}},
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plutono-dashboard-refresher",
					Namespace: namespace,
					Labels:    map[string]string{"component": "plutono"},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     "plutono-dashboard-refresher",
				},
				Subjects: []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Name:      "plutono",
					Namespace: namespace,
				}},
			}

			providerConfigMapYAML = `apiVersion: v1
data:
  default.yaml: |-
    apiVersion: 1
    providers:
    - name: 'default'
      orgId: 1
      folder: ''
      type: file
      disableDeletion: false
      editable: false
      options:
        path: /var/lib/plutono/dashboards
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    component: plutono
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: plutono-dashboard-providers-140e41f3
  namespace: some-namespace
`

			dataSourceConfigMapYAMLFor = func(values Values) string {
				url, maxLine := "http://prometheus-shoot:80", "1000"
				if values.IsGardenCluster {
					url, maxLine = "http://prometheus-garden:80", "5000"
				} else if values.ClusterType == comp.ClusterTypeSeed {
					url, maxLine = "http://prometheus-aggregate:80", "5000"
				}

				configMapData := `apiVersion: 1

    # list of datasources that should be deleted from the database
    deleteDatasources:
    - name: Graphite
      orgId: 1

    # list of datasources to insert/update depending
    # whats available in the database
    datasources:
`
				configMapData += `    - name: prometheus
      type: prometheus
      access: proxy
      url: ` + url + `
      basicAuth: false
      isDefault: true
      version: 1
      editable: false
      jsonData:
        timeInterval: 1m
`
				if values.IsGardenCluster {
					configMapData += `    - name: prometheus-longterm
      type: prometheus
      access: proxy
      url: http://prometheus-longterm:80
      basicAuth: false
      isDefault: false
      jsonData:
        timeInterval: 30s
      version: 1
      editable: false
`

				} else if values.ClusterType == comp.ClusterTypeSeed {
					configMapData += `    - name: seed-prometheus
      type: prometheus
      access: proxy
      url: http://prometheus-seed:80
      basicAuth: false
      version: 1
      editable: false
      jsonData:
        timeInterval: 1m
`
				}

				configMapData += `    - name: vali
      type: vali
      access: proxy
      url: http://logging.` + namespace + `.svc:3100
      jsonData:
        maxLines: ` + maxLine

				configMap := `apiVersion: v1
data:
  datasources.yaml: |
    ` + configMapData + `
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    component: plutono
    resources.gardener.cloud/garbage-collectable-reference: "true"
`
				if values.IsGardenCluster {
					configMap += `  name: plutono-datasources-b320ffed
  namespace: some-namespace
`
					return configMap
				}

				if values.ClusterType == comp.ClusterTypeShoot {
					configMap += `  name: plutono-datasources-f82429ca
  namespace: some-namespace
`
				} else {
					configMap += `  name: plutono-datasources-be28eaa6
  namespace: some-namespace
`
				}

				return configMap
			}

			deploymentYAMLFor = func(values Values) *appsv1.Deployment {
				dataSourceConfigMap, labelKey := "plutono-datasources-be28eaa6", "seed"
				if values.ClusterType == comp.ClusterTypeShoot {
					dataSourceConfigMap, labelKey = "plutono-datasources-f82429ca", "shoot"
				}
				if values.IsGardenCluster {
					dataSourceConfigMap, labelKey = "plutono-datasources-b320ffed", "garden"
				}

				deployment := &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "plutono",
						Namespace: namespace,
						Labels:    getLabels(),
					},
					Spec: appsv1.DeploymentSpec{
						RevisionHistoryLimit: ptr.To[int32](2),
						Replicas:             ptr.To(values.Replicas),
						Selector: &metav1.LabelSelector{
							MatchLabels: getLabels(),
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: utils.MergeStringMaps(getLabels(), getPodLabels(values)),
							},
							Spec: corev1.PodSpec{
								ServiceAccountName: "plutono",
								PriorityClassName:  values.PriorityClassName,
								Containers: []corev1.Container{
									{
										Name:            "plutono",
										Image:           values.Image,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Env: []corev1.EnvVar{
											{Name: "PL_AUTH_ANONYMOUS_ENABLED", Value: "true"},
											{Name: "PL_USERS_VIEWERS_CAN_EDIT", Value: "true"},
											{Name: "PL_DATE_FORMATS_DEFAULT_TIMEZONE", Value: "UTC"},
											{Name: "PL_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
											{Name: "PL_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
											{Name: "PL_ALERTING_ENABLED", Value: "false"},
											{Name: "PL_SNAPSHOTS_EXTERNAL_ENABLED", Value: "false"},
											{Name: "PL_PATHS_CONFIG", Value: "/usr/local/etc/plutono/plutono.ini"},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "datasources",
												MountPath: "/etc/plutono/provisioning/datasources",
											},
											{
												Name:      "dashboard-providers",
												MountPath: "/etc/plutono/provisioning/dashboards",
											},
											{
												Name:      "storage",
												MountPath: "/var/lib/plutono",
											},
											{
												Name:      "config",
												MountPath: "/usr/local/etc/plutono",
											},
										},
										Ports: []corev1.ContainerPort{
											{
												Name:          "web",
												ContainerPort: int32(3000),
											},
										},
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("5m"),
												corev1.ResourceMemory: resource.MustParse("45Mi"),
											},
										},
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: ptr.To(false),
										},
									},
									{
										Name:            "dashboard-refresher",
										Image:           values.ImageDashboardRefresher,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Command: []string{
											"python",
											"-u",
											"sidecar.py",
											"--req-username-file=/etc/dashboard-refresher/plutono-admin/username",
											"--req-password-file=/etc/dashboard-refresher/plutono-admin/password",
										},
										Env: []corev1.EnvVar{
											{Name: "LOG_LEVEL", Value: "INFO"},
											{Name: "RESOURCE", Value: "configmap"},
											{Name: "NAMESPACE", Value: namespace},
											{Name: "FOLDER", Value: "/var/lib/plutono/dashboards"},
											{Name: "LABEL", Value: "dashboard.monitoring.gardener.cloud/" + labelKey},
											{Name: "LABEL_VALUE", Value: "true"},
											{Name: "METHOD", Value: "WATCH"},
											{Name: "REQ_URL", Value: "http://localhost:3000/api/admin/provisioning/dashboards/reload"},
											{Name: "REQ_METHOD", Value: "POST"},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "storage",
												MountPath: "/var/lib/plutono",
											},
											{
												Name:      "admin-user",
												MountPath: "/etc/dashboard-refresher/plutono-admin",
											},
										},
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("5m"),
												corev1.ResourceMemory: resource.MustParse("85M"),
											},
										},
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: ptr.To(false),
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "datasources",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dataSourceConfigMap,
												},
											},
										},
									},
									{
										Name: "dashboard-providers",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "plutono-dashboard-providers-140e41f3",
												},
											},
										},
									},
									{
										Name: "config",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "plutono-config-fd97f886",
											},
										},
									},
									{
										Name: "admin-user",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "plutono-admin-68aadabd",
											},
										},
									},
									{
										Name: "storage",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{
												SizeLimit: ptr.To(resource.MustParse("100Mi")),
											},
										},
									},
									{
										Name: "dashboards",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{},
										},
									},
								},
							},
						},
					},
				}

				if values.ClusterType == comp.ClusterTypeSeed {
					deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{"role": "monitoring"})
				} else {
					deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{"gardener.cloud/role": "monitoring"})
				}

				return deployment
			}

			serviceYAMLFor = func(values Values) string {
				out := `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    component: plutono
`
				if values.ClusterType == comp.ClusterTypeSeed {
					out += `    role: monitoring
`
				}
				out += `  name: plutono
  namespace: ` + namespace + `
spec:
  ports:
  - name: web
    port: 3000
    protocol: TCP
    targetPort: 3000
  selector:
    component: plutono
  type: ClusterIP
status:
  loadBalancer: {}
`
				return out
			}

			ingressYAMLFor = func(values Values) string {
				out := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/auth-realm: Authentication Required
`
				if !values.IsGardenCluster {
					if values.ClusterType == comp.ClusterTypeShoot {
						out += `    nginx.ingress.kubernetes.io/auth-secret: observability-ingress-users
    nginx.ingress.kubernetes.io/auth-type: basic
`
					} else {
						out += `    nginx.ingress.kubernetes.io/auth-secret: global-monitoring-secret
    nginx.ingress.kubernetes.io/auth-type: basic
`
					}
				} else {
					out += `    nginx.ingress.kubernetes.io/auth-secret: observability-ingress
    nginx.ingress.kubernetes.io/auth-type: basic
`
				}
				out += `    nginx.ingress.kubernetes.io/server-snippet: |-
      location /api/admin/ {
        return 403;
      }
  creationTimestamp: null
`
				if values.ClusterType == comp.ClusterTypeShoot {
					out += `  labels:
    component: plutono
`
				}
				out += `  name: plutono
  namespace: ` + namespace + `
spec:
  ingressClassName: nginx-ingress-gardener
  rules:
  - host: ` + values.IngressHost + `
    http:
      paths:
      - backend:
          service:
            name: plutono
            port:
              number: 3000
        path: /
`
				if values.IsGardenCluster {
					out += `        pathType: ImplementationSpecific
`
				} else {
					out += `        pathType: Prefix
`
				}

				out += `  tls:
  - hosts:
    - ` + values.IngressHost + `
    secretName: plutono-tls
status:
  loadBalancer: {}
`
				return out
			}
		)

		JustBeforeEach(func() {
			component = New(c, namespace, fakeSecretManager, values)
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels: map[string]string{
						v1beta1constants.GardenRole:          "seed-system-component",
						"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
					},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:       ptr.To("seed"),
					KeepObjects: ptr.To(false),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())
		})

		checkDeployedResources := func(dashboardConfigMapName string, dashboardCount int) {
			GinkgoHelper()

			deployment := deploymentYAMLFor(values)
			utilruntime.Must(references.InjectAnnotations(deployment))

			plutonoConfigSecretYAML, err := kubernetesutils.Serialize(plutonoConfigSecret, c.Scheme())
			Expect(err).NotTo(HaveOccurred())
			serviceAccountYAML, err := kubernetesutils.Serialize(serviceAccount, c.Scheme())
			Expect(err).NotTo(HaveOccurred())
			roleYAML, err := kubernetesutils.Serialize(role, c.Scheme())
			Expect(err).NotTo(HaveOccurred())
			roleBindingYAML, err := kubernetesutils.Serialize(roleBinding, c.Scheme())
			Expect(err).NotTo(HaveOccurred())
			deploymentYAML, err := kubernetesutils.Serialize(deployment, c.Scheme())
			Expect(err).NotTo(HaveOccurred())

			Expect(manifests).To(ConsistOf(
				plutonoConfigSecretYAML,
				serviceAccountYAML,
				roleYAML,
				roleBindingYAML,
				deploymentYAML,
				providerConfigMapYAML,
				dataSourceConfigMapYAMLFor(values),
				serviceYAMLFor(values),
				ingressYAMLFor(values),
			), "Resource manifests do not match the expected ones")

			dashboardsConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: dashboardConfigMapName, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(dashboardsConfigMap), dashboardsConfigMap)).To(Succeed(), "Could not successfully get dashboards configMap")

			labelKey := "seed"
			if values.ClusterType == comp.ClusterTypeShoot {
				labelKey = "shoot"
			}
			if values.IsGardenCluster {
				labelKey = "garden"
			}
			Expect(dashboardsConfigMap.Labels).To(HaveKeyWithValue("dashboard.monitoring.gardener.cloud/"+labelKey, "true"), "Dashboards configMap does not contain expected key")

			availableDashboards := sets.Set[string]{}
			for key := range dashboardsConfigMap.Data {
				availableDashboards.Insert(key)
			}
			Expect(availableDashboards).To(HaveLen(dashboardCount), "The number of deployed dashboards differs from the expected one")
		}

		Context("Cluster type is seed", func() {
			BeforeEach(func() {
				values.ClusterType = comp.ClusterTypeSeed
				values.IngressHost = "seed.example.com"
			})

			Context("Cluster is not garden cluster", func() {
				BeforeEach(func() {
					values.AuthSecretName = "global-monitoring-secret"
					values.IncludeIstioDashboards = true
				})

				It("should successfully deploy all resources", func() {
					checkDeployedResources("plutono-dashboards", 21)
				})

				Context("w/ enabled vpa", func() {
					BeforeEach(func() {
						values.VPAEnabled = true
					})

					It("should successfully deploy all resources", func() {
						checkDeployedResources("plutono-dashboards", 24)
					})
				})
			})

			Context("Cluster is garden cluster", func() {
				BeforeEach(func() {
					values.IsGardenCluster = true
				})

				Context("with VPAEnabled=true", func() {
					BeforeEach(func() {
						values.VPAEnabled = true
					})

					It("should successfully deploy all resources", func() {
						checkDeployedResources("plutono-dashboards-garden", 27)
					})
				})

				It("should successfully deploy all resources", func() {
					checkDeployedResources("plutono-dashboards-garden", 24)
				})
			})
		})

		Context("Cluster type is shoot", func() {
			BeforeEach(func() {
				values.ClusterType = comp.ClusterTypeShoot
				values.IngressHost = "shoot.example.com"
			})

			It("should successfully deploy all resources", func() {
				checkDeployedResources("plutono-dashboards", 33)
			})

			Context("w/ include istio, mcm, ha-vpn, vpa", func() {
				BeforeEach(func() {
					values.IncludeIstioDashboards = true
					values.VPNHighAvailabilityEnabled = true
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					checkDeployedResources("plutono-dashboards", 37)
				})
			})

			Context("shoot is workerless", func() {
				BeforeEach(func() {
					values.IsWorkerless = true
				})

				It("should successfully deploy all resources", func() {
					checkDeployedResources("plutono-dashboards", 25)
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			dashboardConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "plutono-dashboards", Namespace: namespace}}

			component = New(c, namespace, fakeSecretManager, values)
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, dashboardConfigMap)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(dashboardConfigMap), dashboardConfigMap)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(dashboardConfigMap), dashboardConfigMap)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			component = New(c, namespace, fakeSecretManager, values)
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func getLabels() map[string]string {
	return map[string]string{
		"component": "plutono",
	}
}

func getPodLabels(values Values) map[string]string {
	labels := map[string]string{
		"observability.gardener.cloud/app":                "plutono",
		"networking.gardener.cloud/to-dns":                "allowed",
		"networking.gardener.cloud/to-runtime-apiserver":  "allowed",
		gardenerutils.NetworkPolicyLabel("logging", 3100): "allowed",
	}

	if values.IsGardenCluster {
		labels = utils.MergeStringMaps(labels, map[string]string{
			gardenerutils.NetworkPolicyLabel("prometheus-garden", 9090):   v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("prometheus-longterm", 9091): v1beta1constants.LabelNetworkPolicyAllowed,
		})

		return labels
	}

	if values.ClusterType == comp.ClusterTypeSeed {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.LabelRole:                                     v1beta1constants.LabelMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-aggregate", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("prometheus-seed", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
		})
	} else if values.ClusterType == comp.ClusterTypeShoot {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.GardenRole:                                v1beta1constants.GardenRoleMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-shoot", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return labels
}
