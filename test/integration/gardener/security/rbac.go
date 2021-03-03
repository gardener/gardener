// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/test/framework"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/onsi/ginkgo"
	g "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rbacEnabledTimeout              = 60 * time.Second
	serviceAccountPermissionTimeout = 60 * time.Second
)

var _ = ginkgo.Describe("RBAC testing", func() {

	f := framework.NewGardenerFramework(nil)

	f.Release().CIt("Should have rbac enabled", func(ctx context.Context) {
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
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    f.ProjectNamespace,
			},
		}

		err := f.GardenClient.Client().Create(ctx, serviceAccount)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(f.GardenClient.Client().Delete(ctx, serviceAccount))
		}()

		err = retry.UntilTimeout(ctx, 10*time.Second, serviceAccountPermissionTimeout, func(ctx context.Context) (bool, error) {
			newServiceAccount := &corev1.ServiceAccount{}
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: serviceAccount.Namespace, Name: serviceAccount.Name}, newServiceAccount); err != nil {
				return retry.MinorError(err)
			}
			serviceAccount = newServiceAccount
			if len(serviceAccount.Secrets) != 0 {
				return retry.Ok()
			}
			return retry.NotOk()
		})
		framework.ExpectNoError(err)

		saClient, err := framework.NewClientFromServiceAccount(ctx, f.GardenClient, serviceAccount)
		framework.ExpectNoError(err)

		shoots := &gardencorev1beta1.ShootList{}
		err = saClient.Client().List(ctx, shoots, client.InNamespace(v1beta1constants.GardenNamespace))
		g.Expect(err).To(g.HaveOccurred())
		g.Expect(errors.IsForbidden(err)).To(g.BeTrue())
	}, serviceAccountPermissionTimeout)

})
