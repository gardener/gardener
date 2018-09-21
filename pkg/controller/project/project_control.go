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
	"fmt"
	"path/filepath"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func (c *Controller) projectAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.projectQueue.Add(key)
}

func (c *Controller) projectUpdate(oldObj, newObj interface{}) {
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

	if err := c.control.ReconcileProject(project); err != nil {
		c.projectQueue.AddAfter(key, 15*time.Second)
	} else {
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
	ReconcileProject(project *gardenv1beta1.Project) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Projects. updater is the UpdaterInterface used
// to update the status of Projects. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, recorder record.EventRecorder, updater UpdaterInterface, backupInfrastructureLister gardenlisters.BackupInfrastructureLister, shootLister gardenlisters.ShootLister, namespaceLister kubecorev1listers.NamespaceLister) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, recorder, updater, backupInfrastructureLister, shootLister, namespaceLister}
}

type defaultControl struct {
	k8sGardenClient            kubernetes.Client
	k8sGardenInformers         gardeninformers.SharedInformerFactory
	recorder                   record.EventRecorder
	updater                    UpdaterInterface
	backupInfrastructureLister gardenlisters.BackupInfrastructureLister
	shootLister                gardenlisters.ShootLister
	namespaceLister            kubecorev1listers.NamespaceLister
}

func (c *defaultControl) ReconcileProject(obj *gardenv1beta1.Project) error {
	_, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	logger.Logger.Infof("[PROJECT RECONCILE] %s", obj.Name)

	var (
		project       = obj.DeepCopy()
		projectLogger = logger.NewFieldLogger(logger.Logger, "project", project.Name)

		namespace = project.Spec.Namespace

		// Initialize conditions based on the current status.
		newConditions                    = helper.NewConditions(project.Status.Conditions, gardenv1beta1.ProjectNamespaceReady, gardenv1beta1.ProjectNamespaceEmpty, gardenv1beta1.ProjectShootsWithErrors)
		conditionProjectNamespaceReady   = newConditions[0]
		conditionProjectNamespaceEmpty   = newConditions[1]
		conditionProjectShootsWithErrors = newConditions[2]
	)

	if namespace == nil {
		return fmt.Errorf("cannot reconcile project %q as its namespace is empty", project.Name)
	}

	backupInfrastructureList, err := c.backupInfrastructureLister.BackupInfrastructures(*namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	shootList, err := c.shootLister.Shoots(*namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	chartRenderer, err := chartrenderer.New(c.k8sGardenClient)
	if err != nil {
		return err
	}

	var (
		numberOfBackupInfrastructure = len(backupInfrastructureList)
		numberOfShoots               = len(shootList)
		namespaceEmpty               = numberOfBackupInfrastructure == 0 && numberOfShoots == 0
	)

	// Handle project deletion.
	if project.DeletionTimestamp != nil {
		if namespaceObj, err := c.namespaceLister.Get(*namespace); err == nil {
			// If namespace is marked for deletion we have to do nothing and wait for deletion.
			if namespaceObj.DeletionTimestamp != nil {
				conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceDeletionProcessing, fmt.Sprintf("Waiting for namespace '%s' to be deleted", namespaceObj.Name))
				_, err = c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
				return err
			}

			// If the namespace is not empty we cannot delete it (otherwise we would end in a deadlock situation as Gardener won't be able to do anything with
			// that particular namespace anymore).
			if !namespaceEmpty {
				projectLogger.Infof("Project deletion is not possible (there still exist objects which prevent deleting namespace '%s')", namespaceObj.Name)
				conditionProjectNamespaceEmpty = helper.ModifyCondition(conditionProjectNamespaceEmpty, corev1.ConditionFalse, "ShootsOrBackupInfrastructuresExist", fmt.Sprintf("Number of BackupInfrastructures = %d, Number of Shoots = %d", numberOfBackupInfrastructure, numberOfShoots))
				conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceDeletionImpossible, "Namespace is not empty")
				_, err = c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
				return err
			}

			// Namespace is empty and allowed to be deleted.
			conditionProjectNamespaceEmpty = helper.ModifyCondition(conditionProjectNamespaceEmpty, corev1.ConditionTrue, "NoShootsOrBackupInfrastructuresExist", fmt.Sprintf("Number of BackupInfrastructures = %d, Number of Shoots = %d", numberOfBackupInfrastructure, numberOfShoots))
			conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceDeletionAllowed, "Namespace is empty and allowed to be deleted")
			project, err = c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
			if err != nil {
				projectLogger.Error(err.Error())
				return err
			}

			projectLogger.Infof("Project deletion is now possible (no blocking objects exist anymore). Deleting namespace '%s'", namespaceObj.Name)
			if err := c.k8sGardenClient.DeleteNamespace(namespaceObj.Name); err != nil {
				conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceDeletionFailed, fmt.Sprintf("Error while deleting namespace %s: %+v", namespaceObj.Name, err))
				c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
				return err
			}

			conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceDeletionProcessing, fmt.Sprintf("Waiting for namespace '%s' to be deleted", namespaceObj.Name))
			_, err = c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
			return err
		}
		if err != nil && !apierrors.IsNotFound(err) {
			projectLogger.Error(err.Error())
			return err
		}

		// Remove finalizer from Project
		projectFinalizers := sets.NewString(project.Finalizers...)
		projectFinalizers.Delete(gardenv1beta1.GardenerName)
		project.Finalizers = projectFinalizers.UnsortedList()
		if _, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().Projects().Update(project); err != nil && !apierrors.IsNotFound(err) {
			projectLogger.Error(err.Error())
			return err
		}
		return nil
	}

	// Update namespace and check ProjectNamespaceReady condition.
	if err := c.updateNamespace(project); err != nil {
		message := fmt.Sprintf("Error while updating namespace for project %q: %+v", project.Name, err)
		conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceCreationFailed, message)
		projectLogger.Error(message)
		c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
		return err
	}
	conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionTrue, gardenv1beta1.ProjectNamespaceReconciled, "Namespace has been reconciled.")

	// Create RBAC rules to allow project owner to read, update, and delete the project.
	owners := []rbacv1.Subject{project.Spec.Owner}
	// This is to make sure that the actual creator does not accidentally lock himself out of the project.
	if createdBy, ok := project.Annotations[common.GardenCreatedBy]; ok && project.Spec.Owner.Name != createdBy {
		owners = append(owners, rbacv1.Subject{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.UserKind,
			Name:     createdBy,
		})
	}

	values := map[string]interface{}{
		"project": map[string]interface{}{
			"name":   project.Name,
			"uid":    project.UID,
			"owners": owners,
		},
	}
	if err := common.ApplyChart(c.k8sGardenClient, chartRenderer, filepath.Join(common.ChartPath, "garden-project", "charts", "project-rbac"), "project-rbac", *namespace, values, nil); err != nil {
		message := fmt.Sprintf("Error while creating RBAC rules for namespace %q: %+v", *namespace, err)
		conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceReconcileFailed, message)
		projectLogger.Error(message)
		c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
		return err
	}

	if err := c.ensureMemberRoleBinding(project, owners); err != nil {
		message := fmt.Sprintf("Error while creating member rolebinding for namespace %q: %+v", *namespace, err)
		conditionProjectNamespaceReady = helper.ModifyCondition(conditionProjectNamespaceReady, corev1.ConditionFalse, gardenv1beta1.ProjectNamespaceReconcileFailed, message)
		projectLogger.Error(message)
		c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
		return err
	}

	// Check ProjectNamespaceEmpty condition.
	if namespaceEmpty {
		conditionProjectNamespaceEmpty = helper.ModifyCondition(conditionProjectNamespaceEmpty, corev1.ConditionTrue, "NoShootsOrBackupInfrastructuresExist", fmt.Sprintf("Number of BackupInfrastructures = %d, Number of Shoots = %d", numberOfBackupInfrastructure, numberOfShoots))
	} else {
		conditionProjectNamespaceEmpty = helper.ModifyCondition(conditionProjectNamespaceEmpty, corev1.ConditionFalse, "ShootsOrBackupInfrastructuresExist", fmt.Sprintf("Number of BackupInfrastructures = %d, Number of Shoots = %d", numberOfBackupInfrastructure, numberOfShoots))
	}

	// Check ProjectShootsWithErrors condition.
	conditionProjectShootsWithErrors = helper.ModifyCondition(conditionProjectShootsWithErrors, corev1.ConditionFalse, "NoShootsWithErrorsFound", "No operation failed for Shoots.")
	for _, shoot := range shootList {
		if shoot.Status.LastError != nil {
			conditionProjectShootsWithErrors = helper.ModifyCondition(conditionProjectShootsWithErrors, corev1.ConditionTrue, "ShootsWithErrorsFound", "Shoot with .status.lastError found")
			break
		}
	}

	_, err = c.updateProjectStatus(project, *conditionProjectNamespaceReady, *conditionProjectNamespaceEmpty, *conditionProjectShootsWithErrors)
	return err
}

