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

package plutono_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	comp "github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	. "github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Plutono", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "plutono"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c                 client.Client
		component         comp.DeployWaiter
		fakeSecretManager secretsmanager.Interface
		values            Values

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
		extensionConfigMap    *corev1.ConfigMap
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)

		values = Values{
			Image:    image,
			Replicas: int32(1),
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

		// extensions dashboard
		filePath := filepath.Join("testdata", "configmap.yaml")
		cm, err := os.ReadFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(yaml.Unmarshal(cm, &extensionConfigMap)).To(Succeed())
		extensionConfigMap.ObjectMeta.ResourceVersion = ""
		Expect(c.Create(ctx, extensionConfigMap)).To(Succeed())
	})

	Describe("#Deploy", func() {
		var (
			providerConfigMapYAMLFor = func(values Values) string {
				configMapName := "plutono-dashboard-providers-29d306e7"
				configMapData := `  apiVersion: 1
    providers:
    - name: 'default'
      orgId: 1
      folder: ''
      type: file
      disableDeletion: false
      editable: false
      options:
        path: /var/lib/plutono/dashboards`

				if values.IsGardenCluster {
					configMapName = "plutono-dashboard-providers-5be2bcda"
					configMapData = `  apiVersion: 1
    providers:
    - name: 'global'
      orgId: 1
      folder: 'Global'
      type: file
      disableDeletion: false
      editable: false
      updateIntervalSeconds: 120
      options:
        path: /var/lib/plutono/dashboards/global
    - name: 'garden'
      orgId: 1
      folder: 'Garden'
      type: file
      disableDeletion: false
      editable: false
      updateIntervalSeconds: 120
      options:
        path: /var/lib/plutono/dashboards/garden`
				}

				configMap := `apiVersion: v1
data:
  default.yaml: |
  ` + configMapData + `
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    component: plutono
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapName + `
  namespace: some-namespace
`

				return configMap
			}

			dataSourceConfigMapYAMLFor = func(values Values) string {
				url := "http://prometheus-web:80"
				maxLine := "1000"
				if values.IsGardenCluster {
					url = "http://" + namespace + "-prometheus:80"
					maxLine = "5000"
				} else if values.ClusterType == comp.ClusterTypeSeed {
					url = "http://aggregate-prometheus-web:80"
					maxLine = "5000"
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
					configMapData += `    - name: availability-prometheus
      type: prometheus
      access: proxy
      url: http://` + namespace + `-avail-prom:80
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
      url: http://seed-prometheus-web:80
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
					configMap += `  name: plutono-datasources-ff212e8b
  namespace: some-namespace
`
					return configMap
				}

				if values.ClusterType == comp.ClusterTypeShoot {
					configMap += `  name: plutono-datasources-0fd41775
  namespace: some-namespace
`
				} else {
					configMap += `  name: plutono-datasources-27f1a6c5
  namespace: some-namespace
`
				}

				return configMap
			}

			deploymentYAMLFor = func(values Values, dashboardConfigMaps []string) *appsv1.Deployment {
				providerConfigMap := "plutono-dashboard-providers-29d306e7"
				dataSourceConfigMap := "plutono-datasources-27f1a6c5"
				if values.ClusterType == comp.ClusterTypeShoot {
					dataSourceConfigMap = "plutono-datasources-0fd41775"
				}
				if values.IsGardenCluster {
					providerConfigMap = "plutono-dashboard-providers-5be2bcda"
					dataSourceConfigMap = "plutono-datasources-ff212e8b"
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
						RevisionHistoryLimit: ptr.To(int32(2)),
						Replicas:             ptr.To(values.Replicas),
						Selector: &metav1.LabelSelector{
							MatchLabels: getLabels(),
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: utils.MergeStringMaps(getLabels(), getPodLabels(values)),
							},
							Spec: corev1.PodSpec{
								AutomountServiceAccountToken: ptr.To(false),
								PriorityClassName:            values.PriorityClassName,
								Containers: []corev1.Container{
									{
										Name:            "plutono",
										Image:           values.Image,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Env: []corev1.EnvVar{
											{Name: "PL_AUTH_ANONYMOUS_ENABLED", Value: "true"},
											{Name: "PL_USERS_VIEWERS_CAN_EDIT", Value: "true"},
											{Name: "PL_DATE_FORMATS_DEFAULT_TIMEZONE", Value: "UTC"},
											{Name: "PL_AUTH_BASIC_ENABLED", Value: "false"},
											{Name: "PL_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
											{Name: "PL_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
											{Name: "PL_ALERTING_ENABLED", Value: "false"},
											{Name: "PL_SNAPSHOTS_EXTERNAL_ENABLED", Value: "false"},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "plutono-datasources",
												MountPath: "/etc/plutono/provisioning/datasources",
											},
											{
												Name:      "plutono-dashboard-providers",
												MountPath: "/etc/plutono/provisioning/dashboards",
											},
											{
												Name:      "plutono-storage",
												MountPath: "/var/lib/plutono",
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
												corev1.ResourceCPU:    resource.MustParse("10m"),
												corev1.ResourceMemory: resource.MustParse("32Mi"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("400Mi"),
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "plutono-datasources",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: dataSourceConfigMap,
												},
											},
										},
									},
									{
										Name: "plutono-dashboard-providers",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: providerConfigMap,
												},
											},
										},
									},
									{
										Name: "plutono-storage",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{
												SizeLimit: utils.QuantityPtr(resource.MustParse("100Mi")),
											},
										},
									},
								},
							},
						},
					},
				}

				if values.IsGardenCluster {
					deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
						{
							Name: "plutono-dashboards-garden",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: dashboardConfigMaps[0],
									},
								},
							},
						},
						{
							Name: "plutono-dashboards-global",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: dashboardConfigMaps[1],
									},
								},
							},
						}}...)

					deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
						{
							Name:      "plutono-dashboards-garden",
							MountPath: "/var/lib/plutono/dashboards/garden",
						},
						{
							Name:      "plutono-dashboards-global",
							MountPath: "/var/lib/plutono/dashboards/global",
						},
					}...)
				} else {
					deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
						Name: "plutono-dashboards",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: dashboardConfigMaps[0],
								},
							},
						},
					})

					deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "plutono-dashboards",
						MountPath: "/var/lib/plutono/dashboards",
					})
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
						out += `    nginx.ingress.kubernetes.io/auth-secret: observability-ingress-users-f27eb0bf
    nginx.ingress.kubernetes.io/auth-type: basic
`
					} else {
						out += `    nginx.ingress.kubernetes.io/auth-secret: global-monitoring-secret
    nginx.ingress.kubernetes.io/auth-type: basic
`
					}
				} else {
					out += `    nginx.ingress.kubernetes.io/auth-secret: observability-ingress-0da36eb1
    nginx.ingress.kubernetes.io/auth-type: basic
`
				}
				out += `  creationTimestamp: null
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
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
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
			Expect(managedResourceSecret.Data).To(HaveLen(5))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})

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
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-dashboard-providers-29d306e7.yaml"])).To(Equal(providerConfigMapYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-datasources-27f1a6c5.yaml"])).To(Equal(dataSourceConfigMapYAMLFor(values)))
					plutonoDashboardsConfigMap, err := getDashboardConfigMaps(ctx, c, namespace, "plutono-dashboards-[^-]{8}")
					Expect(err).ToNot(HaveOccurred())
					testDashboardConfigMap(ctx, c, types.NamespacedName{Namespace: namespace, Name: plutonoDashboardsConfigMap.Name}, 22)
					Expect(string(managedResourceSecret.Data["service__some-namespace__plutono.yaml"])).To(Equal(serviceYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["ingress__some-namespace__plutono.yaml"])).To(Equal(ingressYAMLFor(values)))
					managedResourceDeployment, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["deployment__some-namespace__plutono.yaml"], nil, &appsv1.Deployment{})
					Expect(err).ToNot(HaveOccurred())
					deployment := deploymentYAMLFor(values, []string{plutonoDashboardsConfigMap.Name})
					utilruntime.Must(references.InjectAnnotations(deployment))
					Expect(deployment).To(DeepEqual(managedResourceDeployment))
				})
			})

			Context("Cluster is garden cluster", func() {
				BeforeEach(func() {
					values.IsGardenCluster = true
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-dashboard-providers-5be2bcda.yaml"])).To(Equal(providerConfigMapYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-datasources-ff212e8b.yaml"])).To(Equal(dataSourceConfigMapYAMLFor(values)))
					plutonoDashboardsGardenConfigMap, err := getDashboardConfigMaps(ctx, c, namespace, "plutono-dashboards-garden-[^-]{8}")
					Expect(err).ToNot(HaveOccurred())
					plutonoDashboardsGlobalConfigMap, err := getDashboardConfigMaps(ctx, c, namespace, "plutono-dashboards-global-[^-]{8}")
					Expect(err).ToNot(HaveOccurred())
					testDashboardConfigMap(ctx, c, types.NamespacedName{Namespace: namespace, Name: plutonoDashboardsGardenConfigMap.Name}, 16)
					testDashboardConfigMap(ctx, c, types.NamespacedName{Namespace: namespace, Name: plutonoDashboardsGlobalConfigMap.Name}, 7)
					Expect(string(managedResourceSecret.Data["service__some-namespace__plutono.yaml"])).To(Equal(serviceYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["ingress__some-namespace__plutono.yaml"])).To(Equal(ingressYAMLFor(values)))
					managedResourceDeployment, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["deployment__some-namespace__plutono.yaml"], nil, &appsv1.Deployment{})
					Expect(err).ToNot(HaveOccurred())
					deployment := deploymentYAMLFor(values, []string{plutonoDashboardsGardenConfigMap.Name, plutonoDashboardsGlobalConfigMap.Name})
					utilruntime.Must(references.InjectAnnotations(deployment))
					Expect(deployment).To(DeepEqual(managedResourceDeployment))
				})
			})
		})

		Context("Cluster type is shoot", func() {
			BeforeEach(func() {
				values.ClusterType = comp.ClusterTypeShoot
				values.IngressHost = "shoot.example.com"
			})

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-dashboard-providers-29d306e7.yaml"])).To(Equal(providerConfigMapYAMLFor(values)))
				Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-datasources-0fd41775.yaml"])).To(Equal(dataSourceConfigMapYAMLFor(values)))
				plutonoDashboardsConfigMap, err := getDashboardConfigMaps(ctx, c, namespace, "plutono-dashboards-[^-]{8}")
				Expect(err).ToNot(HaveOccurred())
				testDashboardConfigMap(ctx, c, types.NamespacedName{Namespace: namespace, Name: plutonoDashboardsConfigMap.Name}, 34)
				Expect(string(managedResourceSecret.Data["service__some-namespace__plutono.yaml"])).To(Equal(serviceYAMLFor(values)))
				Expect(string(managedResourceSecret.Data["ingress__some-namespace__plutono.yaml"])).To(Equal(ingressYAMLFor(values)))
				managedResourceDeployment, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["deployment__some-namespace__plutono.yaml"], nil, &appsv1.Deployment{})
				Expect(err).ToNot(HaveOccurred())
				deployment := deploymentYAMLFor(values, []string{plutonoDashboardsConfigMap.Name})
				utilruntime.Must(references.InjectAnnotations(deployment))
				Expect(deployment).To(DeepEqual(managedResourceDeployment))
			})

			Context("w/ include istio, node-local-dns, mcm, ha-vpn, vpa", func() {
				BeforeEach(func() {
					values.IncludeIstioDashboards = true
					values.NodeLocalDNSEnabled = true
					values.VPNHighAvailabilityEnabled = true
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-dashboard-providers-29d306e7.yaml"])).To(Equal(providerConfigMapYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-datasources-0fd41775.yaml"])).To(Equal(dataSourceConfigMapYAMLFor(values)))
					plutonoDashboardsConfigMap, err := getDashboardConfigMaps(ctx, c, namespace, "plutono-dashboards-[^-]{8}")
					Expect(err).ToNot(HaveOccurred())
					testDashboardConfigMap(ctx, c, types.NamespacedName{Namespace: namespace, Name: plutonoDashboardsConfigMap.Name}, 38)
					Expect(string(managedResourceSecret.Data["service__some-namespace__plutono.yaml"])).To(Equal(serviceYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["ingress__some-namespace__plutono.yaml"])).To(Equal(ingressYAMLFor(values)))
					managedResourceDeployment, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["deployment__some-namespace__plutono.yaml"], nil, &appsv1.Deployment{})
					Expect(err).ToNot(HaveOccurred())
					deployment := deploymentYAMLFor(values, []string{plutonoDashboardsConfigMap.Name})
					utilruntime.Must(references.InjectAnnotations(deployment))
					Expect(deployment).To(DeepEqual(managedResourceDeployment))
				})
			})

			Context("shoot is workerless", func() {
				BeforeEach(func() {
					values.IsWorkerless = true
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-dashboard-providers-29d306e7.yaml"])).To(Equal(providerConfigMapYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["configmap__some-namespace__plutono-datasources-0fd41775.yaml"])).To(Equal(dataSourceConfigMapYAMLFor(values)))
					plutonoDashboardsConfigMap, err := getDashboardConfigMaps(ctx, c, namespace, "plutono-dashboards-[^-]{8}")
					Expect(err).ToNot(HaveOccurred())
					testDashboardConfigMap(ctx, c, types.NamespacedName{Namespace: namespace, Name: plutonoDashboardsConfigMap.Name}, 27)
					Expect(string(managedResourceSecret.Data["service__some-namespace__plutono.yaml"])).To(Equal(serviceYAMLFor(values)))
					Expect(string(managedResourceSecret.Data["ingress__some-namespace__plutono.yaml"])).To(Equal(ingressYAMLFor(values)))
					managedResourceDeployment, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["deployment__some-namespace__plutono.yaml"], nil, &appsv1.Deployment{})
					Expect(err).ToNot(HaveOccurred())
					deployment := deploymentYAMLFor(values, []string{plutonoDashboardsConfigMap.Name})
					utilruntime.Must(references.InjectAnnotations(deployment))
					Expect(deployment).To(DeepEqual(managedResourceDeployment))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			component = New(c, namespace, fakeSecretManager, values)
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
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

func testDashboardConfigMap(ctx context.Context, c client.Client, namespaceName types.NamespacedName, dashboardCount int) {
	var (
		configmap           = &corev1.ConfigMap{}
		availableDashboards = sets.Set[string]{}
	)

	ExpectWithOffset(1, c.Get(ctx, namespaceName, configmap)).To(Succeed())

	for key := range configmap.Data {
		availableDashboards.Insert(key)
	}
	ExpectWithOffset(1, availableDashboards).To(HaveLen(dashboardCount))
}

func getDashboardConfigMaps(ctx context.Context, c client.Client, namespace string, pattern string) (*corev1.ConfigMap, error) {
	configMapList := &corev1.ConfigMapList{}
	if err := c.List(ctx, configMapList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	for _, configMap := range configMapList.Items {
		if ok, err := regexp.Match(pattern, []byte(configMap.Name)); ok && err == nil {
			return &configMap, nil
		}
	}

	return nil, errors.New("ConfigMap Not Found")
}

func getLabels() map[string]string {
	return map[string]string{
		"component": "plutono",
	}
}

func getPodLabels(values Values) map[string]string {
	labels := map[string]string{
		v1beta1constants.LabelNetworkPolicyToDNS:                          v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(vali.ServiceName, vali.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
	}

	if values.IsGardenCluster {
		labels = utils.MergeStringMaps(labels, map[string]string{
			gardenerutils.NetworkPolicyLabel("garden-prometheus", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("garden-avail-prom", 9091): v1beta1constants.LabelNetworkPolicyAllowed,
		})

		return labels
	}

	if values.ClusterType == comp.ClusterTypeSeed {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.LabelRole:                                         v1beta1constants.LabelMonitoring,
			"networking.gardener.cloud/to-seed-prometheus":                     v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("aggregate-prometheus-web", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("seed-prometheus-web", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
		})
	} else if values.ClusterType == comp.ClusterTypeShoot {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.GardenRole:                              v1beta1constants.GardenRoleMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-web", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return labels
}
