// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nginxingress_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/nginxingress"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Nginx Ingress", func() {
	var (
		ctx                 = context.TODO()
		namespace           = "some-namespace"
		imageController     = "some-image:some-tag"
		imageDefaultBackend = "some-image2:some-tag2"
		managedResourceName = "nginx-ingress"

		c            client.Client
		nginxIngress component.DeployWaiter

		values = Values{
			ImageController:     imageController,
			ImageDefaultBackend: imageDefaultBackend,
		}

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		configMapData = map[string]string{
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
		}
		configMapName = "nginx-ingress-controller-" + utils.ComputeConfigMapChecksum(configMapData)[:8]
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

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
			configMapYAML = `apiVersion: v1
data:
  server-name-hash-bucket-size: "256"
  use-proxy-protocol: "false"
  worker-processes: "2"
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapName + `
  namespace: ` + namespace + `
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: gardener.cloud:seed:nginx-ingress
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - nodes
  - pods
  - secrets
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - services
  - configmaps
  verbs:
  - get
  - list
  - update
  - watch
- apiGroups:
  - extensions
  - networking.k8s.io
  resources:
  - ingresses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - extensions
  - networking.k8s.io
  resources:
  - ingresses/status
  verbs:
  - update
- apiGroups:
  - networking.k8s.io
  resources:
  - ingressclasses
  verbs:
  - get
  - list
  - watch
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: gardener.cloud:seed:nginx-ingress
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:seed:nginx-ingress
subjects:
- kind: ServiceAccount
  name: nginx-ingress
  namespace: ` + namespace + `
`
			roleBackendYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: nginx-ingress
  namespace: ` + namespace + `
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  - namespaces
  - pods
  - secrets
  verbs:
  - get
- apiGroups:
  - ""
  resourceNames:
  - ingress-controller-leader-nginx
  resources:
  - configmaps
  verbs:
  - get
  - update
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - create
  - get
  - update
`
			roleBindingBackendYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress-k8s-backend
  name: nginx-ingress
  namespace: ` + namespace + `
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: nginx-ingress
subjects:
- kind: ServiceAccount
  name: nginx-ingress
  namespace: ` + namespace + `
`
			serviceControllerYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 443
  selector:
    app: nginx-ingress
    component: controller
    release: addons
  type: LoadBalancer
status:
  loadBalancer: {}
`
			serviceBackendYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: nginx-ingress-k8s-backend
  namespace: ` + namespace + `
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
    release: addons
  type: ClusterIP
status:
  loadBalancer: {}
`
			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: nginx-ingress
  namespace: ` + namespace + `
`
			ingressClassYAML = `apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: ` + v1beta1constants.SeedNginxIngressClass122 + `
spec:
  controller: k8s.io/` + v1beta1constants.SeedNginxIngressClass122 + `
`
			podDisruptionBudgetControllerYAML = `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: nginx-ingress
      component: controller
      release: addons
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        cpu: 25m
        memory: 100Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx-ingress-controller
  updatePolicy:
    updateMode: Auto
status: {}
`
			deploymentBackendYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: nginx-ingress-k8s-backend
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: nginx-ingress
      component: nginx-ingress-k8s-backend
      release: addons
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: nginx-ingress-k8s-backend
        release: addons
    spec:
      containers:
      - image: ` + imageDefaultBackend + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /healthy
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        name: nginx-ingress-k8s-backend
        ports:
        - containerPort: 8080
          protocol: TCP
        resources:
          limits:
            cpu: 100m
            memory: 100Mi
          requests:
            cpu: 20m
            memory: 20Mi
      priorityClassName: gardener-shoot-controlplane
      securityContext:
        fsGroup: 65534
        runAsUser: 65534
      terminationGracePeriodSeconds: 60
status: {}
`
			deploymentControllerYAMLFor = func(kubernetesVersion *semver.Version) string {
				out := `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  replicas: 3
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: nginx-ingress
      component: controller
      release: addons
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
        security.gardener.cloud/trigger: rollout
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: controller
        release: addons
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: nginx-ingress
                  component: controller
                  release: addons
              topologyKey: kubernetes.io/hostname
            weight: 100
      containers:
      - args:
        - /nginx-ingress-controller
        - --default-backend-service=` + namespace + `/nginx-ingress-k8s-backend
        - --enable-ssl-passthrough=true
        - --publish-service=` + namespace + `/nginx-ingress-controller
        - --election-id=ingress-controller-seed-leader
        - --update-status=true
        - --annotations-prefix=nginx.ingress.kubernetes.io
        - --configmap=` + namespace + `/` + configMapName + ``

				if version.ConstraintK8sGreaterEqual122.Check(kubernetesVersion) {
					out += `
        - --ingress-class=` + v1beta1constants.SeedNginxIngressClass122 + `
        - --controller-class=k8s.io/` + v1beta1constants.SeedNginxIngressClass122 + ``
				} else {
					out += `
        - --ingress-class=` + v1beta1constants.SeedNginxIngressClass + ``
				}

				out += `
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        image: ` + imageController + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 40
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: nginx-ingress-controller
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
        - containerPort: 443
          name: https
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 40
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            cpu: 400m
            memory: 1500Mi
          requests:
            cpu: 100m
            memory: 375Mi
        securityContext:
          allowPrivilegeEscalation: true
          capabilities:
            add:
            - NET_BIND_SERVICE
            drop:
            - ALL
          runAsUser: 101
      priorityClassName: gardener-shoot-controlplane
      serviceAccountName: nginx-ingress
      terminationGracePeriodSeconds: 60
status: {}
`
				return out
			}
		)

		JustBeforeEach(func() {
			nginxIngress = New(c, namespace, values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			Expect(nginxIngress.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.StringPtr("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.BoolPtr(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))

			Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_seed_nginx-ingress.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_seed_nginx-ingress.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["role__"+namespace+"__nginx-ingress.yaml"])).To(Equal(roleBackendYAML))
			Expect(string(managedResourceSecret.Data["rolebinding__"+namespace+"__nginx-ingress.yaml"])).To(Equal(roleBindingBackendYAML))
			Expect(string(managedResourceSecret.Data["service__"+namespace+"__nginx-ingress-controller.yaml"])).To(Equal(serviceControllerYAML))
			Expect(string(managedResourceSecret.Data["service__"+namespace+"__nginx-ingress-k8s-backend.yaml"])).To(Equal(serviceBackendYAML))
			Expect(string(managedResourceSecret.Data["serviceaccount__"+namespace+"__nginx-ingress.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__nginx-ingress-controller.yaml"])).To(Equal(podDisruptionBudgetControllerYAML))
			Expect(string(managedResourceSecret.Data["verticalpodautoscaler__"+namespace+"__nginx-ingress-controller.yaml"])).To(Equal(vpaYAML))
			Expect(string(managedResourceSecret.Data["configmap__"+namespace+"__"+configMapName+".yaml"])).To(Equal(configMapYAML))
			Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__nginx-ingress-k8s-backend.yaml"])).To(Equal(deploymentBackendYAML))
		})

		Context("Kubernetes version >= 1.22", func() {
			BeforeEach(func() {
				values.KubernetesVersion, _ = semver.NewVersion("1.24")
			})

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__nginx-ingress-controller.yaml"])).To(Equal(deploymentControllerYAMLFor(values.KubernetesVersion)))
				Expect(string(managedResourceSecret.Data["ingressclass____"+v1beta1constants.SeedNginxIngressClass122+".yaml"])).To(Equal(ingressClassYAML))
			})
		})

		Context("Kubernetes version < 1.22", func() {
			BeforeEach(func() {
				values.KubernetesVersion, _ = semver.NewVersion("1.18")
			})

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__nginx-ingress-controller.yaml"])).To(Equal(deploymentControllerYAMLFor(values.KubernetesVersion)))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			nginxIngress = New(c, namespace, Values{})

			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(nginxIngress.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			nginxIngress = New(c, namespace, Values{})

			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(nginxIngress.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
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
				}))
				Expect(nginxIngress.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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
				}))
				Expect(nginxIngress.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(nginxIngress.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(nginxIngress.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
