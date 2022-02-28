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
	"reflect"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const projectReconcilerName = "project"

func (c *Controller) projectAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
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
			c.log.Error(err, "Couldn't get key for object", "object", newObj)
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
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.projectQueue.Add(key)
}

// NewProjectReconciler creates a new instance of a reconciler which reconciles Projects.
func NewProjectReconciler(config *config.ProjectControllerConfiguration, gardenClient kubernetes.Interface, recorder record.EventRecorder) reconcile.Reconciler {
	return &projectReconciler{
		config:       config,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type projectReconciler struct {
	config       *config.ProjectControllerConfiguration
	gardenClient kubernetes.Interface
	recorder     record.EventRecorder
}

func (r *projectReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if project.DeletionTimestamp != nil {
		log.Info("Deleting project")
		return r.delete(ctx, log, project, r.gardenClient.Client())
	}

	log.Info("Reconciling project")
	return r.reconcile(ctx, log, project, r.gardenClient.Client())
}

func updateStatus(ctx context.Context, c client.Client, project *gardencorev1beta1.Project, transform func()) error {
	patch := client.StrategicMergeFrom(project.DeepCopy())
	transform()
	return c.Status().Patch(ctx, project, patch)
}
