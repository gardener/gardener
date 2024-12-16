// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/install"
	"github.com/gardener/gardener/pkg/utils/retry"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/node"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		It("Create, Hibernate, Wake up and Delete Shoot", Offset(1), func() {
			ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()

			if shoot.Spec.CloudProfileName == nil && shoot.Spec.CloudProfile != nil && shoot.Spec.CloudProfile.Kind == "NamespacedCloudProfile" {
				By("Create NamespacedCloudProfile")
				namespacedCloudProfile := addCustomMachineImage(e2e.DefaultNamespacedCloudProfile())
				Expect(f.GardenClient.Client().Create(ctx, namespacedCloudProfile)).To(Or(Succeed()))
				DeferCleanup(func() {
					By("Delete NamespacedCloudProfile")
					ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
					defer cancel()
					Expect(f.GardenClient.Client().Delete(ctx, namespacedCloudProfile)).To(Or(Succeed(), BeNotFoundError()))
				})

				By("Wait for new NamespacedCloudProfile to be reconciled")
				Expect(retry.UntilTimeout(ctx, 10*time.Second, 60*time.Second, func(ctx context.Context) (done bool, err error) {
					err = f.GardenClient.Client().Get(ctx, k8sclient.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
					if err != nil {
						return retry.SevereError(err)
					}
					if namespacedCloudProfile.Status.ObservedGeneration != namespacedCloudProfile.Generation {
						return retry.MinorError(fmt.Errorf("namespaced cloud profile exists but has not been reconciled yet"))
					}
					return retry.Ok()
				})).To(Succeed())

				By("Check for correct mutation of the status provider config")
				scheme := f.GardenClient.Client().Scheme()
				utilruntime.Must(install.AddToScheme(scheme))

				cloudProfileConfig := &api.CloudProfileConfig{}
				decoder := serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig).To(Not(BeNil()))
				Expect(util.Decode(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw, cloudProfileConfig)).To(Succeed())
				Expect(cloudProfileConfig.MachineImages).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Name": Equal("nscpfl-machine-image-1"),
					"Versions": ContainElements(
						api.MachineImageVersion{Version: "1.1", Image: "local/image:1.1"},
					),
				})))
			}

			By("Create Shoot")
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				By("Verify Bootstrapping of Nodes with node-critical components")
				// We verify the node readiness feature in this specific e2e test because it uses a single-node shoot cluster.
				// The default shoot e2e test deals with multiple nodes, deleting all of them and waiting for them to be recreated
				// might increase the test duration undesirably.
				ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
				defer cancel()
				node.VerifyNodeCriticalComponentsBootstrapping(ctx, f.ShootFramework)
			}

			By("Hibernate Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.HibernateShoot(ctx, f.Shoot)).To(Succeed())

			By("Wake up Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.WakeUpShoot(ctx, f.Shoot)).To(Succeed())

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", Label("basic"), func() {
		test(e2e.DefaultShoot("e2e-wake-up"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-wake-up"))
	})

	Context("Shoot with workers with NamespacedCloudProfile", Label("basic"), func() {
		shoot := e2e.DefaultShoot("e2e-wake-up-ncp")
		shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
			Kind: "NamespacedCloudProfile",
			Name: "my-profile",
		}
		test(shoot)
	})
})

func addCustomMachineImage(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) *gardencorev1beta1.NamespacedCloudProfile {
	namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
		{
			Name:           "nscpfl-machine-image-1",
			UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMinor),
			Versions: []gardencorev1beta1.MachineImageVersion{
				{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1"}, Architectures: []string{"amd64"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
			},
		},
	}
	namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{
		Raw: []byte(`{
			"apiVersion":"local.provider.extensions.gardener.cloud/v1alpha1",
			"kind":"CloudProfileConfig",
			"machineImages":[
			 {"name":"nscpfl-machine-image-1","versions":[{"version":"1.1","image":"local/image:1.1"}]}
			]}`),
	}
	return namespacedCloudProfile
}
