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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/logger"
	kutils "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) projectAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.projectQueue.Add(key)
	c.projectStaleQueue.Add(key)
}

func (c *Controller) projectUpdate(oldObj, newObj interface{}) {
	newProject, ok := newObj.(*gardencorev1beta1.Project)
	if !ok {
		return
	}

	if newProject.Generation == newProject.Status.ObservedGeneration {
		return
	}

	c.projectAdd(newObj)
}

func (c *Controller) projectDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.projectQueue.Add(key)
}

func (c *Controller) reconcileProjectKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	project, err := c.projectLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[PROJECT RECONCILE] %s - skipping because Project has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[PROJECT RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	needsRequeue, err := c.control.ReconcileProject(project)
	if err != nil {
		return err
	}
	if needsRequeue {
		c.projectQueue.AddAfter(key, time.Minute)
	}

	return nil
}

// ControlInterface implements the control logic for updating Projects. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileProject implements the control logic for Project creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileProject(project *gardencorev1beta1.Project) (bool, error)
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Projects. updater is the UpdaterInterface used
// to update the status of Projects. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, recorder record.EventRecorder, namespaceLister kubecorev1listers.NamespaceLister) ControlInterface {
	return &defaultControl{clientMap, k8sGardenCoreInformers, recorder, namespaceLister}
}

type defaultControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	recorder               record.EventRecorder
	namespaceLister        kubecorev1listers.NamespaceLister
}

func newProjectLogger(project *gardencorev1beta1.Project) logrus.FieldLogger {
	if project == nil {
		return logger.Logger
	}
	return logger.NewFieldLogger(logger.Logger, "project", project.Name)
}

func (c *defaultControl) ReconcileProject(obj *gardencorev1beta1.Project) (bool, error) {
	var (
		ctx           = context.TODO()
		project       = obj.DeepCopy()
		projectLogger = newProjectLogger(project)
	)

	projectLogger.Infof("[PROJECT RECONCILE]")

	if project.DeletionTimestamp != nil {
		return c.delete(ctx, project)
	}

	return false, c.reconcile(ctx, project)
}

func updateProjectStatus(ctx context.Context, g gardencore.Interface, objectMeta metav1.ObjectMeta, transform func(project *gardencorev1beta1.Project) (*gardencorev1beta1.Project, error)) (*gardencorev1beta1.Project, error) {
	project, err := kutils.TryUpdateProjectStatus(ctx, g, retry.DefaultRetry, objectMeta, transform)
	if err != nil {
		newProjectLogger(project).Errorf("Error updating the status of the project: %q", err.Error())
	}
	return project, err
}

func (c *defaultControl) reportEvent(project *gardencorev1beta1.Project, isError bool, eventReason, messageFmt string, args ...interface{}) {
	var (
		eventType     string
		projectLogger = newProjectLogger(project)
	)

	if !isError {
		eventType = corev1.EventTypeNormal
		projectLogger.Infof(messageFmt, args...)
	} else {
		eventType = corev1.EventTypeWarning
		projectLogger.Errorf(messageFmt, args...)
	}

	c.recorder.Eventf(project, eventType, eventReason, messageFmt, args...)
}
