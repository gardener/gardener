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

package backupbucket

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type actuator struct {
	backupbucket.Actuator
	client      client.Client
	bbDirectory string
}

func newActuator(bbDirectory string) backupbucket.Actuator {
	return &actuator{
		bbDirectory: bbDirectory,
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

func (a *actuator) Reconcile(_ context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	var (
		filePath             = filepath.Join(a.bbDirectory, bb.Name)
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

	return nil
}

func (a *actuator) Delete(_ context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	path := filepath.Join(a.bbDirectory, bb.Name)
	log.Info("Deleting directory", "path", path)
	return os.RemoveAll(path)
}
