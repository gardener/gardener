// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the Gardener Controller Manager reconciliation.

	Test: Check if RBAC is enabled.
	Expected Output
	- Is enabled.

	Test: Create Service Account in non-garden namespace.
	Expected Output
	- Service account should not have access to garden namespace.
 **/

package rbac

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/framework"
)

const (
	rbacEnabledTimeout              = 60 * time.Second
	serviceAccountPermissionTimeout = 60 * time.Second
)

var (
	labels = map[string]string{"testmachinery.gardener.cloud/name": "rbac"}
)

var _ = ginkgo.Describe("RBAC testing", func() {

	f := framework.NewGardenerFramework(nil)

	f.Release().CIt("Should have rbac enabled", func(_ context.Context) {
		apiGroups, err := f.GardenClient.Kubernetes().Discovery().ServerGroups()
		framework.ExpectNoError(err)

		hasRBACEnabled := false
		for _, group := range apiGroups.Groups {
			if group.Name == rbacv1.GroupName {
				hasRBACEnabled = true
			}
		}

		g.Expect(hasRBACEnabled).To(g.BeTrue())

	}, rbacEnabledTimeout)

	f.Release().CIt("service account should not have access to garden namespace", func(ctx context.Context) {
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    f.ProjectNamespace,
				Labels:       labels,
			},
		}

		err := f.GardenClient.Client().Create(ctx, serviceAccount)
		framework.ExpectNoError(err)

		saClient, err := framework.NewClientFromServiceAccount(ctx, f.GardenClient, serviceAccount)
		framework.ExpectNoError(err)

		shoots := &gardencorev1beta1.ShootList{}
		err = saClient.Client().List(ctx, shoots, client.InNamespace(v1beta1constants.GardenNamespace))
		g.Expect(err).To(g.HaveOccurred())
		g.Expect(errors.IsForbidden(err)).To(g.BeTrue())
	}, serviceAccountPermissionTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		framework.ExpectNoError(f.GardenClient.Client().DeleteAllOf(
			ctx,
			&corev1.ServiceAccount{},
			client.InNamespace(f.ProjectNamespace),
			client.MatchingLabels(labels)),
		)
	}, serviceAccountPermissionTimeout))

})
