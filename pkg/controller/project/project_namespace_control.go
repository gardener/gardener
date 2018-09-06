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
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) namespaceAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.namespaceQueue.Add(key)
}

func (c *Controller) namespaceUpdate(oldObj, newObj interface{}) {
	c.namespaceAdd(newObj)
}

func (c *Controller) namespaceDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.namespaceQueue.Add(key)
}

func (c *Controller) reconcileProjectNamespaceKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	namespace, err := c.namespaceLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[PROJECT NAMESPACE RECONCILE] %s - skipping because Namespace has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[PROJECT NAMESPACE RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.projectNamespaceControl.ReconcileProjectNamespace(namespace, key); err != nil {
		c.namespaceQueue.AddAfter(key, 15*time.Second)
	}

	return nil
}

// ControlInterfaceProjectNamespace implements the control logic for updating Project namespaces. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterfaceProjectNamespace interface {
	// ReconcileProject implements the control logic for Project namespace creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileProjectNamespace(namespace *corev1.Namespace, key string) error
}

// NewDefaultProjectNamespaceControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Projects. updater is the UpdaterInterface used
// to update the status of Projects. You should use an instance returned from NewDefaultProjectNamespaceControl() for any
// scenario other than testing.
func NewDefaultProjectNamespaceControl(k8sGardenClient kubernetes.Client, kubeInformerFactory kubeinformers.SharedInformerFactory, namespaceLister kubecorev1listers.NamespaceLister, projectLister gardenlisters.ProjectLister) ControlInterfaceProjectNamespace {
	return &defaultProjectNamespaceControl{k8sGardenClient, kubeInformerFactory, namespaceLister, projectLister}
}

type defaultProjectNamespaceControl struct {
	k8sGardenClient kubernetes.Client
	kubeInformers   kubeinformers.SharedInformerFactory
	namespaceLister kubecorev1listers.NamespaceLister
	projectLister   gardenlisters.ProjectLister
}

func (c *defaultProjectNamespaceControl) ReconcileProjectNamespace(obj *corev1.Namespace, key string) error {
	// We only reconcile namespaces which were marked to be used as a project.
	if role, ok := obj.Labels[common.GardenRole]; !ok || role != common.GardenRoleProject {
		return nil
	}

	var (
		namespace              = obj.DeepCopy()
		projectNamespaceLogger = logger.NewFieldLogger(logger.Logger, "project_namespace", namespace.Name)

		project *gardenv1beta1.Project
		err     error

		mustCreateProject = false
	)

	// Determine project name for the given namespace object.
	project, err = common.ProjectForNamespace(c.k8sGardenClient, namespace)
	if err != nil {
		return err
	}
	if project == nil {
		mustCreateProject = true
		project = &gardenv1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ProjectNameForNamespace(namespace),
				Annotations: map[string]string{
					common.ProjectNamespace: namespace.Name,
				},
			},
			Spec: gardenv1beta1.ProjectSpec{
				Owner: rbacv1.Subject{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     "unknown",
				},
			},
		}
	}

	// The namespace is marked for deletion, we should check whether the corresponding project object is also marked for
	// deletion already. If not, we do mark it.
	if namespace.DeletionTimestamp != nil {
		if project.DeletionTimestamp == nil {
			if err := c.k8sGardenClient.GardenClientset().GardenV1beta1().Projects().Delete(project.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				projectNamespaceLogger.Errorf("[PROJECT NAMESPACE RECONCILE] Failed to delete project object for namespace %s: '%v'", namespace.Name, err)
				return err
			}
		}
		return nil
	}

	// Create or update project object for namespace.
	if owner, ok := namespace.Annotations[common.ProjectOwner]; ok {
		project.Spec.Owner.Name = owner
	}
	if purpose, ok := namespace.Annotations[common.ProjectPurpose]; ok {
		project.Spec.Purpose = &purpose
	} else {
		project.Spec.Purpose = nil
	}
	if description, ok := namespace.Annotations[common.ProjectDescription]; ok {
		project.Spec.Description = &description
	} else {
		project.Spec.Description = nil
	}

	if mustCreateProject {
		project, err = c.k8sGardenClient.GardenClientset().GardenV1beta1().Projects().Create(project)
	} else {
		project, err = c.k8sGardenClient.GardenClientset().GardenV1beta1().Projects().Update(project)
	}
	if err == nil {
		projectNamespaceLogger.Infof("[PROJECT NAMESPACE RECONCILE] Reconciled project %s for namespace %s", project.Name, namespace.Name)
	} else if !apierrors.IsAlreadyExists(err) {
		projectNamespaceLogger.Errorf("[PROJECT NAMESPACE RECONCILE] Failed to reconcile project object for namespace %s: '%v'", namespace.Name, err)
		return err
	}

	return nil
}
