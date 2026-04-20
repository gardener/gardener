// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("WaiterTest", func() {
	var (
		ctx                   = context.TODO()
		testLogger            = logr.Discard()
		errorMsg              = "fake error"
		fakeErr               = fmt.Errorf("fake error")
		kubeControllerManager Interface
		namespace             = "shoot--foo--bar"
		version               = semver.MustParse("v1.31.1")
		isWorkerless          = false
		values                Values

		fakeSeedInterface kubernetes.Interface
		seedClient        client.Client
		shootClient       client.Client
		waiter            *retryfake.Ops
	)

	BeforeEach(func() {
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeSeedInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(seedClient).WithClient(seedClient).Build()
	})

	Describe("#WaitForControllerToBeActive", func() {
		BeforeEach(func() {
			values = Values{
				RuntimeVersion: semver.MustParse("1.31.1"),
				TargetVersion:  version,
				IsWorkerless:   isWorkerless,
			}
			kubeControllerManager = New(testLogger, fakeSeedInterface, namespace, nil, values)
			kubeControllerManager.SetShootClient(shootClient)

			waiter = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, waiter.Until,
				&retry.UntilTimeout, waiter.UntilTimeout,
			))
		})

		It("should fail if the seed client cannot talk to the Seed API Server", func() {
			errSeedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fakeErr
				},
			}).Build()
			fakeSeedInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(errSeedClient).WithClient(errSeedClient).Build()
			kubeControllerManager = New(testLogger, fakeSeedInterface, namespace, nil, values)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(fakeErr))
		})

		It("should fail if the kube controller manager deployment does not exist", func() {
			// No deployment created, so Get will return NotFound
			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(ContainSubstring("kube controller manager deployment not found")))
		})

		It("should fail if it fails to list pods in the shoot namespace in the Seed", func() {
			// Rebuild client with list error
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
					return fakeErr
				},
			}).Build()

			Expect(seedClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}})).To(Succeed())

			fakeSeedInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(seedClient).WithClient(seedClient).Build()
			kubeControllerManager = New(testLogger, fakeSeedInterface, namespace, nil, values)
			kubeControllerManager.SetShootClient(shootClient)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(fmt.Sprintf("could not check whether controller kube-controller-manager is active: %s", errorMsg)))
		})

		It("should fail if no kube controller manager pod can be found", func() {
			// Create the deployment so Get succeeds
			Expect(seedClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}})).To(Succeed())
			// No pods created - List returns empty list

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if one of the existing kube controller manager pods has a deletion timestamp", func() {
			// Create the deployment
			Expect(seedClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}})).To(Succeed())

			// Create pods with matching labels
			Expect(seedClient.Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "pod1", Namespace: namespace,
				Labels: map[string]string{"app": "kubernetes", "role": "controller-manager"},
			}})).To(Succeed())
			// For the pod with deletion timestamp, we need a finalizer to prevent immediate deletion
			pod2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "pod2", Namespace: namespace,
				Labels:     map[string]string{"app": "kubernetes", "role": "controller-manager"},
				Finalizers: []string{"test-finalizer"},
			}}
			Expect(seedClient.Create(ctx, pod2)).To(Succeed())
			// Now delete pod2 to set DeletionTimestamp
			Expect(seedClient.Delete(ctx, pod2)).To(Succeed())

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if the existing kube controller manager fails to acquire leader election", func() {
			// Create the deployment
			Expect(seedClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}})).To(Succeed())

			// Create a matching pod
			Expect(seedClient.Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "pod1", Namespace: namespace,
				Labels: map[string]string{"app": "kubernetes", "role": "controller-manager"},
			}})).To(Succeed())

			// Create a lease with old renew time in the shoot client
			renewTime := metav1.NewMicroTime(time.Now().UTC().Add(-10 * time.Second))
			Expect(shootClient.Create(ctx, &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-controller-manager",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime: &renewTime,
				},
			})).To(Succeed())

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should succeed", func() {
			// Create the deployment
			Expect(seedClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}})).To(Succeed())

			// Create a matching pod
			Expect(seedClient.Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "pod1", Namespace: namespace,
				Labels: map[string]string{"app": "kubernetes", "role": "controller-manager"},
			}})).To(Succeed())

			// Create a lease with recent renew time in the shoot client
			renewTime := metav1.NewMicroTime(time.Now().UTC())
			Expect(shootClient.Create(ctx, &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-controller-manager",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: coordinationv1.LeaseSpec{
					RenewTime: &renewTime,
				},
			})).To(Succeed())

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		var (
			fakeClient              client.Client
			fakeKubernetesInterface kubernetes.Interface
			managedResource         *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-kube-controller-manager",
					Namespace: namespace,
				},
			}

			kubeControllerManager = New(testLogger, fakeKubernetesInterface, namespace, nil, Values{})

			fakeOps := &retryfake.Ops{MaxAttempts: 2}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		It("should fail when the wait for the runtime managed resource deletion times out", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

			Expect(kubeControllerManager.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})

		It("should not return an error when they are already removed", func() {
			Expect(kubeControllerManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
