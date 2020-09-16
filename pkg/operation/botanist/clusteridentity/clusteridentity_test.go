// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteridentity_test

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/clusteridentity"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("#ClusterIdentity", func() {
	const (
		shootName             = "shootName"
		shootNamespace        = "shootNamespace"
		shootSeedNamespace    = "shootSeedNamespace"
		shootUID              = "shootUID"
		gardenClusterIdentity = "garden-cluster-identity"
	)

	var (
		ctx          context.Context
		gardenClient client.Client
		seedClient   client.Client

		shoot *gardencorev1beta1.Shoot

		logger          *logrus.Entry
		defaultDeployer component.Deployer

		expectedShootClusterIdentity = fmt.Sprintf("%s-%s-%s", shootSeedNamespace, shootUID, gardenClusterIdentity)
	)

	BeforeEach(func() {
		ctx = context.TODO()
		logger = logrus.NewEntry(logrus.New())

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(gardencorev1beta1.AddToScheme(s))

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
		}

		cluster := &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootSeedNamespace,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{Object: shoot},
			},
		}

		gardenClient = fake.NewFakeClientWithScheme(s, shoot)
		seedClient = fake.NewFakeClientWithScheme(s, cluster)

		defaultDeployer = clusteridentity.New(shoot.Status.ClusterIdentity, gardenClusterIdentity, shootName, shootNamespace, shootSeedNamespace, shootUID, gardenClient, seedClient, logger)
	})

	Describe("#Deploy", func() {
		DescribeTable("ClusterIdentity", func(mutator func()) {
			mutator()

			Expect(defaultDeployer.Deploy(ctx)).NotTo(HaveOccurred())

			Expect(gardenClient.Get(ctx, kutil.Key(shootNamespace, shootName), shoot)).NotTo(HaveOccurred())
			Expect(shoot.Status.ClusterIdentity).NotTo(BeNil())
			Expect(*shoot.Status.ClusterIdentity).To(Equal(expectedShootClusterIdentity))
		},
			Entry("cluster identity is nil", func() {
				shoot.Status.ClusterIdentity = nil
			}),
			Entry("cluster idenitty already exists", func() {
				shoot.Status.ClusterIdentity = pointer.StringPtr(expectedShootClusterIdentity)
			}),
		)
	})
})
