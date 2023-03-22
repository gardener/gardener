// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		- Tests the deletion of a shoot
 **/

package shoot_deletion

import (
	"context"
	"flag"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/framework"
)

var shootName = flag.String("shoot-name", "", "name of the shoot")

func init() {
	framework.RegisterGardenerFrameworkFlags()
}

func validateFlags() {
	if !framework.StringSet(*shootName) {
		Fail("flag '--shoot-name' needs to be specified")
	}
}

var _ = Describe("Shoot deletion testing", func() {

	f := framework.NewGardenerFramework(nil)

	framework.CIt("Testing if Shoot can be deleted", func(ctx context.Context) {
		validateFlags()

		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: f.ProjectNamespace, Name: *shootName}}
		if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.ProjectNamespace, Name: *shootName}, shoot); err != nil {
			if apierrors.IsNotFound(err) {
				Skip("shoot is already deleted")
			}
			Expect(err).ToNot(HaveOccurred())
		}

		// Dump gardener state if delete shoot is in exit handler
		if os.Getenv("TM_PHASE") == "Exit" {
			if shootFramework, err := f.NewShootFramework(ctx, shoot); err == nil {
				shootFramework.DumpState(ctx)
			} else {
				f.DumpState(ctx)
			}
		}

		Expect(f.DeleteShootAndWaitForDeletion(ctx, shoot)).To(Succeed())
	}, 1*time.Hour)
})
