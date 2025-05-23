// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package system_test

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/seed/system"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("SeedSystem", func() {
	var (
		ctx = context.Background()

		managedResourceName        = "system"
		namespace                  = "some-namespace"
		reserveExcessCapacityImage = "some-image:some-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		deployment0YAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    resources.gardener.cloud/skip-health-check: "true"
  creationTimestamp: null
  labels:
    app: kubernetes
    role: reserve-excess-capacity
  name: reserve-excess-capacity-0
  namespace: ` + namespace + `
spec:
  replicas: 2
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: kubernetes
      role: reserve-excess-capacity
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: kubernetes
        role: reserve-excess-capacity
    spec:
      containers:
      - image: ` + reserveExcessCapacityImage + `
        imagePullPolicy: IfNotPresent
        name: pause-container
        resources:
          limits:
            cpu: "2"
            memory: 6Gi
          requests:
            cpu: "2"
            memory: 6Gi
        securityContext:
          allowPrivilegeEscalation: false
      priorityClassName: gardener-reserve-excess-capacity
      terminationGracePeriodSeconds: 5
status: {}
`
		deployment1YAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    resources.gardener.cloud/skip-health-check: "true"
  creationTimestamp: null
  labels:
    app: kubernetes
    role: reserve-excess-capacity
  name: reserve-excess-capacity-1
  namespace: ` + namespace + `
spec:
  replicas: 2
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: kubernetes
      role: reserve-excess-capacity
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: kubernetes
        role: reserve-excess-capacity
    spec:
      containers:
      - image: ` + reserveExcessCapacityImage + `
        imagePullPolicy: IfNotPresent
        name: pause-container
        resources:
          limits:
            cpu: "4"
            memory: 8Gi
          requests:
            cpu: "4"
            memory: 8Gi
        securityContext:
          allowPrivilegeEscalation: false
      nodeSelector:
        foo: bar
      priorityClassName: gardener-reserve-excess-capacity
      terminationGracePeriodSeconds: 5
      tolerations:
      - effect: NoExecute
        key: bar
        operator: Equal
        value: foo
status: {}
`
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			ReserveExcessCapacity: ReserveExcessCapacityValues{
				Enabled:  true,
				Image:    reserveExcessCapacityImage,
				Replicas: 2,
				Configs: []gardencorev1beta1.SeedSettingExcessCapacityReservationConfig{
					{
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("6Gi"),
						},
					},
				},
			},
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
		var manifests []string

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(component.Deploy(ctx)).To(Succeed())

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
		})

		It("should successfully deploy the resources", func() {
			expectedManifets := append(expectedPriorityClasses(), deployment0YAML)
			Expect(manifests).To(ConsistOf(expectedManifets))
		})

		Context("in case of additional reserve-excess-capacity configs", func() {
			BeforeEach(func() {
				values.ReserveExcessCapacity.Configs = append(values.ReserveExcessCapacity.Configs, gardencorev1beta1.SeedSettingExcessCapacityReservationConfig{
					Resources: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
					NodeSelector: map[string]string{"foo": "bar"},
					Tolerations: []corev1.Toleration{
						{Key: "bar", Value: "foo", Operator: "Equal", Effect: corev1.TaintEffectNoExecute},
					},
				})
				component = New(c, namespace, values)
			})

			It("should successfully deploy the resources", func() {
				expectedManifets := append(expectedPriorityClasses(), deployment0YAML, deployment1YAML)
				Expect(manifests).To(ConsistOf(expectedManifets))
			})
		})

		Context("in case reserve-excess-capacity is disabled", func() {
			BeforeEach(func() {
				values.ReserveExcessCapacity.Enabled = false
				component = New(c, namespace, values)
			})

			It("should successfully deploy the resources", func() {
				Expect(manifests).To(ConsistOf(expectedPriorityClasses()))
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

func expectedPriorityClasses() []string {
	priorityClasses := make([]string, 0, 10)

	expected := []struct {
		name        string
		value       int32
		description string
	}{
		{"gardener-system-900", 999998900, "PriorityClass for Seed system components"},
		{"gardener-system-800", 999998800, "PriorityClass for Seed system components"},
		{"gardener-system-700", 999998700, "PriorityClass for Seed system components"},
		{"gardener-system-600", 999998600, "PriorityClass for Seed system components"},
		{"gardener-reserve-excess-capacity", -5, "PriorityClass for reserving excess capacity on a Seed cluster"},
		{"gardener-system-500", 999998500, "PriorityClass for Shoot control plane components"},
		{"gardener-system-400", 999998400, "PriorityClass for Shoot control plane components"},
		{"gardener-system-300", 999998300, "PriorityClass for Shoot control plane components"},
		{"gardener-system-200", 999998200, "PriorityClass for Shoot control plane components"},
		{"gardener-system-100", 999998100, "PriorityClass for Shoot control plane components"},
	}

	for _, pc := range expected {
		priorityClasses = append(priorityClasses, `apiVersion: scheduling.k8s.io/v1
description: `+pc.description+`
kind: PriorityClass
metadata:
  creationTimestamp: null
  name: `+pc.name+`
value: `+strconv.FormatInt(int64(pc.value), 10)+`
`)
	}

	return priorityClasses
}
