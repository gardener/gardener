// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dependencywatchdog_test

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	. "github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("DependencyWatchdog", func() {
	var (
		ctx = context.Background()

		namespace = "some-namespace"
		image     = "some-image:some-tag"

		c                client.Client
		dwd              component.DeployWaiter
		manifests        []string
		expectedManifest []string

		kubernetesVersion *semver.Version
	)

	BeforeEach(func() {
		kubernetesVersion = semver.MustParse("1.25.0")

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#Deploy, #Destroy", func() {
		testSuite := func(values BootstrapperValues, configMapDataHash string) {
			var (
				managedResource       *resourcesv1alpha1.ManagedResource
				managedResourceSecret *corev1.Secret

				dwdName       = fmt.Sprintf("dependency-watchdog-%s", values.Role)
				configMapName = dwdName + "-config-" + configMapDataHash

				serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
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
  name: gardener.cloud:` + dwdName + `
rules:`
					if role == RoleWeeder {
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

					if role == RoleProber {
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
  - patch
- apiGroups:
  - machine.sapcloud.io
  resources:
  - machines
  verbs:
  - get
  - list
  - watch
`
					}

					return out
				}

				clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:` + dwdName + `
subjects:
- kind: ServiceAccount
  name: ` + dwdName + `
  namespace: ` + namespace + `
`

				roleYAMLFor = func(role Role) string {
					out := `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `
  namespace: ` + namespace + `
rules:`
					if role == RoleWeeder {
						out += `
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - dwd-weeder-leader-election
  resources:
  - leases
  verbs:
  - get
  - watch
  - update
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - get
  - update
  - patch
`
					}

					if role == RoleProber {
						out += `
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - dwd-prober-leader-election
  resources:
  - leases
  verbs:
  - get
  - watch
  - update
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - get
  - update
  - patch
`
					}
					return out
				}

				roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:` + dwdName + `
  namespace: ` + namespace + `
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gardener.cloud:` + dwdName + `
subjects:
- kind: ServiceAccount
  name: ` + dwdName + `
  namespace: ` + namespace + `
`

				configMapYAMLFor = func(role Role) string {
					out := `apiVersion: v1
data:
  dep-config.yaml: |`

					if role == RoleWeeder {
						out += `
    servicesAndDependantSelectors: null
`
					}

					if role == RoleProber {
						out += `
    dependentResourceInfos: null
    kubeConfigSecretName: ""
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
    app: ` + dwdName + `
    high-availability-config.resources.gardener.cloud/type: controller
  name: ` + dwdName + `
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: ` + dwdName + `
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
      creationTimestamp: null
      labels:
        app: ` + dwdName

					if role == RoleWeeder {
						out += `
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed`
					}

					if role == RoleProber {
						out += `
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-private-networks: allowed
        networking.gardener.cloud/to-public-networks: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
        networking.resources.gardener.cloud/to-all-istio-ingresses-istio-ingressgateway-tcp-9443: allowed
        networking.resources.gardener.cloud/to-all-shoots-kube-apiserver-tcp-443: allowed`
					}

					out += `
    spec:
      containers:
      - command:`

					if role == RoleWeeder {
						out += `
        - /usr/local/bin/dependency-watchdog
        - weeder
        - --config-file=/etc/dependency-watchdog/config/dep-config.yaml
        - --enable-leader-election=true
`
					}

					if role == RoleProber {
						out += `
        - /usr/local/bin/dependency-watchdog
        - prober
        - --config-file=/etc/dependency-watchdog/config/dep-config.yaml
        - --kube-api-qps=20.0
        - --kube-api-burst=100
        - --zap-log-level=INFO
        - --enable-leader-election=true
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
            memory: 512Mi
          requests:
            cpu: 200m
            memory: 256Mi
        volumeMounts:
        - mountPath: /etc/dependency-watchdog/config
          name: config
          readOnly: true
      priorityClassName: gardener-system-800
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
					out := `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: ` + dwdName + `
  namespace: ` + namespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
`

					if role == RoleWeeder {
						out += `        memory: 25Mi`
					}

					if role == RoleProber {
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

				podDisruptionYAMLFor = func(k8sGreaterEquals126 bool) string {
					out := `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    app: ` + dwdName + `
  name: ` + dwdName + `
  namespace: ` + namespace + `
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: ` + dwdName + `
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
			)

			JustBeforeEach(func() {
				values.KubernetesVersion = kubernetesVersion
				dwd = NewBootstrapper(c, namespace, values)

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

			Context("Different kubernetes versions", func() {
				JustBeforeEach(func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

					Expect(dwd.Deploy(ctx)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					expectedMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResource.Name,
							Namespace:       managedResource.Namespace,
							Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
							ResourceVersion: "1",
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

					var err error
					manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
					Expect(err).NotTo(HaveOccurred())

					expectedManifest = []string{
						clusterRoleYAMLFor(values.Role),
						clusterRoleBindingYAML,
						configMapYAMLFor(values.Role),
						deploymentYAMLFor(values.Role),
						roleYAMLFor(values.Role),
						roleBindingYAML,
						serviceAccountYAML,
						vpaYAMLFor(values.Role),
					}
				})

				Context("kubernetes versions < 1.26", func() {
					It("should successfully deploy all resources for role "+string(values.Role), func() {
						expectedManifest = append(expectedManifest, podDisruptionYAMLFor(false))
						Expect(manifests).To(ConsistOf(expectedManifest))
					})
				})

				Context("kubernetes versions >= 1.26", func() {
					BeforeEach(func() {
						kubernetesVersion = semver.MustParse("1.26.4")
					})

					It("should successfully deploy all resources for role "+string(values.Role), func() {
						expectedManifest = append(expectedManifest, podDisruptionYAMLFor(true))
						Expect(manifests).To(ConsistOf(expectedManifest))
					})
				})
			})

			It("should successfully destroy all resources for role "+string(values.Role), func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(dwd.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		}

		Describe("RoleWeeder", func() {
			testSuite(BootstrapperValues{Role: RoleWeeder, Image: image}, "d1e2e712")
		})

		Describe("RoleProber", func() {
			testSuite(BootstrapperValues{Role: RoleProber, Image: image}, "3c10a163")
		})
	})

	Context("waiting functions", func() {
		var (
			role                = Role("some-role")
			managedResourceName = fmt.Sprintf("dependency-watchdog-%s", role)
			managedResource     *resourcesv1alpha1.ManagedResource

			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			dwd = NewBootstrapper(c, namespace, BootstrapperValues{Role: role})
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
				},
			}

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
				Expect(dwd.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
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

				Expect(dwd.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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

				Expect(dwd.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(dwd.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(dwd.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
