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

package gardener_rbac_test

import (
	"context"
	"flag"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/test/integration/framework"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	kubeconfigPath   = flag.String("kubeconfig", "", "the path to the kubeconfig path  of the garden cluster that will be used for integration tests")
	projectNamespace = flag.String("project-namespace", "garden-it", "garden project to create the service account")
	logLevel         = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

const (
	RBACEnabledTimeout              = 60 * time.Second
	ServiceAccountPermissionTimeout = 60 * time.Second
	InitializationTimeout           = 20 * time.Second
	DumpStateTimeout                = 5 * time.Minute
)

func validateFlags() {

	if !StringSet(*kubeconfigPath) {
		Fail("you need to specify the correct path for the kubeconfigpath")
	}

	if !FileExists(*kubeconfigPath) {
		Fail("kubeconfigpath path does not exist")
	}
}

var _ = Describe("RBAC testing", func() {
	var (
		gardenerTestOperation *framework.GardenerTestOperation
		gardenClient          kubernetes.Interface
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		var err error
		gardenClient, err = kubernetes.NewClientFromFile("", *kubeconfigPath, kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.GardenScheme,
			}),
		)
		Expect(err).ToNot(HaveOccurred())

		testLogger := logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)
		gardenerTestOperation, err = framework.NewGardenTestOperation(ctx, gardenClient, testLogger, nil)
		Expect(err).ToNot(HaveOccurred())

	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenerTestOperation.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("Should have rbac enabled", func(ctx context.Context) {
		apiGroups, err := gardenClient.Kubernetes().Discovery().ServerGroups()
		Expect(err).ToNot(HaveOccurred())

		hasRBACEnabled := false
		for _, group := range apiGroups.Groups {
			if group.Name == rbacv1.GroupName {
				hasRBACEnabled = true
			}
		}

		Expect(hasRBACEnabled).To(BeTrue())

	}, RBACEnabledTimeout)

	CIt("service account should not have access to garden namespace", func(ctx context.Context) {
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    *projectNamespace,
			},
		}

		err := gardenClient.Client().Create(ctx, serviceAccount)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			Expect(gardenClient.Client().Delete(ctx, serviceAccount)).ToNot(HaveOccurred())
		}()

		err = retry.UntilTimeout(ctx, 10*time.Second, ServiceAccountPermissionTimeout, func(ctx context.Context) (bool, error) {
			newServiceAccount := &corev1.ServiceAccount{}
			if err := gardenClient.Client().Get(ctx, client.ObjectKey{Namespace: serviceAccount.Namespace, Name: serviceAccount.Name}, newServiceAccount); err != nil {
				return retry.MinorError(err)
			}
			serviceAccount = newServiceAccount
			if len(serviceAccount.Secrets) != 0 {
				return retry.Ok()
			}
			return retry.NotOk()
		})
		Expect(err).ToNot(HaveOccurred())

		saClient, err := framework.NewClientFromServiceAccount(ctx, gardenClient, serviceAccount)
		Expect(err).ToNot(HaveOccurred())

		shoots := &v1beta1.ShootList{}
		err = saClient.Client().List(ctx, shoots, client.InNamespace(common.GardenNamespace))
		Expect(err).To(HaveOccurred())
		Expect(errors.IsForbidden(err)).To(BeTrue())
	}, ServiceAccountPermissionTimeout)

})
