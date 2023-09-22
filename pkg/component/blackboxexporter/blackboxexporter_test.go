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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/blackboxexporter"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BlackboxExporter", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-core-blackbox-exporter"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:             image,
			VPAEnabled:        false,
			KubernetesVersion: semver.MustParse("1.25.5"),
		}
		component = New(c, namespace, values)

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
	})

	Describe("#Deploy", func() {
		var (
			vpaEnabled    = false
			configMapName = "blackbox-exporter-config-07d191e0"

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
  namespace: kube-system
`

			configMapYAML = `apiVersion: v1
data:
  blackbox.yaml: |
    modules:
      http_kubernetes_service:
        prober: http
        timeout: 10s
        http:
          headers:
            Accept: "*/*"
            Accept-Language: "en-US"
          tls_config:
            ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
          bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
          preferred_ip_protocol: "ip4"
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: prometheus
    resources.gardener.cloud/garbage-collectable-reference: "true"
    role: monitoring
  name: ` + configMapName + `
  namespace: kube-system
`

			pdbYAML = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    component: blackbox-exporter
    gardener.cloud/role: monitoring
  name: blackbox-exporter
  namespace: kube-system
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      component: blackbox-exporter
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`

			deploymentYAML = `apiVersion: apps/v1
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
  namespace: kube-system
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
        component: blackbox-exporter
        gardener.cloud/role: monitoring
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-public-networks: allowed
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
          name: blackbox-exporter-config
      dnsConfig:
        options:
        - name: ndots
          value: "3"
      priorityClassName: system-cluster-critical
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
        name: blackbox-exporter-config
status: {}
`

			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    component: blackbox-exporter
  name: blackbox-exporter
  namespace: kube-system
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

			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: blackbox-exporter
  namespace: kube-system
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
		)

		BeforeEach(func() {
			vpaEnabled = false
		})

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))

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
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			if vpaEnabled {
				Expect(managedResourceSecret.Data).To(HaveLen(6))
			} else {
				Expect(managedResourceSecret.Data).To(HaveLen(5))
			}
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__blackbox-exporter.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__blackbox-exporter-config-07d191e0.yaml"])).To(Equal(configMapYAML))
			Expect(string(managedResourceSecret.Data["deployment__kube-system__blackbox-exporter.yaml"])).To(Equal(deploymentYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__blackbox-exporter.yaml"])).To(Equal(serviceYAML))
		})

		BeforeEach(func() {
			values.KubernetesVersion = semver.MustParse("1.26.0")
			component = New(c, namespace, values)
		})

		Context("w/o vpa enabled", func() {
			It("should successfully deploy the resources", func() {
				Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__blackbox-exporter.yaml"])).To(Equal(pdbYAML))
			})
		})

		Context("w/ vpa enabled", func() {
			BeforeEach(func() {
				vpaEnabled = true
				values.VPAEnabled = true
				component = New(c, namespace, values)
			})

			It("should successfully deploy the resources", func() {
				Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__blackbox-exporter.yaml"])).To(Equal(pdbYAML))
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

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
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
