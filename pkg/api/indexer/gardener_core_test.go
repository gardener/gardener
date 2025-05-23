// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Core", func() {
	var indexer *fakeFieldIndexer

	BeforeEach(func() {
		indexer = &fakeFieldIndexer{}
	})

	DescribeTable("#AddProjectNamespace",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddProjectNamespace(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.Project{}))
			Expect(indexer.field).To(Equal("spec.namespace"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no Project", &corev1.Secret{}, ConsistOf("")),
		Entry("Project w/o namespace", &gardencorev1beta1.Project{}, ConsistOf("")),
		Entry("Project w/ namespace", &gardencorev1beta1.Project{Spec: gardencorev1beta1.ProjectSpec{Namespace: ptr.To("namespace")}}, ConsistOf("namespace")),
	)

	DescribeTable("#AddShootSeedName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddShootSeedName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.Shoot{}))
			Expect(indexer.field).To(Equal("spec.seedName"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no Shoot", &corev1.Secret{}, ConsistOf("")),
		Entry("Shoot w/o seedName", &gardencorev1beta1.Shoot{}, ConsistOf("")),
		Entry("Shoot w/ seedName", &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{SeedName: ptr.To("seed")}}, ConsistOf("seed")),
	)

	DescribeTable("#AddShootStatusSeedName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddShootStatusSeedName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.Shoot{}))
			Expect(indexer.field).To(Equal("status.seedName"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no Shoot", &corev1.Secret{}, ConsistOf("")),
		Entry("Shoot w/o seedName", &gardencorev1beta1.Shoot{}, ConsistOf("")),
		Entry("Shoot w/ seedName", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{SeedName: ptr.To("seed")}}, ConsistOf("seed")),
	)

	DescribeTable("#AddBackupBucketSeedName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddBackupBucketSeedName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.BackupBucket{}))
			Expect(indexer.field).To(Equal("spec.seedName"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no BackupBucket", &corev1.Secret{}, ConsistOf("")),
		Entry("BackupBucket w/o seedName", &gardencorev1beta1.BackupBucket{}, ConsistOf("")),
		Entry("BackupBucket w/ seedName", &gardencorev1beta1.BackupBucket{Spec: gardencorev1beta1.BackupBucketSpec{SeedName: ptr.To("seed")}}, ConsistOf("seed")),
	)

	DescribeTable("#AddBackupEntrySeedName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddBackupEntrySeedName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.BackupEntry{}))
			Expect(indexer.field).To(Equal("spec.seedName"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no BackupEntry", &corev1.Secret{}, ConsistOf("")),
		Entry("BackupEntry w/o seedName", &gardencorev1beta1.BackupEntry{}, ConsistOf("")),
		Entry("BackupEntry w/ seedName", &gardencorev1beta1.BackupEntry{Spec: gardencorev1beta1.BackupEntrySpec{SeedName: ptr.To("seed")}}, ConsistOf("seed")),
	)

	DescribeTable("#AddBackupEntryBucketName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddBackupEntryBucketName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.BackupEntry{}))
			Expect(indexer.field).To(Equal("spec.bucketName"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no BackupEntry", &corev1.Secret{}, ConsistOf("")),
		Entry("BackupEntry w/o bucketName", &gardencorev1beta1.BackupEntry{}, ConsistOf("")),
		Entry("BackupEntry w/ bucketName", &gardencorev1beta1.BackupEntry{Spec: gardencorev1beta1.BackupEntrySpec{BucketName: "bucket"}}, ConsistOf("bucket")),
	)

	DescribeTable("#AddControllerInstallationSeedRefName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddControllerInstallationSeedRefName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.ControllerInstallation{}))
			Expect(indexer.field).To(Equal("spec.seedRef.name"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no ControllerInstallation", &corev1.Secret{}, ConsistOf("")),
		Entry("ControllerInstallation w/o seedRef", &gardencorev1beta1.ControllerInstallation{}, ConsistOf("")),
		Entry("ControllerInstallation w/ seedRef", &gardencorev1beta1.ControllerInstallation{Spec: gardencorev1beta1.ControllerInstallationSpec{SeedRef: corev1.ObjectReference{Name: "seed"}}}, ConsistOf("seed")),
	)

	DescribeTable("#AddControllerInstallationRegistrationRefName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddControllerInstallationRegistrationRefName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.ControllerInstallation{}))
			Expect(indexer.field).To(Equal("spec.registrationRef.name"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no ControllerInstallation", &corev1.Secret{}, ConsistOf("")),
		Entry("ControllerInstallation w/o registrationRef", &gardencorev1beta1.ControllerInstallation{}, ConsistOf("")),
		Entry("ControllerInstallation w/ registrationRef", &gardencorev1beta1.ControllerInstallation{Spec: gardencorev1beta1.ControllerInstallationSpec{RegistrationRef: corev1.ObjectReference{Name: "registration"}}}, ConsistOf("registration")),
	)

	DescribeTable("#AddInternalSecretType",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddInternalSecretType(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&gardencorev1beta1.InternalSecret{}))
			Expect(indexer.field).To(Equal("type"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no InternalSecret", &corev1.Secret{}, ConsistOf("")),
		Entry("InternalSecret w/o type", &gardencorev1beta1.InternalSecret{}, ConsistOf("")),
		Entry("InternalSecret w/ type", &gardencorev1beta1.InternalSecret{Type: corev1.SecretTypeBootstrapToken}, ConsistOf("bootstrap.kubernetes.io/token")),
	)
})
