// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericactuator

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	backupEntryDelegate BackupEntryDelegate
	client              client.Client
	logger              logr.Logger
}

// InjectClient injects the given client into the valuesProvider.
func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// InjectFunc enables injecting Kubernetes dependencies into actuator's dependencies.
func (a *actuator) InjectFunc(f inject.Func) error {
	return f(a.backupEntryDelegate)
}

// NewActuator creates a new Actuator that updates the status of the handled BackupEntry resources.
func NewActuator(backupEntryDelegate BackupEntryDelegate, logger logr.Logger) backupentry.Actuator {
	return &actuator{
		logger:              logger,
		backupEntryDelegate: backupEntryDelegate,
	}
}

// Reconcile reconciles the update of a BackupEntry.
func (a *actuator) Reconcile(ctx context.Context, be *extensionsv1alpha1.BackupEntry) error {
	return a.deployEtcdBackupSecret(ctx, be)
}

func (a *actuator) deployEtcdBackupSecret(ctx context.Context, be *extensionsv1alpha1.BackupEntry) error {
	shootTechnicalID, _ := backupentry.ExtractShootDetailsFromBackupEntryName(be.Name)
	namespace := &corev1.Namespace{}
	if err := a.client.Get(ctx, kutil.Key(shootTechnicalID), namespace); err != nil {
		if apierrors.IsNotFound(err) {
			a.logger.Info("SeedNamespace for shoot not found. Avoiding etcd backup secret deployment")
			return nil
		}
		a.logger.Error(err, "Failed to get seed namespace")
		return err
	}
	if namespace.DeletionTimestamp != nil {
		a.logger.Info("SeedNamespace for shoot is being terminated. Avoiding etcd backup secret deployment")
		return nil
	}

	backupSecret, err := kutil.GetSecretByReference(ctx, a.client, &be.Spec.SecretRef)
	if err != nil {
		a.logger.Error(err, "Failed to read backup extension secret")
		return err
	}

	backupSecretData := backupSecret.DeepCopy().Data
	backupSecretData[v1beta1constants.DataKeyBackupBucketName] = []byte(be.Spec.BucketName)
	etcdSecretData, err := a.backupEntryDelegate.GetETCDSecretData(ctx, be, backupSecretData)
	if err != nil {
		return err
	}

	etcdSecret := emptyEtcdBackupSecret(be.Name)

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, a.client, etcdSecret, func() error {
		etcdSecret.Data = etcdSecretData
		return nil
	})
	return err
}

// Delete deletes the BackupEntry.
func (a *actuator) Delete(ctx context.Context, be *extensionsv1alpha1.BackupEntry) error {
	if err := a.deleteEtcdBackupSecret(ctx, be.Name); err != nil {
		return err
	}
	return a.backupEntryDelegate.Delete(ctx, be)
}

func (a *actuator) deleteEtcdBackupSecret(ctx context.Context, secretName string) error {
	etcdSecret := emptyEtcdBackupSecret(secretName)
	return kutil.DeleteObject(ctx, a.client, etcdSecret)
}

func emptyEtcdBackupSecret(backupEntryName string) *corev1.Secret {
	secretName := v1beta1constants.BackupSecretName
	if strings.HasPrefix(backupEntryName, v1beta1constants.BackupSourcePrefix) {
		secretName = fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, v1beta1constants.BackupSecretName)
	}
	shootTechnicalID, _ := backupentry.ExtractShootDetailsFromBackupEntryName(backupEntryName)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: shootTechnicalID,
		},
	}
}

// Restore restores the BackupEntry.
func (a *actuator) Restore(ctx context.Context, be *extensionsv1alpha1.BackupEntry) error {
	return a.Reconcile(ctx, be)
}

// Migrate migrates the BackupEntry.
func (a *actuator) Migrate(_ context.Context, _ *extensionsv1alpha1.BackupEntry) error {
	return nil
}
