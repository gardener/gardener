// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/gardener/gardener/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		if err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: f.ProjectNamespace, Name: *shootName}, shoot); err != nil {
			if apierrors.IsNotFound(err) {
				Skip("shoot is already deleted")
			}
			Expect(err).ToNot(HaveOccurred())
		}

		// Dump gardener state if delete shoot is in exit handler
		if os.Getenv("TM_PHASE") == "Exit" {
			if shootFramework, err := f.NewShootFramework(shoot); err == nil {
				shootFramework.DumpState(ctx)
			} else {
				f.DumpState(ctx)
			}
		}

		if err := f.DeleteShootAndWaitForDeletion(ctx, shoot); err != nil && !errors.IsNotFound(err) {
			if shootFramework, err := f.NewShootFramework(shoot); err == nil {
				shootFramework.DumpState(ctx)
			}
			f.Logger.Fatalf("Cannot delete shoot %s: %s", *shootName, err.Error())
		}
	}, 1*time.Hour)
})
