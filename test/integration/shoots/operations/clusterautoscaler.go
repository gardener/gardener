// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

/**
	Overview
		- Tests Shoot cluster autoscaling

	AfterSuite
		- Cleanup Workload in Shoot

	Test:
		1. Create a deployment with Pods each requesting half the capacity of a single node
		2. Scale up the deployment and see one node added
		3. Scale down the deployment and see one node removed (after spec.kubernetes.clusterAutoscaler.scaleDownUnneededTime|scaleDownDelayAfterAdd
	Expected Output
		- Scale-up/down should work properly
 **/

package operations

import (
	"context"
	"time"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

const (
	reserveCapacityDeploymentName      = "reserve-capacity"
	reserveCapacityDeploymentNamespace = metav1.NamespaceDefault

	scaleDownDelayAfterAdd = 1 * time.Minute
	scaleDownUnneededTime  = 1 * time.Minute
	testTimeout            = 60 * time.Minute
	scaleUpTimeout         = 20 * time.Minute
	scaleDownTimeout       = 20 * time.Minute
	cleanupTimeout         = 20 * time.Minute
)

var _ = ginkgo.Describe("Shoot clusterautoscaler testing", func() {

	var (
		f = framework.NewShootFramework(nil)

		testWorkerPoolName          = "ca-test"
		origClusterAutoscalerConfig *corev1beta1.ClusterAutoscaler
		origWorkers                 []corev1beta1.Worker
		origMinWorkers              int32
		origMaxWorkers              int32
	)

	f.Beta().Serial().CIt("should autoscale a single worker group", func(ctx context.Context) {
		var (
			shoot = f.Shoot

			workerPoolName = shoot.Spec.Provider.Workers[0].Name
		)

		origClusterAutoscalerConfig = shoot.Spec.Kubernetes.ClusterAutoscaler.DeepCopy()
		origMinWorkers = shoot.Spec.Provider.Workers[0].Minimum
		origMaxWorkers = shoot.Spec.Provider.Workers[0].Maximum

		ginkgo.By("updating shoot spec for test")
		// set clusterautoscaler params to lower values so we don't have to wait too long
		// and ensure the worker pool has maximum > minimum
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			if s.Spec.Kubernetes.ClusterAutoscaler == nil {
				s.Spec.Kubernetes.ClusterAutoscaler = &corev1beta1.ClusterAutoscaler{}
			}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd = &metav1.Duration{Duration: scaleDownDelayAfterAdd}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime = &metav1.Duration{Duration: scaleDownUnneededTime}

			if origMaxWorkers != origMinWorkers+1 {
				s.Spec.Provider.Workers[0].Maximum = origMinWorkers + 1
			}

			return nil
		})
		framework.ExpectNoError(err)

		nodeList, err := framework.GetAllNodesInWorkerPool(ctx, f.ShootClient, &workerPoolName)
		framework.ExpectNoError(err)

		origNodeCount := len(nodeList.Items)
		gomega.Expect(origNodeCount).To(gomega.BeEquivalentTo(origMinWorkers), "shoot should have minimum node count before the test")

		ginkgo.By("creating reserve-capacity deployment")
		// take half the node allocatable capacity as requests for the deployment
		nodeAllocatable := nodeList.Items[0].Status.Allocatable
		requestCPUMilli := nodeAllocatable.Cpu().ScaledValue(resource.Milli) / 2
		requestMemoryMegs := nodeAllocatable.Memory().ScaledValue(resource.Mega) / 2

		values := reserveCapacityValues{
			Name:      reserveCapacityDeploymentName,
			Namespace: reserveCapacityDeploymentNamespace,
			Replicas:  origMinWorkers,
			Requests: struct {
				CPU    string
				Memory string
			}{
				CPU:    resource.NewScaledQuantity(requestCPUMilli, resource.Milli).String(),
				Memory: resource.NewScaledQuantity(requestMemoryMegs, resource.Mega).String(),
			},
			WorkerPool: workerPoolName,
		}
		err = f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.ReserveCapacityName, values)
		framework.ExpectNoError(err)

		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling up reserve-capacity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.Client(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, origMinWorkers+1)
		framework.ExpectNoError(err)

		ginkgo.By("one node should be added to the worker pool")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, int(origMinWorkers+1), &workerPoolName, scaleUpTimeout)
		framework.ExpectNoError(err)

		ginkgo.By("reserve-capacity deployment should get healthy again")
		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling down reserve-capacity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.Client(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, origMinWorkers)
		framework.ExpectNoError(err)

		ginkgo.By("one node should be removed from the worker pool")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, int(origMinWorkers), &workerPoolName, scaleDownTimeout)
		framework.ExpectNoError(err)
	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {

		ginkgo.By("reverting shoot spec changes by test")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			s.Spec.Kubernetes.ClusterAutoscaler = origClusterAutoscalerConfig
			s.Spec.Provider.Workers[0].Maximum = origMaxWorkers

			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("deleting reserve-capacity deployment")
		err = framework.DeleteResource(ctx, f.ShootClient, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: reserveCapacityDeploymentNamespace,
				Name:      reserveCapacityDeploymentName,
			},
		})
		framework.ExpectNoError(err)
	}, cleanupTimeout))

	f.Beta().Serial().CIt("should autoscale a single worker group to/from zero", func(ctx context.Context) {
		var (
			shoot = f.Shoot

			workerPoolName = shoot.Spec.Provider.Workers[0].Name
		)

		origClusterAutoscalerConfig = shoot.Spec.Kubernetes.ClusterAutoscaler.DeepCopy()
		origWorkers = shoot.Spec.Provider.Workers

		if shoot.Spec.Provider.Type != "aws" && shoot.Spec.Provider.Type != "azure" {
			ginkgo.Skip("not applicable")
		}

		// Create a dedicated worker-pool for cluster autoscaler.
		testWorkerPool := origWorkers[0]
		testWorkerPool.Name = testWorkerPoolName
		testWorkerPool.Minimum = 0
		testWorkerPool.Maximum = 2
		testWorkerPool.Taints = []corev1.Taint{
			{
				Key:    testWorkerPoolName,
				Effect: corev1.TaintEffectNoSchedule,
				Value:  testWorkerPoolName,
			},
		}

		ginkgo.By("updating shoot spec for test")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			s.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, testWorkerPool)

			if s.Spec.Kubernetes.ClusterAutoscaler == nil {
				s.Spec.Kubernetes.ClusterAutoscaler = &corev1beta1.ClusterAutoscaler{}
			}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd = &metav1.Duration{Duration: scaleDownDelayAfterAdd}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime = &metav1.Duration{Duration: scaleDownUnneededTime}

			return nil
		})
		framework.ExpectNoError(err)

		nodeList, err := framework.GetAllNodesInWorkerPool(ctx, f.ShootClient, &testWorkerPoolName)
		framework.ExpectNoError(err)

		nodeCount := len(nodeList.Items)
		gomega.Expect(nodeCount).To(gomega.BeEquivalentTo(testWorkerPool.Minimum), "shoot should have minimum node count before the test")

		ginkgo.By("creating reserve-capacity deployment")

		// As  testWorkerpool doesn't have the node objects, fetching the nodesList from original worker-pool,
		// to calculate the requestCPUMilli/requestMemoryMegs of the workload.
		origNodeList, err := framework.GetAllNodesInWorkerPool(ctx, f.ShootClient, &workerPoolName)
		framework.ExpectNoError(err)

		// take half the node allocatable capacity as requests for the deployment
		nodeAllocatable := origNodeList.Items[0].Status.Allocatable
		requestCPUMilli := nodeAllocatable.Cpu().ScaledValue(resource.Milli) / 2
		requestMemoryMegs := nodeAllocatable.Memory().ScaledValue(resource.Mega) / 2

		values := reserveCapacityValues{
			Name:      reserveCapacityDeploymentName,
			Namespace: reserveCapacityDeploymentNamespace,
			Replicas:  0, // This is to test the scale-from-zero.
			Requests: struct {
				CPU    string
				Memory string
			}{
				CPU:    resource.NewScaledQuantity(requestCPUMilli, resource.Milli).String(),
				Memory: resource.NewScaledQuantity(requestMemoryMegs, resource.Mega).String(),
			},
			WorkerPool:    testWorkerPoolName,
			TolerationKey: testWorkerPoolName,
		}
		err = f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.ReserveCapacityName, values)
		framework.ExpectNoError(err)

		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling up reserve-capacity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.Client(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, 1)
		framework.ExpectNoError(err)

		ginkgo.By("one node should be added to the worker pool")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, 1, &testWorkerPoolName, scaleUpTimeout)
		framework.ExpectNoError(err)

		ginkgo.By("reserve-capacity deployment should get healthy again")
		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling down reserve-capacity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.Client(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, 0)
		framework.ExpectNoError(err)

		ginkgo.By("worker pool should be scaled-down to 0")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, 0, &testWorkerPoolName, scaleDownTimeout)
		framework.ExpectNoError(err)
	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("reverting shoot spec changes by test")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			s.Spec.Kubernetes.ClusterAutoscaler = origClusterAutoscalerConfig

			for i, worker := range s.Spec.Provider.Workers {
				if worker.Name == testWorkerPoolName {
					// Remove the dedicated ca-test workerpool
					s.Spec.Provider.Workers[i] = s.Spec.Provider.Workers[len(s.Spec.Provider.Workers)-1]
					s.Spec.Provider.Workers = s.Spec.Provider.Workers[:len(s.Spec.Provider.Workers)-1]
					break
				}
			}

			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("deleting reserve-capacity deployment")
		err = framework.DeleteResource(ctx, f.ShootClient, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: reserveCapacityDeploymentNamespace,
				Name:      reserveCapacityDeploymentName,
			},
		})
		framework.ExpectNoError(err)
	}, cleanupTimeout))

})

type reserveCapacityValues struct {
	Name      string
	Namespace string
	Replicas  int32
	Requests  struct {
		CPU    string
		Memory string
	}
	WorkerPool    string
	TolerationKey string
}
