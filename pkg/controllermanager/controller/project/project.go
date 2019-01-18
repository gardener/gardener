// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutils "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"

	multierror "github.com/hashicorp/go-multierror"
)

// Controller controls Projects.
type Controller struct {
	k8sGardenClient    kubernetes.Interface
	k8sGardenInformers gardeninformers.SharedInformerFactory
	k8sInformers       kubeinformers.SharedInformerFactory

	control  ControlInterface
	recorder record.EventRecorder

	projectLister gardenlisters.ProjectLister
	projectQueue  workqueue.RateLimitingInterface
	projectSynced cache.InformerSynced

	namespaceLister kubecorev1listers.NamespaceLister
	namespaceQueue  workqueue.RateLimitingInterface
	namespaceSynced cache.InformerSynced

	shootLister gardenlisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(k8sGardenClient kubernetes.Interface, gardenInformerFactory gardeninformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = gardenInformerFactory.Garden().V1beta1()
		corev1Informer        = kubeInformerFactory.Core().V1()

		projectInformer = gardenv1beta1Informer.Projects()
		projectLister   = projectInformer.Lister()

		namespaceInformer = corev1Informer.Namespaces()
		namespaceLister   = namespaceInformer.Lister()

		projectUpdater = NewRealUpdater(k8sGardenClient, projectLister)
	)

	projectController := &Controller{
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: gardenInformerFactory,
		control:            NewDefaultControl(k8sGardenClient, gardenInformerFactory, recorder, projectUpdater, namespaceLister),
		recorder:           recorder,
		projectLister:      projectLister,
		projectQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project"),
		namespaceLister:    namespaceLister,
		namespaceQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Namespace"),
		workerCh:           make(chan int),
	}

	projectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    projectController.projectAdd,
		UpdateFunc: projectController.projectUpdate,
		DeleteFunc: projectController.projectDelete,
	})
	projectController.projectSynced = projectInformer.Informer().HasSynced
	projectController.namespaceSynced = namespaceInformer.Informer().HasSynced

	return projectController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.projectSynced, c.namespaceSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Project workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("Project controller initialized.")

	// Migration of legacy rolebinding to new .spec.members field in project resource
	go func() {
		utils.Retry(10*time.Second, 10*time.Minute, func() (ok, severe bool, err error) {
			if err := c.migrateRoleBindingToProjectMembers(); err != nil {
				logger.Logger.Errorf("[ROLEBINDING MIGRATION] Error migrating old rolebindings to project members: %+v", err)
				return false, false, err
			}
			return true, false, nil
		})
	}()

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.projectQueue, "Project", c.reconcileProjectKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.projectQueue.ShutDown()

	for {
		if c.projectQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Project worker and no items left in the queues. Terminated Project controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Project worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.projectQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// This function has been introduced to migrate the legacy project member rolebindings created by older
// versions of the Gardener dashboard (prior 1.23) to the new .spec.members field of the Project resource.
// It can be removed in future versions of Gardener.
func (c *Controller) migrateRoleBindingToProjectMembers() error {
	projectList, err := c.projectLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error fetching project list: %+v", err)
	}

	roleBindingList, err := c.k8sGardenClient.ListRoleBindings(metav1.NamespaceAll, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{common.GardenRole: common.GardenRoleMembers}).String(),
	})
	if err != nil {
		return fmt.Errorf("error fetching rolebinding list: %+v", err)
	}

	var (
		namespaceToProject = make(map[string]string, len(projectList))
		result             error
	)

	for _, project := range projectList {
		if namespace := project.Spec.Namespace; namespace != nil {
			namespaceToProject[*namespace] = project.Name
		}
	}

	for _, roleBinding := range roleBindingList.Items {
		if projectName, ok := namespaceToProject[roleBinding.Namespace]; ok {
			if _, err := kutils.TryUpdateProject(c.k8sGardenClient.Garden(), retry.DefaultBackoff, metav1.ObjectMeta{Name: projectName}, func(project *gardenv1beta1.Project) (*gardenv1beta1.Project, error) {
				project.Spec.Members = roleBinding.Subjects
				return project, nil
			}); err != nil {
				result = multierror.Append(result, err)
				continue
			}

			if err := c.k8sGardenClient.Kubernetes().RbacV1().RoleBindings(roleBinding.Namespace).Delete(roleBinding.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				result = multierror.Append(result, err)
			}
		}
	}

	return result
}
