// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	backupbucket.Actuator
	client      client.Client
	bbDirectory string
}

func newActuator(mgr manager.Manager, bbDirectory string) backupbucket.Actuator {
	return &actuator{
		client:      mgr.GetClient(),
		bbDirectory: bbDirectory,
	}
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) error {
	var (
		filePath             = filepath.Join(a.bbDirectory, backupBucket.Name)
		fileMode os.FileMode = 0775
	)

	log.Info("Reconciling directory", "path", filePath)
	if err := os.Mkdir(filePath, fileMode); err != nil && !os.IsExist(err) {
		return err
	}

	// ensure the backup-bucket directory has the correct set of permissions, even if they have been changed externally
	if err := os.Chmod(filePath, fileMode); err != nil {
		return err
	}

	if backupBucket.Status.GeneratedSecretRef == nil {
		if err := a.createBackupBucketGeneratedSecret(ctx, backupBucket); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	if ref := bb.Status.GeneratedSecretRef; ref != nil {
		if err := kubernetesutils.DeleteObject(ctx, a.client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace}}); err != nil {
			return err
		}
	}

	path := filepath.Join(a.bbDirectory, bb.Name)
	log.Info("Deleting directory", "path", path)
	return os.RemoveAll(path)
}

func (a *actuator) createBackupBucketGeneratedSecret(ctx context.Context, backupBucket *extensionsv1alpha1.BackupBucket) error {
	generatedSecret := &corev1.Secret{ObjectMeta: backupbucket.GeneratedSecretObjectMeta(backupBucket)}
	if _, err := controllerutil.CreateOrUpdate(ctx, a.client, generatedSecret, func() error {
		return nil
	}); err != nil {
		return err
	}

	patch := client.MergeFrom(backupBucket.DeepCopy())
	backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
		Name:      generatedSecret.Name,
		Namespace: generatedSecret.Namespace,
	}
	return a.client.Status().Patch(ctx, backupBucket, patch)
}
