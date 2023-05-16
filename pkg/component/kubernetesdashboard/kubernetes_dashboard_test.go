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

package kubernetesdashboard_test

import (
	"context"

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetesdashboard"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("Kubernetes Dashboard", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-addon-kubernetes-dashboard"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"
		scraperImage        = "scraper-image:scraper-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		namespacesYAML = `apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/purpose: kubernetes-dashboard
  name: kubernetes-dashboard
spec: {}
status: {}
`

		roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
rules:
- apiGroups:
  - ""
  resourceNames:
  - kubernetes-dashboard-key-holder
  - kubernetes-dashboard-certs
  - kubernetes-dashboard-csrf
  resources:
  - secrets
  verbs:
  - get
  - update
  - delete
- apiGroups:
  - ""
  resourceNames:
  - heapster
  - dashboard-metrics-scraper
  resources:
  - services
  verbs:
  - proxy
- apiGroups:
  - ""
  resourceNames:
  - kubernetes-dashboard-settings
  resources:
  - configmaps
  verbs:
  - get
  - update
- apiGroups:
  - ""
  resourceNames:
  - heapster
  - 'http:heapster:'
  - 'https:heapster:'
  - dashboard-metrics-scraper
  - http:dashboard-metrics-scraper
  resources:
  - services/proxy
  verbs:
  - get
`

		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kubernetes-dashboard
subjects:
- kind: ServiceAccount
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
`

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
rules:
- apiGroups:
  - metrics.k8s.io
  resources:
  - pods
  - nodes
  verbs:
  - get
  - list
  - watch
`

		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubernetes-dashboard
subjects:
- kind: ServiceAccount
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
`

		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
`

		secretCertsYAML = `apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-certs
  namespace: kubernetes-dashboard
type: Opaque
`

		secretCSRFYAML = `apiVersion: v1
data:
  csrf: ""
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-csrf
  namespace: kubernetes-dashboard
type: Opaque
`

		secretKeyHolderYAML = `apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-key-holder
  namespace: kubernetes-dashboard
type: Opaque
`

		configMapYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-settings
  namespace: kubernetes-dashboard
`

		deploymentDashboardYAMLFor = func(apiserverHost *string, authenticationMode string) string {
			out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: optional-addon
    k8s-app: kubernetes-dashboard
    origin: gardener
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      k8s-app: kubernetes-dashboard
  strategy:
    rollingUpdate:
      maxSurge: 0
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        gardener.cloud/role: optional-addon
        k8s-app: kubernetes-dashboard
        origin: gardener
    spec:
      containers:
      - args:
        - --auto-generate-certificates
        - --authentication-mode=` + authenticationMode + `
        - --namespace=kubernetes-dashboard
`

			if apiserverHost != nil {
				out += `        env:
        - name: KUBERNETES_SERVICE_HOST
          value: ` + *apiserverHost + `
`
			}

			out += `        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 30
          timeoutSeconds: 30
        name: kubernetes-dashboard
        ports:
        - containerPort: 8443
          protocol: TCP
        resources:
          limits:
            memory: 256Mi
          requests:
            cpu: 50m
            memory: 50Mi
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
        volumeMounts:
        - mountPath: /certs
          name: kubernetes-dashboard-certs
        - mountPath: /tmp
          name: tmp-volume
      securityContext:
        fsGroup: 1
        runAsGroup: 2001
        runAsUser: 1001
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      serviceAccountName: kubernetes-dashboard
      volumes:
      - name: kubernetes-dashboard-certs
        secret:
          secretName: kubernetes-dashboard-certs
      - emptyDir: {}
        name: tmp-volume
status: {}
`
			return out
		}

		deploymentMetricsScraperYAMLFor = func(kubernetesVersion *semver.Version) string {
			out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: optional-addon
    k8s-app: dashboard-metrics-scraper
    origin: gardener
  name: dashboard-metrics-scraper
  namespace: kubernetes-dashboard
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      k8s-app: dashboard-metrics-scraper
  strategy: {}
  template:
    metadata:
`
			if version.ConstraintK8sLessEqual122.Check(kubernetesVersion) {
				out += `      annotations:
        seccomp.security.alpha.kubernetes.io/pod: runtime/default
`
			}
			out += `      creationTimestamp: null
      labels:
        gardener.cloud/role: optional-addon
        k8s-app: dashboard-metrics-scraper
        origin: gardener
    spec:
      containers:
      - image: scraper-image:scraper-tag
        livenessProbe:
          httpGet:
            path: /
            port: 8000
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 30
        name: dashboard-metrics-scraper
        ports:
        - containerPort: 8000
          protocol: TCP
        resources: {}
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsGroup: 2001
          runAsUser: 1001
`

			if version.ConstraintK8sGreaterEqual123.Check(kubernetesVersion) {
				out += `          seccompProfile:
            type: RuntimeDefault
`
			}

			out += `        volumeMounts:
        - mountPath: /tmp
          name: tmp-volume
      securityContext:
        fsGroup: 1
        supplementalGroups:
        - 1
      serviceAccountName: kubernetes-dashboard
      volumes:
      - emptyDir: {}
        name: tmp-volume
status: {}
`
			return out
		}

		serviceDashboardYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
spec:
  ports:
  - port: 443
    targetPort: 8443
  selector:
    k8s-app: kubernetes-dashboard
status:
  loadBalancer: {}
`

		serviceMetricsScraperYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    k8s-app: dashboard-metrics-scraper
  name: dashboard-metrics-scraper
  namespace: kubernetes-dashboard
spec:
  ports:
  - port: 8000
    targetPort: 8000
  selector:
    k8s-app: dashboard-metrics-scraper
status:
  loadBalancer: {}
`

		vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: kubernetes-dashboard
  updatePolicy:
    updateMode: Auto
status: {}
`
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:               image,
			MetricsScraperImage: scraperImage,
			KubernetesVersion:   semver.MustParse("1.24.0"),
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
			vpaEnabled bool
		)

		BeforeEach(func() {
			vpaEnabled = false
		})

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

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
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			if vpaEnabled {
				Expect(managedResourceSecret.Data).To(HaveLen(15))
				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(vpaYAML))
			} else {
				Expect(managedResourceSecret.Data).To(HaveLen(14))
			}

			Expect(string(managedResourceSecret.Data["namespace____kubernetes-dashboard.yaml"])).To(Equal(namespacesYAML))
			Expect(string(managedResourceSecret.Data["role__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(roleYAML))
			Expect(string(managedResourceSecret.Data["rolebinding__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(roleBindingYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____kubernetes-dashboard.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____kubernetes-dashboard.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["serviceaccount__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["secret__kubernetes-dashboard__kubernetes-dashboard-certs.yaml"])).To(Equal(secretCertsYAML))
			Expect(string(managedResourceSecret.Data["secret__kubernetes-dashboard__kubernetes-dashboard-csrf.yaml"])).To(Equal(secretCSRFYAML))
			Expect(string(managedResourceSecret.Data["secret__kubernetes-dashboard__kubernetes-dashboard-key-holder.yaml"])).To(Equal(secretKeyHolderYAML))
			Expect(string(managedResourceSecret.Data["configmap__kubernetes-dashboard__kubernetes-dashboard-settings.yaml"])).To(Equal(configMapYAML))
			Expect(string(managedResourceSecret.Data["service__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(serviceDashboardYAML))
			Expect(string(managedResourceSecret.Data["service__kubernetes-dashboard__dashboard-metrics-scraper.yaml"])).To(Equal(serviceMetricsScraperYAML))
		})

		Context("kubernetes version > 1.22.0", func() {
			Context("w/o apiserver host, w/o authentication mode, w/o vpa", func() {
				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(deploymentDashboardYAMLFor(nil, "")))
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__dashboard-metrics-scraper.yaml"])).To(Equal(deploymentMetricsScraperYAMLFor(values.KubernetesVersion)))
				})
			})

			Context("w/ apiserver host, w/ authentication mode, w/ vpa", func() {
				var (
					apiserverHost      = "apiserver.host"
					authenticationMode = "token"
				)

				BeforeEach(func() {
					vpaEnabled = true
					values.VPAEnabled = true
					values.APIServerHost = &apiserverHost
					values.AuthenticationMode = authenticationMode
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(deploymentDashboardYAMLFor(&apiserverHost, authenticationMode)))
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__dashboard-metrics-scraper.yaml"])).To(Equal(deploymentMetricsScraperYAMLFor(values.KubernetesVersion)))
				})
			})
		})

		Context("kubernetes version <= 1.22.0", func() {
			Context("w/o apiserver host, w/o authentication mode, w/o vpa", func() {
				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(deploymentDashboardYAMLFor(nil, "")))
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__dashboard-metrics-scraper.yaml"])).To(Equal(deploymentMetricsScraperYAMLFor(values.KubernetesVersion)))
				})
			})

			Context("w/ apiserver host w/ authentication mode, w/ vpa", func() {
				var (
					apiserverHost      = "apiserver.host"
					authenticationMode = "token"
				)

				BeforeEach(func() {
					vpaEnabled = true
					values.VPAEnabled = true
					values.APIServerHost = &apiserverHost
					values.AuthenticationMode = authenticationMode
					values.KubernetesVersion = semver.MustParse("1.22.0")
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__kubernetes-dashboard.yaml"])).To(Equal(deploymentDashboardYAMLFor(&apiserverHost, authenticationMode)))
					Expect(string(managedResourceSecret.Data["deployment__kubernetes-dashboard__dashboard-metrics-scraper.yaml"])).To(Equal(deploymentMetricsScraperYAMLFor(values.KubernetesVersion)))
				})
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
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
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

				expectedChecksum := "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9"
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
				}
				Expect(c.Create(ctx, secret)).To(Succeed())
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{{Name: secret.Name}},
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
						SecretsDataChecksum: &expectedChecksum,
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
