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

package nginxingressshoot

import (
	"context"

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
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NginxIngress", func() {
	var (
		ctx = context.TODO()

		managedResourceName  = "shoot-addon-nginx-ingress"
		namespace            = "some-namespace"
		nginxControllerImage = "some-image:some-tag"
		defaultBackendImage  = "some-image2:some-tag2"
		configMapData        = map[string]string{
			"foo":  "bar",
			"dot":  "3",
			"dash": "false",
		}

		c         client.Client
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		values = Values{
			NginxControllerImage: nginxControllerImage,
			DefaultBackendImage:  defaultBackendImage,
			ConfigData:           configMapData,
			PSPDisabled:          true,
			KubeAPIServerHost:    pointer.String("foo.bar"),
		}

		configMapName = "addons-nginx-ingress-controller"

		configMapYAML = `apiVersion: v1
data:
  dash: "false"
  dot: "3"
  foo: bar
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    release: addons
  name: ` + configMapName + `
  namespace: kube-system
`

		ingressClassYAML = `apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    gardener.cloud/role: optional-addon
    origin: gardener
    release: addons
  name: nginx
spec:
  controller: k8s.io/nginx
`

		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
  namespace: kube-system
`

		networkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows all egress and ingress traffic for the nginx-ingress
      controller.
  creationTimestamp: null
  labels:
    origin: gardener
  name: gardener.cloud--allow-to-from-nginx
  namespace: kube-system
spec:
  egress:
  - {}
  ingress:
  - {}
  podSelector:
    matchLabels:
      app: nginx-ingress
      component: controller
      release: addons
  policyTypes:
  - Ingress
  - Egress
status: {}
`

		serviceBackendYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
    release: addons
  name: addons-nginx-ingress-nginx-ingress-k8s-backend
  namespace: kube-system
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

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
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
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - list
  - watch
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
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
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: addons-nginx-ingress
subjects:
- kind: ServiceAccount
  name: addons-nginx-ingress
  namespace: kube-system
`

		serviceControllerYAML = `apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-proxy-protocol: '*'
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    release: addons
  name: addons-nginx-ingress-controller
  namespace: kube-system
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

		deploymentBackendYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
    gardener.cloud/role: optional-addon
    origin: gardener
    release: addons
  name: addons-nginx-ingress-nginx-ingress-k8s-backend
  namespace: kube-system
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
        gardener.cloud/role: optional-addon
        origin: gardener
        release: addons
    spec:
      containers:
      - image: ` + defaultBackendImage + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /healthy
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        name: nginx-ingress-nginx-ingress-k8s-backend
        ports:
        - containerPort: 8080
          protocol: TCP
        resources:
          limits:
            memory: 100Mi
          requests:
            cpu: 20m
            memory: 20Mi
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: gardener-shoot-system-600
      securityContext:
        fsGroup: 65534
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      terminationGracePeriodSeconds: 60
status: {}
`

		deploymentControllerYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    gardener.cloud/role: optional-addon
    origin: gardener
    release: addons
  name: addons-nginx-ingress-controller
  namespace: kube-system
spec:
  replicas: 1
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
        checksum/config: ` + utils.ComputeChecksum(configMapData) + `
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: controller
        gardener.cloud/role: optional-addon
        origin: gardener
        release: addons
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
        - --default-backend-service=kube-system/addons-nginx-ingress-nginx-ingress-k8s-backend
        - --enable-ssl-passthrough=true
        - --publish-service=kube-system/addons-nginx-ingress-controller
        - --election-id=ingress-controller-leader
        - --update-status=true
        - --annotations-prefix=nginx.ingress.kubernetes.io
        - --ingress-class=nginx
        - --configmap=kube-system/` + configMapName + `
        - --controller-class=k8s.io/nginx
        - --watch-ingress-without-class=true
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        image: ` + nginxControllerImage + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 10
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
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            memory: 4Gi
          requests:
            cpu: 100m
            memory: 100Mi
        securityContext:
          allowPrivilegeEscalation: true
          capabilities:
            add:
            - NET_BIND_SERVICE
            - SYS_CHROOT
            drop:
            - ALL
          runAsUser: 101
          seccompProfile:
            type: Unconfined
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: gardener-shoot-system-600
      restartPolicy: Always
      schedulerName: default-scheduler
      serviceAccountName: addons-nginx-ingress
      terminationGracePeriodSeconds: 60
status: {}
`

		roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
  namespace: kube-system
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
  resources:
  - endpoints
  verbs:
  - create
  - get
  - update
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - ingress-controller-leader
  resources:
  - leases
  verbs:
  - get
  - update
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
`

		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: addons-nginx-ingress
subjects:
- kind: ServiceAccount
  name: addons-nginx-ingress
  namespace: kube-system
`

		roleBindingPSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: gardener.cloud:psp:addons-nginx-ingress
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:psp:privileged
subjects:
- kind: ServiceAccount
  name: addons-nginx-ingress
  namespace: kube-system
`

		vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: addons-nginx-ingress-controller
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 128Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: addons-nginx-ingress-controller
  updatePolicy:
    updateMode: Auto
status: {}
`
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
		JustBeforeEach(func() {
			component = New(c, namespace, values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

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

			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__addons-nginx-ingress.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____addons-nginx-ingress.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____addons-nginx-ingress.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__addons-nginx-ingress-controller.yaml"])).To(Equal(serviceControllerYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__addons-nginx-ingress-nginx-ingress-k8s-backend.yaml"])).To(Equal(serviceBackendYAML))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__"+configMapName+".yaml"])).To(Equal(configMapYAML))
			Expect(string(managedResourceSecret.Data["role__kube-system__addons-nginx-ingress.yaml"])).To(Equal(roleYAML))
			Expect(string(managedResourceSecret.Data["rolebinding__kube-system__addons-nginx-ingress.yaml"])).To(Equal(roleBindingYAML))
			Expect(string(managedResourceSecret.Data["deployment__kube-system__addons-nginx-ingress-nginx-ingress-k8s-backend.yaml"])).To(Equal(deploymentBackendYAML))
			Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-to-from-nginx.yaml"])).To(Equal(networkPolicyYAML))
			Expect(string(managedResourceSecret.Data["ingressclass____nginx.yaml"])).To(Equal(ingressClassYAML))
			Expect(string(managedResourceSecret.Data["deployment__kube-system__addons-nginx-ingress-controller.yaml"])).To(Equal(deploymentControllerYAML))
		})

		Context("w/ VPA and PSP enabled", func() {
			BeforeEach(func() {
				values.VPAEnabled = true
				values.PSPDisabled = false
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(14))

				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__addons-nginx-ingress-controller.yaml"])).To(Equal(vpaYAML))
				Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_addons-nginx-ingress.yaml"])).To(Equal(roleBindingPSPYAML))
			})
		})

		Context("w/ VPA and PSP disabled", func() {
			BeforeEach(func() {
				values.VPAEnabled = true
				values.PSPDisabled = true
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(13))

				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__addons-nginx-ingress-controller.yaml"])).To(Equal(vpaYAML))
			})
		})

		Context("w/o VPA and PSP enabled", func() {
			BeforeEach(func() {
				values.VPAEnabled = false
				values.PSPDisabled = false
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(13))

				Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_addons-nginx-ingress.yaml"])).To(Equal(roleBindingPSPYAML))
			})
		})

		Context("w/o VPA and PSP disabled", func() {
			BeforeEach(func() {
				values.VPAEnabled = false
				values.PSPDisabled = true
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(12))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			component = New(c, namespace, Values{})

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
			component = New(c, namespace, Values{})

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
