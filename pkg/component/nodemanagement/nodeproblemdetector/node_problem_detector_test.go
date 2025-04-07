// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeproblemdetector_test

import (
	"context"
	"strconv"
	"time"

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
	. "github.com/gardener/gardener/pkg/component/nodemanagement/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeProblemDetector", func() {
	var (
		ctx = context.Background()

		managedResourceName = "shoot-core-node-problem-detector"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c                       client.Client
		component               component.DeployWaiter
		daemonSetPrometheusPort = 20257
		manifests               []string
		expectedManifests       []string
		managedResource         *resourcesv1alpha1.ManagedResource
		managedResourceSecret   *corev1.Secret
		values                  Values
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:             image,
			VPAEnabled:        false,
			KubernetesVersion: semver.MustParse("1.31.1"),
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
			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
  namespace: kube-system
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - nodes/status
  verbs:
  - patch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
  - update
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: node-problem-detector
subjects:
- kind: ServiceAccount
  name: node-problem-detector
  namespace: kube-system
`

			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: node-problem-detector
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
    gardener.cloud/role: system-component
  name: node-problem-detector
  namespace: kube-system
spec:
  ports:
  - port: 20257
    protocol: TCP
    targetPort: 20257
  selector:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
status:
  loadBalancer: {}
`
			hostPathFileOrCreate = corev1.HostPathFileOrCreate

			daemonsetYAMLFor = func(apiserverHost string) string {
				out := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
    gardener.cloud/role: system-component
    origin: gardener
  name: node-problem-detector
  namespace: kube-system
spec:
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: node-problem-detector
      app.kubernetes.io/instance: shoot-core
      app.kubernetes.io/name: node-problem-detector
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: node-problem-detector
        app.kubernetes.io/instance: shoot-core
        app.kubernetes.io/name: node-problem-detector
        gardener.cloud/role: system-component
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        origin: gardener
    spec:
      containers:
      - command:
        - /bin/sh
        - -c
        - exec /node-problem-detector --logtostderr --config.system-log-monitor=/config/kernel-monitor.json,/config/docker-monitor.json,/config/systemd-monitor.json
          .. --config.custom-plugin-monitor=/config/kernel-monitor-counter.json,/config/systemd-monitor-counter.json
          .. --config.system-stats-monitor=/config/system-stats-monitor.json --prometheus-port=` + strconv.Itoa(daemonSetPrometheusPort) + `
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName`
				if apiserverHost != "" {
					out += `
        - name: KUBERNETES_SERVICE_HOST
          value: ` + apiserverHost + ``
				}
				out += `
        image: ` + image + `
        imagePullPolicy: IfNotPresent
        name: node-problem-detector
        ports:
        - containerPort: 20257
          name: exporter
        resources:
          limits:
            memory: 500Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /var/log/journal
          name: log
          readOnly: true
        - mountPath: /etc/localtime
          name: localtime
          readOnly: true
        - mountPath: /dev/kmsg
          name: kmsg
          readOnly: true
      dnsPolicy: Default
      priorityClassName: gardener-shoot-system-900
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: node-problem-detector
      terminationGracePeriodSeconds: 30
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - hostPath:
          path: /var/log/journal
        name: log
      - hostPath:
          path: /etc/localtime
          type: ` + string(hostPathFileOrCreate) + `
        name: localtime
      - hostPath:
          path: /dev/kmsg
        name: kmsg
  updateStrategy: {}
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`
				return out
			}
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: node-problem-detector
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        memory: 20Mi
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-problem-detector
  updatePolicy:
    updateMode: Auto
status: {}
`
		)

		JustBeforeEach(func() {
			component = New(c, namespace, values)
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(component.Deploy(ctx)).To(Succeed())
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

			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			expectedManifests = []string{
				serviceAccountYAML,
				clusterRoleYAML,
				clusterRoleBindingYAML,
				serviceYAML,
			}
		})

		Context("w/o apiserver host, w/o vpaEnabled", func() {
			It("should successfully deploy all resources", func() {
				expectedManifests = append(expectedManifests, daemonsetYAMLFor(""))
				Expect(manifests).To(ConsistOf(expectedManifests))
			})
		})

		Context("w/ apiserver host, w/ vpaEnabled", func() {
			var (
				apiserverHost = "apiserver.host"
			)

			BeforeEach(func() {
				values.APIServerHost = &apiserverHost
				values.VPAEnabled = true
			})

			It("should successfully deploy all resources", func() {
				expectedManifests = append(expectedManifests, vpaYAML, daemonsetYAMLFor(apiserverHost))
				Expect(manifests).To(ConsistOf(expectedManifests))
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

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
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
