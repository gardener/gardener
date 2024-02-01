// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd_test

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Etcd", func() {
	var (
		c            client.Client
		bootstrapper component.DeployWaiter
		etcdConfig   *config.ETCDConfig

		ctx                      = context.TODO()
		namespace                = "shoot--foo--bar"
		kubernetesVersion        *semver.Version
		etcdDruidImage           = "etcd/druid:1.2.3"
		imageVectorOverwrite     *string
		imageVectorOverwriteFull = ptr.To("some overwrite")

		priorityClassName = "some-priority-class"

		featureGates map[string]bool

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource

		managedResourceName       = "etcd-druid"
		managedResourceSecretName = "managedresource-" + managedResourceName
	)

	JustBeforeEach(func() {
		etcdConfig = &config.ETCDConfig{
			ETCDController: &config.ETCDController{
				Workers: ptr.To(int64(25)),
			},
			CustodianController: &config.CustodianController{
				Workers: ptr.To(int64(3)),
			},
			BackupCompactionController: &config.BackupCompactionController{
				Workers:                   ptr.To(int64(3)),
				EnableBackupCompaction:    ptr.To(true),
				EventsThreshold:           ptr.To(int64(1000000)),
				MetricsScrapeWaitDuration: &metav1.Duration{Duration: time.Second * 60},
				ActiveDeadlineDuration:    &metav1.Duration{Duration: time.Hour * 3},
			},
			FeatureGates: featureGates,
		}

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		bootstrapper = NewBootstrapper(c, namespace, kubernetesVersion, etcdConfig, etcdDruidImage, imageVectorOverwrite, priorityClassName)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			imageVectorOverwrite = nil
			featureGates = nil
			kubernetesVersion = semver.MustParse("1.25.0")
		})

		var (
			configMapName = "etcd-druid-imagevector-overwrite-4475dd36"

			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: etcd-druid
  namespace: ` + namespace + `
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: gardener.cloud:system:etcd-druid
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
  - delete
  - deletecollection
- apiGroups:
  - ""
  resources:
  - secrets
  - endpoints
  verbs:
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - get
  - list
  - watch
  - patch
  - update
- apiGroups:
  - ""
  resources:
  - serviceaccounts
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - roles
  - rolebindings
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - services
  - configmaps
  verbs:
  - get
  - list
  - patch
  - update
  - watch
  - create
  - delete
- apiGroups:
  - apps
  resources:
  - statefulsets
  verbs:
  - get
  - list
  - patch
  - update
  - watch
  - create
  - delete
- apiGroups:
  - batch
  resources:
  - jobs
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - druid.gardener.cloud
  resources:
  - etcds
  - etcdcopybackupstasks
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - druid.gardener.cloud
  resources:
  - etcds/status
  - etcds/finalizers
  - etcdcopybackupstasks/status
  - etcdcopybackupstasks/finalizers
  verbs:
  - get
  - update
  - patch
  - create
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
  - deletecollection
- apiGroups:
  - ""
  resources:
  - persistentvolumeclaims
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - policy
  resources:
  - poddisruptionbudgets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: gardener.cloud:system:etcd-druid
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:system:etcd-druid
subjects:
- kind: ServiceAccount
  name: etcd-druid
  namespace: ` + namespace + `
`
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: etcd-druid-vpa
  namespace: ` + namespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 100M
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: etcd-druid
  updatePolicy:
    updateMode: Auto
status: {}
`
			configMapImageVectorOverwriteYAML = `apiVersion: v1
data:
  images_overwrite.yaml: ` + *imageVectorOverwriteFull + `
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapName + `
  namespace: ` + namespace + `
`

			deploymentWithoutImageVectorOverwriteYAMLFor = func(useEtcdWrapper bool) string {
				out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    high-availability-config.resources.gardener.cloud/type: controller
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        gardener.cloud/role: etcd-druid
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
    spec:
      containers:
      - command:
        - /etcd-druid
        - --enable-leader-election=true
        - --ignore-operation-annotation=false
        - --disable-etcd-serviceaccount-automount=true
        - --workers=25
        - --custodian-workers=3
        - --compaction-workers=3
        - --enable-backup-compaction=true
        - --etcd-events-threshold=1000000
        - --metrics-scrape-wait-duration=1m0s
        - --active-deadline-duration=3h0m0s`
				if useEtcdWrapper {
					out += `
        - --feature-gates=UseEtcdWrapper=true`
				}
				out += `
        image: ` + etcdDruidImage + `
        imagePullPolicy: IfNotPresent
        name: etcd-druid
        ports:
        - containerPort: 8080
        resources:
          limits:
            memory: 512Mi
          requests:
            cpu: 50m
            memory: 128Mi
      priorityClassName: ` + priorityClassName + `
      serviceAccountName: etcd-druid
status: {}
`
				return out
			}
			deploymentWithImageVectorOverwriteYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    high-availability-config.resources.gardener.cloud/type: controller
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
      creationTimestamp: null
      labels:
        gardener.cloud/role: etcd-druid
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
    spec:
      containers:
      - command:
        - /etcd-druid
        - --enable-leader-election=true
        - --ignore-operation-annotation=false
        - --disable-etcd-serviceaccount-automount=true
        - --workers=25
        - --custodian-workers=3
        - --compaction-workers=3
        - --enable-backup-compaction=true
        - --etcd-events-threshold=1000000
        - --metrics-scrape-wait-duration=1m0s
        - --active-deadline-duration=3h0m0s
        env:
        - name: IMAGEVECTOR_OVERWRITE
          value: /charts_overwrite/images_overwrite.yaml
        image: ` + etcdDruidImage + `
        imagePullPolicy: IfNotPresent
        name: etcd-druid
        ports:
        - containerPort: 8080
        resources:
          limits:
            memory: 512Mi
          requests:
            cpu: 50m
            memory: 128Mi
        volumeMounts:
        - mountPath: /charts_overwrite
          name: imagevector-overwrite
          readOnly: true
      priorityClassName: ` + priorityClassName + `
      serviceAccountName: etcd-druid
      volumes:
      - configMap:
          name: ` + configMapName + `
        name: imagevector-overwrite
status: {}
`
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  annotations:
    networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports: '[{"protocol":"TCP","port":8080}]'
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    high-availability-config.resources.gardener.cloud/type: controller
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  ports:
  - name: metrics
    port: 8080
    protocol: TCP
    targetPort: 8080
  selector:
    gardener.cloud/role: etcd-druid
  type: ClusterIP
status:
  loadBalancer: {}
`
			podDisruptionYAMLFor = func(k8sGreaterEquals126 bool) string {
				out := `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
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
			serviceMonitorYAML = `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  creationTimestamp: null
  labels:
    prometheus: cache
  name: cache-etcd-druid
  namespace: ` + namespace + `
spec:
  endpoints:
  - metricRelabelings:
    - action: keep
      regex: ^(etcddruid_compaction_jobs_total|etcddruid_compaction_jobs_current|etcddruid_compaction_job_duration_seconds_bucket|etcddruid_compaction_job_duration_seconds_sum|etcddruid_compaction_job_duration_seconds_count|etcddruid_compaction_num_delta_events)$
      sourceLabels:
      - __name__
    port: metrics
  namespaceSelector: {}
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
`
		)

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))

			Expect(bootstrapper.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
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

			Expect(string(managedResourceSecret.Data["serviceaccount__"+namespace+"__etcd-druid.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_system_etcd-druid.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_system_etcd-druid.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["verticalpodautoscaler__"+namespace+"__etcd-druid-vpa.yaml"])).To(Equal(vpaYAML))
			Expect(string(managedResourceSecret.Data["service__"+namespace+"__etcd-druid.yaml"])).To(Equal(serviceYAML))
			Expect(string(managedResourceSecret.Data["servicemonitor__"+namespace+"__cache-etcd-druid.yaml"])).To(Equal(serviceMonitorYAML))
		})

		Context("w/o image vector overwrite", func() {
			JustBeforeEach(func() {
				Expect(managedResourceSecret.Data).To(HaveLen(8))
				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__etcd-druid.yaml"])).To(Equal(deploymentWithoutImageVectorOverwriteYAMLFor(false)))
			})

			Context("kubernetes versions < 1.26", func() {
				It("should successfully deploy all the resources (w/o image vector overwrite)", func() {
					Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__etcd-druid.yaml"])).To(Equal(podDisruptionYAMLFor(false)))
				})
			})

			Context("kubernetes versions >= 1.26", func() {
				BeforeEach(func() {
					kubernetesVersion = semver.MustParse("1.26.0")
				})

				It("should successfully deploy all the resources", func() {
					Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__etcd-druid.yaml"])).To(Equal(podDisruptionYAMLFor(true)))
				})
			})
		})

		Context("w/ image vector overwrite", func() {
			BeforeEach(func() {
				imageVectorOverwrite = imageVectorOverwriteFull
			})

			It("should successfully deploy all the resources (w/ image vector overwrite)", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(9))
				bootstrapper = NewBootstrapper(c, namespace, kubernetesVersion, etcdConfig, etcdDruidImage, imageVectorOverwriteFull, priorityClassName)

				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__etcd-druid.yaml"])).To(Equal(deploymentWithImageVectorOverwriteYAML))
				Expect(string(managedResourceSecret.Data["configmap__"+namespace+"__"+configMapName+".yaml"])).To(Equal(configMapImageVectorOverwriteYAML))
				Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__etcd-druid.yaml"])).To(Equal(podDisruptionYAMLFor(false)))
			})
		})

		Context("w/ feature gates being present in etcd config", func() {
			BeforeEach(func() {
				featureGates = map[string]bool{
					"UseEtcdWrapper": true,
				}
			})

			It("should successfully deploy all the resources", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(8))
				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__etcd-druid.yaml"])).To(Equal(deploymentWithoutImageVectorOverwriteYAMLFor(true)))
				Expect(string(managedResourceSecret.Data["poddisruptionbudget__"+namespace+"__etcd-druid.yaml"])).To(Equal(podDisruptionYAMLFor(false)))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(bootstrapper.Destroy(ctx)).To(Succeed())

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
				Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the runtime ManagedResource is still progressing", func() {
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("still progressing")))
			})

			It("should succeed because the ManagedResource is healthy and progressed", func() {
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(bootstrapper.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(bootstrapper.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(bootstrapper.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
