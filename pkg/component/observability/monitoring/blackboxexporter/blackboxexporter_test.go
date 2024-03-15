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

package blackboxexporter_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BlackboxExporter", func() {
	var (
		ctx = context.TODO()

		managedResourceName string
		namespace           string
		resourcesNamespace  string
		image               = "some-image:some-tag"
		config              = blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{"foo": {}}}
		podLabels           = map[string]string{"bar": "foo"}
		priorityClassName   = "priority-class"

		c                  client.Client
		fakeSecretsManager secretsmanager.Interface
		values             Values
		deployer           component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		configMapName      = "blackbox-exporter-config-eb6ac772"
		serviceAccountYAML string
		configMapYAML      string
		deploymentYAMLFor  func(clusterType component.ClusterType) string
		pdbYAMLFor         func(k8sGreaterEquals126 bool) string
		serviceYAMLFor     func(clusterType component.ClusterType) string
		vpaYAML            string
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		managedResourceName = "shoot-core-blackbox-exporter"
		namespace = "some-namespace"
		resourcesNamespace = "kube-system"

		values.ClusterType = component.ClusterTypeShoot
		values.Image = image
		values.VPAEnabled = false
		values.KubernetesVersion = semver.MustParse("1.25.5")
		values.Config = config
		values.PodLabels = podLabels
		values.PriorityClassName = priorityClassName
	})

	JustBeforeEach(func() {
		fakeSecretsManager = fakesecretsmanager.New(c, namespace)
		deployer = New(c, fakeSecretsManager, namespace, values)

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

		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    component: blackbox-exporter
    gardener.cloud/role: monitoring
    origin: gardener
  name: blackbox-exporter
  namespace: ` + resourcesNamespace + `
`

		configMapYAML = `apiVersion: v1
data:
  blackbox.yaml: |
    modules:
        foo: {}
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: prometheus
    resources.gardener.cloud/garbage-collectable-reference: "true"
    role: monitoring
  name: ` + configMapName + `
  namespace: ` + resourcesNamespace + `
`

		pdbYAMLFor = func(k8sGreaterEquals126 bool) string {
			out := `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    component: blackbox-exporter
    gardener.cloud/role: monitoring
  name: blackbox-exporter
  namespace: ` + resourcesNamespace + `
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      component: blackbox-exporter
`
			if k8sGreaterEquals126 {
				out += `  unhealthyPodEvictionPolicy: AlwaysAllow
`
			}
			out += `status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`
			return out
		}

		deploymentYAMLFor = func(clusterType component.ClusterType) string {
			out := `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
  creationTimestamp: null
  labels:
    component: blackbox-exporter
    gardener.cloud/role: monitoring
    high-availability-config.resources.gardener.cloud/type: server
    origin: gardener
  name: blackbox-exporter
  namespace: ` + resourcesNamespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      component: blackbox-exporter
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
      creationTimestamp: null
      labels:
        bar: foo
        component: blackbox-exporter
        gardener.cloud/role: monitoring
        origin: gardener
    spec:
      containers:
      - args:
        - --config.file=/etc/blackbox_exporter/blackbox.yaml
        - --log.level=debug
        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        name: blackbox-exporter
        ports:
        - containerPort: 9115
          name: probe
          protocol: TCP
        resources:
          limits:
            memory: 128Mi
          requests:
            cpu: 10m
            memory: 25Mi
        volumeMounts:
        - mountPath: /etc/blackbox_exporter
          name: blackbox-exporter-config`

			if clusterType == component.ClusterTypeSeed {
				out += `
        - mountPath: /var/run/secrets/blackbox_exporter/cluster-access
          name: cluster-access`
			}

			out += `
      dnsConfig:
        options:
        - name: ndots
          value: "3"
      priorityClassName: ` + priorityClassName + `
      securityContext:
        fsGroup: 65534
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      serviceAccountName: blackbox-exporter
      volumes:
      - configMap:
          name: ` + configMapName + `
        name: blackbox-exporter-config`

			if clusterType == component.ClusterTypeSeed {
				out += `
      - name: cluster-access
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: bundle.crt
                path: bundle.crt
              name: ca
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: shoot-access-prometheus-garden
              optional: false`
			}

			out += `
status: {}
`

			return out
		}

		serviceYAMLFor = func(clusterType component.ClusterType) string {
			out := `apiVersion: v1
kind: Service
metadata:`

			if clusterType == component.ClusterTypeSeed {
				out += `
  annotations:
    networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports: '[{"protocol":"TCP","port":9115}]'`
			}

			out += `
  creationTimestamp: null
  labels:
    component: blackbox-exporter
  name: blackbox-exporter
  namespace: ` + resourcesNamespace + `
spec:
  ports:
  - name: probe
    port: 9115
    protocol: TCP
    targetPort: 0
  selector:
    component: blackbox-exporter
  type: ClusterIP
status:
  loadBalancer: {}
`
			return out
		}

		vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: blackbox-exporter
  namespace: ` + resourcesNamespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: blackbox-exporter
  updatePolicy:
    updateMode: Auto
status: {}
`
	})

	Context("cluster type shoot", func() {
		Describe("#Deploy", func() {
			JustBeforeEach(func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels:          map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__blackbox-exporter.yaml"])).To(Equal(serviceAccountYAML))
				Expect(string(managedResourceSecret.Data["configmap__kube-system__blackbox-exporter-config-eb6ac772.yaml"])).To(Equal(configMapYAML))
				Expect(string(managedResourceSecret.Data["deployment__kube-system__blackbox-exporter.yaml"])).To(Equal(deploymentYAMLFor(values.ClusterType)))
				Expect(string(managedResourceSecret.Data["service__kube-system__blackbox-exporter.yaml"])).To(Equal(serviceYAMLFor(values.ClusterType)))
			})

			Context("w/o vpa enabled", func() {
				Context("kubernetes versions < 1.26", func() {
					It("should successfully deploy the resources", func() {
						Expect(managedResourceSecret.Data).To(HaveLen(5))
						Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__blackbox-exporter.yaml"])).To(Equal(pdbYAMLFor(false)))
					})
				})

				Context("kubernetes versions >= 1.26", func() {
					BeforeEach(func() {
						values.KubernetesVersion = semver.MustParse("1.26")
					})

					It("should successfully deploy the resources", func() {
						Expect(managedResourceSecret.Data).To(HaveLen(5))
						Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__blackbox-exporter.yaml"])).To(Equal(pdbYAMLFor(true)))
					})
				})
			})

			Context("w/ vpa enabled", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy the resources", func() {
					Expect(managedResourceSecret.Data).To(HaveLen(6))
					Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__blackbox-exporter.yaml"])).To(Equal(pdbYAMLFor(false)))
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__blackbox-exporter.yaml"])).To(Equal(vpaYAML))
				})
			})
		})

		Describe("#Destroy", func() {
			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(deployer.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		})
	})

	Context("cluster type seed", func() {
		BeforeEach(func() {
			values.ClusterType = component.ClusterTypeSeed
			managedResourceName = "blackbox-exporter"
			resourcesNamespace = namespace

			By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		})

		Describe("#Deploy", func() {
			JustBeforeEach(func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
							"gardener.cloud/role":                "seed-system-component",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("seed"),
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(string(managedResourceSecret.Data["serviceaccount__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(serviceAccountYAML))
				Expect(string(managedResourceSecret.Data["configmap__"+namespace+"__blackbox-exporter-config-eb6ac772.yaml"])).To(Equal(configMapYAML))
				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(deploymentYAMLFor(values.ClusterType)))
				Expect(string(managedResourceSecret.Data["service__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(serviceYAMLFor(values.ClusterType)))
			})

			Context("w/o vpa enabled", func() {
				Context("kubernetes versions < 1.26", func() {
					It("should successfully deploy the resources", func() {
						Expect(managedResourceSecret.Data).To(HaveLen(5))
						Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(pdbYAMLFor(false)))
					})
				})

				Context("kubernetes versions >= 1.26", func() {
					BeforeEach(func() {
						values.KubernetesVersion = semver.MustParse("1.26")
					})

					It("should successfully deploy the resources", func() {
						Expect(managedResourceSecret.Data).To(HaveLen(5))
						Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(pdbYAMLFor(true)))
					})
				})
			})

			Context("w/ vpa enabled", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy the resources", func() {
					Expect(managedResourceSecret.Data).To(HaveLen(6))
					Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(pdbYAMLFor(false)))
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__"+namespace+"__blackbox-exporter.yaml"])).To(Equal(vpaYAML))
				})
			})
		})

		Describe("#Destroy", func() {
			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(deployer.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
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

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
