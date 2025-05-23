// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	namespacedcloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
)

var _ = Describe("NamespacedCloudProfile controller", func() {
	Describe("#MapCloudProfileToNamespacedCloudProfile", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client
			log        = logr.Discard()

			reconciler *namespacedcloudprofilecontroller.Reconciler

			namespaceName string

			cloudProfile           *gardencorev1beta1.CloudProfile
			namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithIndex(
				&gardencorev1beta1.NamespacedCloudProfile{},
				core.NamespacedCloudProfileParentRefName,
				indexer.NamespacedCloudProfileParentRefNameIndexerFunc,
			).Build()
			reconciler = &namespacedcloudprofilecontroller.Reconciler{Client: fakeClient, Recorder: &record.FakeRecorder{}}

			namespaceName = "garden-test"

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-profile",
				},
			}

			namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "n-profile-1",
					Namespace: namespaceName,
				},
				Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
					Parent: gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: cloudProfile.Name,
					},
				},
			}
		})

		It("should successfully find all related NamespacedCloudProfiles referencing a CloudProfile", func() {
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeClient.Create(ctx, namespacedCloudProfile)).To(Succeed())

			Expect(reconciler.MapCloudProfileToNamespacedCloudProfile(log)(ctx, cloudProfile)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: "n-profile-1", Namespace: namespaceName}}))
		})

		It("should successfully return an empty result if no referencing NamespacedCloudProfiles exist", func() {
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			Expect(reconciler.MapCloudProfileToNamespacedCloudProfile(log)(ctx, cloudProfile)).To(BeEmpty())
		})
	})
})
