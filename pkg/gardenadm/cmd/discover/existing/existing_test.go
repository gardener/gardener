// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package existing_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/discover/existing"
	sharedtest "github.com/gardener/gardener/pkg/gardenadm/cmd/discover/internal/shared/test"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
)

var _ = Describe("Existing", func() {
	var (
		globalOpts     *cmd.Options
		stdOut, stdErr *Buffer
		command        *cobra.Command

		fs         afero.Afero
		fakeClient client.Client
		clientSet  kubernetes.Interface

		ctx       = context.Background()
		resources *sharedtest.Resources

		backupBucketSecret          *corev1.Secret
		backupBucketGeneratedSecret *corev1.Secret
		backupBucket                *gardencorev1beta1.BackupBucket
		backupEntry                 *gardencorev1beta1.BackupEntry
	)

	BeforeEach(func() {
		globalOpts = &cmd.Options{}
		globalOpts.IOStreams, _, stdOut, stdErr = clitest.NewTestIOStreams()
		globalOpts.Log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(stdErr))
		command = NewCommand(globalOpts)

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithStatusSubresource(&gardencorev1beta1.BackupBucket{}).
			WithIndex(&gardencorev1beta1.Project{}, core.ProjectNamespace, indexer.ProjectNamespaceIndexerFunc).
			WithIndex(&gardencorev1beta1.BackupEntry{}, core.BackupEntryShootRefName, indexer.BackupEntryShootRefNameIndexerFunc).
			WithIndex(&gardencorev1beta1.BackupEntry{}, core.BackupEntryShootRefNamespace, indexer.BackupEntryShootRefNamespaceIndexerFunc).
			WithIndex(&gardencorev1beta1.BackupBucket{}, core.BackupBucketShootRefName, indexer.BackupBucketShootRefNameIndexerFunc).
			WithIndex(&gardencorev1beta1.BackupBucket{}, core.BackupBucketShootRefNamespace, indexer.BackupBucketShootRefNamespaceIndexerFunc).
			Build()
		clientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		fs = afero.Afero{Fs: afero.NewMemMapFs()}

		DeferCleanup(test.WithVars(
			&NewClientSetFromFile, func(string, *runtime.Scheme) (kubernetes.Interface, error) { return clientSet, nil },
			&NewAferoFs, func() afero.Afero { return fs },
		))

		resources = sharedtest.NewResources()
		Expect(fakeClient.Create(ctx, resources.Namespace)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.Project)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.Secret)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.SecretDNS)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.SecretBinding)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.CloudProfile)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.ControllerDeploymentProvider)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.ControllerRegistrationProvider)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.ControllerDeploymentNetwork)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.ControllerRegistrationNetwork)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.ControllerDeploymentDNS)).To(Succeed())
		Expect(fakeClient.Create(ctx, resources.ControllerRegistrationDNS)).To(Succeed())

		backupBucketSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup-secret",
				Namespace: "garden",
			},
		}
		backupBucketGeneratedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup-bucket-generated-secret",
				Namespace: "garden",
			},
		}
		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-backup-bucket",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       backupBucketSecret.Name,
					Namespace:  backupBucketSecret.Namespace,
				},
				ShootRef: &corev1.ObjectReference{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
					Name:       resources.Shoot.Name,
					Namespace:  resources.Shoot.Namespace,
				},
			},
		}
		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup-entry",
				Namespace: resources.Shoot.Namespace,
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: backupBucket.Name,
				ShootRef: &corev1.ObjectReference{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
					Name:       resources.Shoot.Name,
					Namespace:  resources.Shoot.Namespace,
				},
			},
		}
	})

	Describe("#RunE", func() {
		It("should fail when the Shoot does not exist", func() {
			Expect(command.Flags().Set("name", resources.Shoot.Name)).To(Succeed())
			Expect(command.Flags().Set("namespace", resources.Shoot.Namespace)).To(Succeed())
			Expect(command.Flags().Set("kubeconfig", "some-path-to-kubeconfig")).To(Succeed())
			Expect(command.Flags().Set("config-dir", ".")).To(Succeed())

			Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("failed getting Shoot garden-test-project/test-shoot from garden cluster")))
		})

		It("should return the expected output when the Shoot exists", func() {
			Expect(fakeClient.Create(ctx, resources.Shoot)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupBucketSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupBucketGeneratedSecret)).To(Succeed())
			patch := client.MergeFrom(backupBucket.DeepCopy())
			backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
				Name:      backupBucketGeneratedSecret.Name,
				Namespace: backupBucketGeneratedSecret.Namespace,
			}
			Expect(fakeClient.Status().Patch(ctx, backupBucket, patch)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())

			Expect(command.Flags().Set("name", resources.Shoot.Name)).To(Succeed())
			Expect(command.Flags().Set("namespace", resources.Shoot.Namespace)).To(Succeed())
			Expect(command.Flags().Set("kubeconfig", "some-path-to-kubeconfig")).To(Succeed())
			Expect(command.Flags().Set("config-dir", ".")).To(Succeed())

			Expect(command.RunE(command, nil)).To(Succeed())

			Eventually(func() string { return string(stdOut.Contents()) }).Should(SatisfyAll(
				ContainSubstring("Computing required resources for Shoot..."),
				ContainSubstring("Fetching required resources for from garden cluster..."),
				ContainSubstring("Exported Namespace/"+resources.Namespace.Name),
				ContainSubstring("Exported Project/"+resources.Project.Name),
				ContainSubstring("Exported Secret/"+resources.Secret.Name),
				ContainSubstring("Exported Secret/"+resources.SecretDNS.Name),
				ContainSubstring("Exported SecretBinding/"+resources.SecretBinding.Name),
				ContainSubstring("Exported CloudProfile/"+resources.CloudProfile.Name),
				ContainSubstring("Exported ControllerDeployment/"+resources.ControllerDeploymentProvider.Name),
				ContainSubstring("Exported ControllerRegistration/"+resources.ControllerRegistrationProvider.Name),
				ContainSubstring("Exported ControllerDeployment/"+resources.ControllerDeploymentNetwork.Name),
				ContainSubstring("Exported ControllerRegistration/"+resources.ControllerRegistrationNetwork.Name),
				ContainSubstring("Exported ControllerDeployment/"+resources.ControllerDeploymentDNS.Name),
				ContainSubstring("Exported ControllerRegistration/"+resources.ControllerRegistrationDNS.Name),
				ContainSubstring("Exported Secret/"+backupBucketSecret.Name),
				ContainSubstring("Exported Secret/"+backupBucketGeneratedSecret.Name),
				ContainSubstring("Exported BackupBucket/"+backupBucket.Name),
				ContainSubstring("Exported BackupEntry/"+backupEntry.Name),
			))

			for _, path := range []string{
				fmt.Sprintf("namespace-%s.yaml", resources.Namespace.Name),
				fmt.Sprintf("project-%s.yaml", resources.Project.Name),
				fmt.Sprintf("secret-%s.yaml", resources.Secret.Name),
				fmt.Sprintf("secret-%s.yaml", resources.SecretDNS.Name),
				fmt.Sprintf("secretbinding-%s.yaml", resources.SecretBinding.Name),
				fmt.Sprintf("cloudprofile-%s.yaml", resources.CloudProfile.Name),
				fmt.Sprintf("controllerdeployment-%s.yaml", resources.ControllerDeploymentProvider.Name),
				fmt.Sprintf("controllerregistration-%s.yaml", resources.ControllerRegistrationProvider.Name),
				fmt.Sprintf("controllerdeployment-%s.yaml", resources.ControllerDeploymentNetwork.Name),
				fmt.Sprintf("controllerregistration-%s.yaml", resources.ControllerRegistrationNetwork.Name),
				fmt.Sprintf("controllerdeployment-%s.yaml", resources.ControllerDeploymentDNS.Name),
				fmt.Sprintf("controllerregistration-%s.yaml", resources.ControllerRegistrationDNS.Name),
				fmt.Sprintf("secret-%s.yaml", backupBucketSecret.Name),
				fmt.Sprintf("secret-%s.yaml", backupBucketGeneratedSecret.Name),
				fmt.Sprintf("backupbucket-%s.yaml", backupBucket.Name),
				fmt.Sprintf("backupentry-%s.yaml", backupEntry.Name),
			} {
				exists, err := fs.Exists(path)
				Expect(err).NotTo(HaveOccurred(), "for path "+path)
				Expect(exists).To(BeTrue(), "for path "+path)
			}
		})
	})
})
