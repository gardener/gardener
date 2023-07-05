// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry

import (
	"context"
	"os"
	"path/filepath"

	etcddruidutils "github.com/gardener/etcd-druid/pkg/utils"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
	backupSecretData[etcddruidutils.EtcdBackupSecretHostPath] = []byte(filepath.Join(a.containerMountPath))
	return backupSecretData, nil
}

func (a *actuator) Delete(_ context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	path := filepath.Join(a.backBucketPath, be.Spec.BucketName, be.Name)
	log.Info("Deleting directory", "path", path)
	return os.RemoveAll(path)
}
