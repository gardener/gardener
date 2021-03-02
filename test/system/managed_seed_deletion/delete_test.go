// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managed_seed_deletion

import (
	"context"
	"flag"
	"os"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var managedSeedName = flag.String("managed-seed-name", "", "name of the managed seed")

func init() {
	framework.RegisterGardenerFrameworkFlags()
}

func validateFlags() {
	if !framework.StringSet(*managedSeedName) {
		Fail("flag '-managed-seed-name' needs to be specified")
	}
}

var _ = Describe("ManagedSeed deletion testing", func() {
	f := framework.NewGardenerFramework(nil)

	framework.CIt("Delete ManagedSeed", func(ctx context.Context) {
		validateFlags()

		managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
		if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: *managedSeedName}, managedSeed); err != nil {
			if apierrors.IsNotFound(err) {
				Skip("managed seed is already deleted")
			}
			Expect(err).ToNot(HaveOccurred())
		}

		// Dump gardener state if delete managed seed is in exit handler
		if os.Getenv("TM_PHASE") == "Exit" {
			f.DumpState(ctx)
		}

		if err := f.DeleteManagedSeed(ctx, managedSeed); err != nil && !apierrors.IsNotFound(err) {
			f.Logger.Fatalf("Could not delete managed seed %s: %s", *managedSeedName, err.Error())
		}
	}, 1*time.Hour)
})
