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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newProjectEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		requests := []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: "",
				Name:      obj.GetName(),
			},
		}, {
			NamespacedName: types.NamespacedName{
				Namespace: "stale",
				Name:      obj.GetName(),
			},
		}}

		return requests
	})
}

// NewProjectReconciler creates a new instance of a reconciler which reconciles Projects.
func NewProjectReconciler(logger logr.Logger, config *config.ProjectControllerConfiguration, gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &projectReconciler{
		logger:       logger,
		config:       config,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type projectReconciler struct {
	logger       logr.Logger
	config       *config.ProjectControllerConfiguration
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *projectReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithValues("project", request)

	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	logger.Info("Reconciling")

	if project.DeletionTimestamp != nil {
		return r.delete(ctx, project, r.gardenClient, logger)
	}

	return r.reconcile(ctx, project, r.gardenClient, logger)
}

func (r *projectReconciler) reportEvent(project *gardencorev1beta1.Project, logger logr.Logger, isError bool, eventReason, messageFmt string, args ...interface{}) {
	var eventType string

	if !isError {
		eventType = corev1.EventTypeNormal
		logger.Info(fmt.Sprintf(messageFmt, args...))
	} else {
		eventType = corev1.EventTypeWarning
		logger.Error(nil, fmt.Sprintf(messageFmt, args...))
	}

	r.recorder.Eventf(project, eventType, eventReason, messageFmt, args...)
}

func updateStatus(ctx context.Context, c client.Client, project *gardencorev1beta1.Project, transform func()) error {
	patch := client.StrategicMergeFrom(project.DeepCopy())
	transform()
	return c.Status().Patch(ctx, project, patch)
}
