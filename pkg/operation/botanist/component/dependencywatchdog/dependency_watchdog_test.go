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

package dependencywatchdog_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("DependencyWatchdog", func() {
	var (
		ctx = context.TODO()

		namespace = "some-namespace"
		image     = "some-image:some-tag"

		c   client.Client
		dwd component.DeployWaiter
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#Deploy, #Destroy", func() {
		testSuite := func(values Values, configMapDataHash string) {
			var (
				managedResource       *resourcesv1alpha1.ManagedResource
				managedResourceSecret *corev1.Secret

				dwdName       = fmt.Sprintf("dependency-watchdog-%s", values.Role)
				configMapName = dwdName + "-config-" + configMapDataHash

				serviceAccountYAML = `apiVersion: v1
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: ` + dwdName + `
  namespace: ` + namespace + `
`

				clusterRoleYAMLFor = func(role Role) string {
					out := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `:cluster-role
rules:`
					if role == RoleEndpoint {
						out += `
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
  - delete
`
					}

					if role == RoleProbe {
						out += `
- apiGroups:
  - extensions.gardener.cloud
  resources:
  - clusters
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - namespaces
  - secrets
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - apps
  resources:
  - deployments
  - deployments/scale
  verbs:
  - get
  - list
  - watch
  - update
`
					}

					return out
				}

				clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `:cluster-role-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:` + dwdName + `:cluster-role
subjects:
- kind: ServiceAccount
  name: ` + dwdName + `
  namespace: ` + namespace + `
`

				roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `:role
  namespace: ` + namespace + `
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - events
  verbs:
  - create
  - get
  - update
  - patch
`
				roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `:role-binding
  namespace: ` + namespace + `
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gardener.cloud:` + dwdName + `:role
subjects:
- kind: ServiceAccount
  name: ` + dwdName + `
  namespace: ` + namespace + `
`

				configMapYAMLFor = func(role Role) string {
					out := `apiVersion: v1
data:
  dep-config.yaml: |`

					if role == RoleEndpoint {
						out += `
    namespace: ""
    services: null
`
					}

					if role == RoleProbe {
						out += `
    namespace: ""
    probes: null
`
					}

					out += `immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: ` + dwdName + `
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapName + `
  namespace: ` + namespace + `
`

					return out
				}

				deploymentYAMLFor = func(role Role) string {
					out := `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
  creationTimestamp: null
  labels:
    role: ` + dwdName + `
  name: ` + dwdName + `
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      role: ` + dwdName + `
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
      creationTimestamp: null`

					if role == RoleEndpoint {
						out += `
      labels:
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-seed-apiserver: allowed
`
					}

					if role == RoleProbe {
						out += `
      labels:
        networking.gardener.cloud/to-all-shoot-apiservers: allowed
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-private-networks: allowed
        networking.gardener.cloud/to-public-networks: allowed
        networking.gardener.cloud/to-seed-apiserver: allowed
`
					}

					out += `        role: ` + dwdName + `
    spec:
      containers:
      - command:`

					if role == RoleEndpoint {
						out += `
        - /usr/local/bin/dependency-watchdog
        - --config-file=/etc/dependency-watchdog/config/dep-config.yaml
        - --deployed-namespace=` + namespace + `
        - --watch-duration=5m
`
					}

					if role == RoleProbe {
						out += `
        - /usr/local/bin/dependency-watchdog
        - probe
        - --config-file=/etc/dependency-watchdog/config/dep-config.yaml
        - --deployed-namespace=some-namespace
        - --qps=20.0
        - --burst=100
        - --v=4
`
					}

					out += `        image: ` + image + `
        imagePullPolicy: IfNotPresent
        name: dependency-watchdog
        ports:
        - containerPort: 9643
          name: metrics
          protocol: TCP
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 200m
            memory: 256Mi
        volumeMounts:
        - mountPath: /etc/dependency-watchdog/config
          name: config
          readOnly: true
      serviceAccountName: ` + dwdName + `
      terminationGracePeriodSeconds: 5
      volumes:
      - configMap:
          name: ` + configMapName + `
        name: config
status: {}
`

					return out
				}

				vpaYAMLFor = func(role Role) string {
					out := `apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: ` + dwdName + `-vpa
  namespace: ` + namespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        cpu: 25m
`

					if role == RoleEndpoint {
						out += `        memory: 25Mi`
					}

					if role == RoleProbe {
						out += `        memory: 50Mi`
					}

					out += `
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ` + dwdName + `
  updatePolicy:
    updateMode: Auto
status: {}
`

					return out
				}
			)

			BeforeEach(func() {
				dwd = New(c, namespace, values)

				managedResource = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dwdName,
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

			It("should successfully deploy all resources for role "+string(values.Role), func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

				Expect(dwd.Deploy(ctx)).To(Succeed())

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
				Expect(managedResourceSecret.Data).To(HaveLen(8))
				Expect(managedResourceSecret.Data["clusterrole____gardener.cloud_"+dwdName+"_cluster-role.yaml"]).To(DeepEqual([]byte(clusterRoleYAMLFor(values.Role))))
				Expect(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_"+dwdName+"_cluster-role-binding.yaml"]).To(DeepEqual([]byte(clusterRoleBindingYAML)))
				Expect(managedResourceSecret.Data["configmap__"+namespace+"__"+configMapName+".yaml"]).To(DeepEqual([]byte(configMapYAMLFor(values.Role))))
				Expect(managedResourceSecret.Data["deployment__"+namespace+"__"+dwdName+".yaml"]).To(DeepEqual([]byte(deploymentYAMLFor(values.Role))))
				Expect(managedResourceSecret.Data["role__"+namespace+"__gardener.cloud_"+dwdName+"_role.yaml"]).To(DeepEqual([]byte(roleYAML)))
				Expect(managedResourceSecret.Data["rolebinding__"+namespace+"__gardener.cloud_"+dwdName+"_role-binding.yaml"]).To(DeepEqual([]byte(roleBindingYAML)))
				Expect(managedResourceSecret.Data["serviceaccount__"+namespace+"__"+dwdName+".yaml"]).To(DeepEqual([]byte(serviceAccountYAML)))
				Expect(managedResourceSecret.Data["verticalpodautoscaler__"+namespace+"__"+dwdName+"-vpa.yaml"]).To(DeepEqual([]byte(vpaYAMLFor(values.Role))))
			})

			It("should successfully destroy all resources for role "+string(values.Role), func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(dwd.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
			})
		}

		Describe("RoleEndpoint", func() {
			testSuite(Values{Role: RoleEndpoint, Image: image}, "885b78df")
		})

		Describe("RoleProbe", func() {
			testSuite(Values{Role: RoleProbe, Image: image}, "07491e14")
		})
	})

	Context("waiting functions", func() {
		var (
			role                = Role("some-role")
			managedResourceName = fmt.Sprintf("dependency-watchdog-%s", role)
			managedResource     *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			dwd = New(c, namespace, Values{Role: role})
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
				},
			}
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(dwd.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

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

				Expect(dwd.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

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

				Expect(dwd.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			timeNowFunc := func() time.Time { return time.Time{} }

			It("should fail when the wait for the managed resource deletion times out", func() {
				oldTimeNow := gutil.TimeNow
				defer func() { gutil.TimeNow = oldTimeNow }()
				gutil.TimeNow = timeNowFunc

				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(dwd.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully wait for the deletion", func() {
				oldTimeNow := gutil.TimeNow
				defer func() { gutil.TimeNow = oldTimeNow }()
				gutil.TimeNow = timeNowFunc

				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Second

				interval := time.Millisecond
				oldIntervalWait := managedresources.IntervalWait
				defer func() { managedresources.IntervalWait = oldIntervalWait }()
				managedresources.IntervalWait = interval

				go func() {
					Expect(c.Create(ctx, managedResource)).To(Succeed())
					time.Sleep(10 * interval)
					Expect(c.Delete(ctx, managedResource)).To(Succeed())
				}()

				Expect(dwd.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
