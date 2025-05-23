// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/controller/backupoptions"
)

type actuator struct {
	client             client.Client
	containerMountPath string
	backBucketPath     string
}

func newActuator(mgr manager.Manager, containerMountPath, backupBucketPath string) genericactuator.BackupEntryDelegate {
	return &actuator{
		client:             mgr.GetClient(),
		containerMountPath: containerMountPath,
		backBucketPath:     backupBucketPath,
	}
}

func (a *actuator) GetETCDSecretData(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.BackupEntry, backupSecretData map[string][]byte) (map[string][]byte, error) {
	backupSecretData[backupoptions.EtcdBackupSecretHostPath] = []byte(filepath.Join(a.containerMountPath))
	return backupSecretData, nil
}

func (a *actuator) Delete(_ context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	entryName := strings.TrimPrefix(be.Name, v1beta1constants.BackupSourcePrefix+"-")
	path := filepath.Join(a.backBucketPath, be.Spec.BucketName, entryName)
	log.Info("Deleting directory", "path", path)
	return os.RemoveAll(path)
}