func (c *defaultControl) updateProjectStatus(project *gardenv1beta1.Project, conditions ...gardenv1beta1.Condition) (*gardenv1beta1.Project, error) {
	if len(conditions) != 0 {
		if !helper.ConditionsNeedUpdate(project.Status.Conditions, conditions) {
			return project, nil
		}
		project.Status.Conditions = conditions
	}

	project, err := c.updater.UpdateProjectStatus(project)
	if err != nil {
		logger.Logger.Errorf("Could not update the Project status: %+v", err)
	}

	return project, err
}

func (c *defaultControl) updateNamespace(project *gardenv1beta1.Project) error {
	var (
		namespaceLabels = map[string]string{
			common.GardenRole:  common.GardenRoleProject,
			common.ProjectName: project.Name,
		}
		namespaceAnnotations = map[string]string{
			common.ProjectOwner: project.Spec.Owner.Name,
		}
	)

	if project.Spec.Description != nil {
		namespaceAnnotations[common.ProjectDescription] = *project.Spec.Description
	}
	if project.Spec.Purpose != nil {
		namespaceAnnotations[common.ProjectPurpose] = *project.Spec.Purpose
	}

	namespaceObj, err := c.k8sGardenClient.GetNamespace(*project.Spec.Namespace)
	if err != nil {
		return err
	}

	namespaceObj.OwnerReferences = common.MergeOwnerReferences(namespaceObj.OwnerReferences, *metav1.NewControllerRef(project, gardenv1beta1.SchemeGroupVersion.WithKind("Project")))
	namespaceObj.Annotations = utils.MergeStringMaps(namespaceObj.Annotations, namespaceAnnotations)
	namespaceObj.Labels = utils.MergeStringMaps(namespaceObj.Labels, namespaceLabels)

	_, err = c.k8sGardenClient.UpdateNamespace(namespaceObj)
	return err
}

func (c *defaultControl) ensureMemberRoleBinding(project *gardenv1beta1.Project, owners []rbacv1.Subject) error {
	_, err := c.k8sGardenClient.CreateOrPatchRoleBinding(metav1.ObjectMeta{
		Name:      common.ProjectMemberRoleBinding,
		Namespace: *project.Spec.Namespace,
	}, func(rolebinding *rbacv1.RoleBinding) *rbacv1.RoleBinding {
		if rolebinding.Labels == nil {
			rolebinding.Labels = make(map[string]string)
		}
		rolebinding.Labels[common.GardenRole] = common.GardenRoleMembers

		rolebinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     common.ProjectMemberClusterRole,
		}

	outer:
		for _, owner := range owners {
			for _, subject := range rolebinding.Subjects {
				if apiequality.Semantic.DeepEqual(subject, owner) {
					continue outer
				}
			}
			rolebinding.Subjects = append(rolebinding.Subjects, owner)
		}

		return rolebinding
	})
	return err
}
