// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

// SkipVerification is a common function to skip object verification during admission
func SkipVerification(operation admission.Operation, metadata metav1.ObjectMeta) bool {
	return operation == admission.Update && metadata.DeletionTimestamp != nil
}

// GetProject retrieves the project with the corresponding namespace
func GetProject(namespace string, projectLister corelisters.ProjectLister) (*core.Project, error) {
	projects, err := projectLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespace {
			return project, nil
		}
	}

	return nil, fmt.Errorf("no project found for namespace %q", namespace)
}

// IsSeedUsedByShoot checks whether there is a shoot cluster refering the provided seed name
func IsSeedUsedByShoot(seedName string, shoots []*core.Shoot) bool {
	for _, shoot := range shoots {
		if shoot.Spec.SeedName != nil && *shoot.Spec.SeedName == seedName {
			return true
		}
		if shoot.Status.SeedName != nil && *shoot.Status.SeedName == seedName {
			return true
		}
	}
	return false
}

// IsSeedUsedByBackupBucket checks whether there is a backupbucket refering the provided seed name
func IsSeedUsedByBackupBucket(seedName string, backupbuckets []*core.BackupBucket) bool {
	for _, backupbucket := range backupbuckets {
		if backupbucket.Spec.SeedName != nil && *backupbucket.Spec.SeedName == seedName {
			return true
		}
	}
	return false
}
