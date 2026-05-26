// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	extensionsmockcloudprovider "github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider/mock"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestCloudProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Webhook CloudProvider Suite")
}

var _ = Describe("Mutator", func() {
	var (
		ctrl       *gomock.Controller
		mgr        test.FakeManager
		fakeClient client.Client
		logger     = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		fakeClient = fakeclient.NewClientBuilder().Build()
		mgr = test.FakeManager{Client: fakeClient}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Mutate", func() {
		var (
			ensurer        *extensionsmockcloudprovider.MockEnsurer
			newSecret, old *corev1.Secret
			mutator        webhook.Mutator
		)

		BeforeEach(func() {
			ensurer = extensionsmockcloudprovider.NewMockEnsurer(ctrl)
			mutator = cloudprovider.NewMutator(mgr, logger, ensurer)
			newSecret = nil
			old = nil
		})

		It("Should ignore secrets other than cloudprovider", func() {
			newSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
			err := mutator.Mutate(context.TODO(), newSecret, old)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should mutate cloudprovider secret", func() {
			newSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCloudProvider}}

			ensurer.EXPECT().EnsureCloudProviderSecret(context.TODO(), gomock.Any(), newSecret, old)
			err := mutator.Mutate(context.TODO(), newSecret, old)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
