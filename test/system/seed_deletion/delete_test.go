// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

/**
	Overview
		- Tests the deletion of a seed
 **/

package seed_deletion

import (
	"context"
	"flag"
	"os"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var seedName *string

func init() {
	seedName = flag.String("seed-name", "", "name of the seed")
	framework.RegisterGardenerFrameworkFlags()
}

func validateFlags() {
	if !framework.StringSet(*seedName) {
		Fail("flag '-seed-name' needs to be specified")
	}
}

var _ = Describe("Seed deletion testing", func() {

	f := framework.NewGardenerFramework(nil)

	framework.CIt("Testing if Seed can be deleted", func(ctx context.Context) {
		validateFlags()
		if err := deleteSeed(ctx, f); client.IgnoreNotFound(err) != nil {
			Fail(err.Error())
		}
	}, 1*time.Hour)
})

func dumpStateOnTMPhaseExit(ctx context.Context, f *framework.GardenerFramework, seedFramework *framework.SeedRegistrationFramework, shootFramework *framework.ShootFramework) {
	// Dump gardener state if delete seed or shoot is in exit handler
	if os.Getenv("TM_PHASE") == "Exit" {
		if seedFramework != nil {
			seedFramework.DumpState(ctx)
		} else if shootFramework != nil {
			shootFramework.DumpState(ctx)
		} else {
			f.DumpState(ctx)
		}
	}
}

func deleteSeed(ctx context.Context, f *framework.GardenerFramework) error {
	seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: *seedName}}
	seedFramework, err := f.NewSeedRegistrationFramework(seed)
	if err != nil {
		f.DumpState(ctx)
		return err
	}
	dumpStateOnTMPhaseExit(ctx, f, seedFramework, nil)

	f.Logger.Info("Deleting backupbucket...")
	if err := deleteBackupbucket(ctx, f, *seedName); client.IgnoreNotFound(err) != nil {
		return err
	}

	f.Logger.Info("Deleting seed...")
	if err := f.DeleteSeedAndWaitForDeletion(ctx, seed); client.IgnoreNotFound(err) != nil {
		f.Logger.Errorf("Cannot delete seed %s: %s", *seedName, err.Error())
		return err
	}

	f.Logger.Info("Deleting secret...")
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: seed.Spec.SecretRef.Name, Namespace: seed.Spec.SecretRef.Namespace}}
	if err := f.GardenClient.DirectClient().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		err = errors.Wrapf(err, "Secret %s/%s can't be deleted", seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
		return err
	}
	f.Logger.Infof("Secret %s/%s deleted successfully", seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
	return nil
}

func deleteBackupbucket(ctx context.Context, f *framework.GardenerFramework, seedName string) error {
	backupbuckets := &gardencorev1beta1.BackupBucketList{}

	if err := f.GardenClient.DirectClient().List(ctx, backupbuckets); err != nil {
		return err
	}

	for _, backupbucket := range backupbuckets.Items {
		if backupbucket.Spec.SeedName != nil && *backupbucket.Spec.SeedName == seedName {
			f.Logger.Infof("Backupbucket found: %s", backupbucket.Name)
			return f.GardenClient.DirectClient().Delete(ctx, &backupbucket)
		}
	}
	return nil
}
