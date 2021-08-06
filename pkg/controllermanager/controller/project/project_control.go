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
	"reflect"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	oldProject, ok := oldObj.(*gardencorev1beta1.Project)
	if !ok {
		return
	}

	if reflect.DeepEqual(newProject.Status.LastActivityTimestamp, oldProject.Status.LastActivityTimestamp) {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			logger.Logger.Errorf("Couldn't get key for object %+v: %v", newObj, err)
			return
		}
		c.projectStaleQueue.Add(key)
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

// NewProjectReconciler creates a new instance of a reconciler which reconciles Projects.
func NewProjectReconciler(l logrus.FieldLogger, config *config.ProjectControllerConfiguration, gardenClient kubernetes.Interface, recorder record.EventRecorder) reconcile.Reconciler {
	return &projectReconciler{
		logger:       l,
		config:       config,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type projectReconciler struct {
	logger       logrus.FieldLogger
	config       *config.ProjectControllerConfiguration
	gardenClient kubernetes.Interface
	recorder     record.EventRecorder
}

func (r *projectReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	projectLogger := newProjectLogger(project)
	projectLogger.Infof("[PROJECT RECONCILE] %s", project.Name)

	if project.DeletionTimestamp != nil {
		return r.delete(ctx, project, r.gardenClient.Client(), r.gardenClient.Client())
	}

	return r.reconcile(ctx, project, r.gardenClient.Client(), r.gardenClient.Client())
}

func newProjectLogger(project *gardencorev1beta1.Project) logrus.FieldLogger {
	if project == nil {
		return logger.Logger
	}
	return logger.NewFieldLogger(logger.Logger, "project", project.Name)
}

func (r *projectReconciler) reportEvent(project *gardencorev1beta1.Project, isError bool, eventReason, messageFmt string, args ...interface{}) {
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

	r.recorder.Eventf(project, eventType, eventReason, messageFmt, args...)
}

func updateStatus(ctx context.Context, c client.Client, project *gardencorev1beta1.Project, transform func()) error {
	patch := client.StrategicMergeFrom(project.DeepCopy())
	transform()
	return c.Status().Patch(ctx, project, patch)
}
