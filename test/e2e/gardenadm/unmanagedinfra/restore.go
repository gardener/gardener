// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package unmanagedinfra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("gardenadm unmanaged infrastructure control plane restoration test", Label("gardenadm", "unmanaged-infra", "restore"), func() {
	Describe("Single-node control plane", Ordered, Label("single"), func() {
		var (
			log = logf.Log.WithName("test")

			shootClientSet                   kubernetes.Interface
			gardenClientSet                  kubernetes.Interface
			shootClusterKubeconfigPathOnHost = filepath.Join("..", "..", "..", "dev-setup", "kubeconfigs", "self-hosted-shoot", "kubeconfig")

			shootNamespace        = "garden"
			shootName             = "root"
			controlPlaneNamespace = "kube-system"

			// gardenKubeconfigPathOnHost is the virtual garden kubeconfig on the host; it is copied onto the node.
			gardenKubeconfigPathOnHost = filepath.Join("..", "..", "..", "dev-setup", "kubeconfigs", "virtual-garden", "kubeconfig")
			// gardenKubeconfigPathOnNode is where that kubeconfig is placed on the node for 'gardenadm discover existing'.
			gardenKubeconfigPathOnNode = "/virtual-garden-kubeconfig"
			// configDirOnNode holds the discovered resources consumed by 'gardenadm restore -d'.
			configDirOnNode = "/gardenadm/discover-output"

			// backupDataPathOnNode is the etcd backup path passed to 'gardenadm restore --backup-data-path'; computed
			// when copying the local backup onto the node.
			backupDataPathOnNode string
		)

		It("should create a client for the self-hosted shoot API server", func(ctx SpecContext) {
			initClientSet(ctx, &shootClientSet, shootClusterKubeconfigPathOnHost, client.Options{Scheme: kubernetes.SeedScheme})
		}, SpecTimeout(time.Minute))

		It("should ensure the self-hosted shoot is connected and a ShootState exists", func(ctx SpecContext) {
			// Idempotent: skip if a ShootState already exists, otherwise connect the shoot and drive its creation.
			// 'gardenadm discover existing' (run after the disaster) needs the Shoot and ShootState in the garden.
			By("Create a client for the garden cluster")
			initClientSet(ctx, &gardenClientSet, gardenKubeconfigPathOnHost, client.Options{Scheme: kubernetes.GardenScheme})

			shootState := &gardencorev1beta1.ShootState{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
			if err := gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shootState), shootState); err == nil {
				log.Info("ShootState already exists, skipping connect", "shootState", client.ObjectKeyFromObject(shootState))
				return
			}

			By("Copy the garden cluster kubeconfig onto the node")
			gardenKubeconfig, err := os.ReadFile(gardenKubeconfigPathOnHost) // #nosec: G304 -- variable points to a static file path
			Expect(err).NotTo(HaveOccurred())
			Eventually(ctx, func() error {
				_, _, err := execute(ctx, 0, "sh", "-c", fmt.Sprintf("echo '%s' > %s", string(gardenKubeconfig), gardenKubeconfigPathOnNode))
				return err
			}).Should(Succeed())

			By("Connect the self-hosted shoot to Gardener")
			stdOut, _, err := execute(ctx, 0, "sh", "-c", fmt.Sprintf("KUBECONFIG=%s gardenadm token create --print-connect-command --shoot-namespace=%s --shoot-name=%s", gardenKubeconfigPathOnNode, shootNamespace, shootName))
			Expect(err).NotTo(HaveOccurred())
			connectCommand := strings.Split(strings.ReplaceAll(string(stdOut.Contents()), `"`, ``), " ")
			stdOut, _, err = execute(ctx, 0, append(connectCommand, "--log-level=debug")...)
			Expect(err).NotTo(HaveOccurred())
			Eventually(ctx, stdOut).Should(gbytes.Say("Your self-hosted shoot cluster has successfully been connected to Gardener!"))

			By("Patch the Shoot status with a successful create lastOperation")
			shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
			Eventually(ctx, func(g Gomega) {
				g.Expect(gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				g.Expect(gardenClientSet.Client().Status().Patch(ctx, shoot, patch)).To(Succeed())
			}).Should(Succeed())

			By("Roll out the gardenlet Deployment to trigger ShootState creation")
			gardenletDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "gardenlet"}}
			Eventually(ctx, func(g Gomega) {
				g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(gardenletDeployment), gardenletDeployment)).To(Succeed())
				patch := client.MergeFrom(gardenletDeployment.DeepCopy())
				metav1.SetMetaDataAnnotation(&gardenletDeployment.Spec.Template.ObjectMeta, "kubectl.kubernetes.io/restartedAt", time.Now().Format(time.RFC3339))
				g.Expect(shootClientSet.Client().Patch(ctx, gardenletDeployment, patch)).To(Succeed())
			}).Should(Succeed())
			Eventually(ctx, func(g Gomega) {
				done, err := kubernetesutils.HasDeploymentRolloutCompleted(ctx, shootClientSet.Client(), controlPlaneNamespace, "gardenlet")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(done).To(BeTrue())
			}).Should(Succeed())

			By("Wait until the ShootState is created")
			Eventually(ctx, func() error {
				return gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shootState), shootState)
			}).Should(Succeed())
		}, SpecTimeout(5*time.Minute))

		It("should seed a workload ConfigMap whose survival proves the etcd data was restored", func(ctx SpecContext) {
			// Seeded before the disaster and asserted after recovery to prove the etcd data survived. Creation is
			// idempotent so re-runs against a connected environment do not fail.
			experimentalConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "experimental-configmap"},
				Data:       map[string]string{"content": "experimenting with control plane disaster recovery"},
			}
			Eventually(ctx, func() error {
				return client.IgnoreAlreadyExists(shootClientSet.Client().Create(ctx, experimentalConfigMap))
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("should trigger an etcd delta snapshot before the disaster", func(ctx SpecContext) {
			// etcd-backup-restore only takes deltas on a schedule, so we explicitly trigger one to flush the latest
			// cluster state to the backup store, exercising full+delta replay on recovery. etcd-main is a host-network
			// static Pod, so the endpoint is reachable via localhost. The request blocks until the delta is uploaded.
			By("Send an HTTP request for a delta snapshot")
			_, _, err := execute(ctx, 0, "curl", "-sk", "--fail", "https://localhost:8080/snapshot/delta")
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(time.Minute))

		It("should simulate a disaster by destroying the control plane node", func(ctx SpecContext) {
			// Stopping and removing the container with its volumes destroys the etcd data. The container is then
			// recreated (without running gardenadm), leaving the cluster broken until 'gardenadm restore' recovers it.
			By("Stop and remove the control plane node container including its volumes")
			_, _, err := dockerCommand(ctx, "stop", machineContainerName(0))
			Expect(err).NotTo(HaveOccurred())
			_, _, err = dockerCommand(ctx, "rm", "--volumes", machineContainerName(0))
			Expect(err).NotTo(HaveOccurred())

			By("Recreate the control plane node container")
			cmd := exec.CommandContext(ctx, "make", "gind-up", "SCENARIO=machines") // #nosec G204 -- Used for e2e tests only.
			cmd.Dir = filepath.Join("..", "..", "..")
			cmd.Stdout = gexec.NewPrefixedWriter("[out] ", GinkgoWriter)
			cmd.Stderr = gexec.NewPrefixedWriter("[err] ", GinkgoWriter)
			Expect(cmd.Run()).To(Succeed())
		}, SpecTimeout(5*time.Minute))

		It("should discover the Gardener configuration resources from the garden cluster", func(ctx SpecContext) {
			// The virtual garden survives the node disaster, so we copy its kubeconfig onto the recreated node and run
			// 'gardenadm discover existing' to download the resources (Shoot, ShootState, BackupBucket, BackupEntry,
			// CloudProfile, ...) that 'gardenadm restore' consumes.
			By("Copy the virtual garden kubeconfig onto the recreated node")
			_, _, err := dockerCommand(ctx, "cp", gardenKubeconfigPathOnHost, machineContainerName(0)+":"+gardenKubeconfigPathOnNode)
			Expect(err).NotTo(HaveOccurred())

			By("Run gardenadm discover existing")
			_, _, err = execute(ctx, 0, "mkdir", "-p", configDirOnNode)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = execute(ctx, 0, "gardenadm", "discover", "existing",
				"--name", shootName,
				"--namespace", shootNamespace,
				"--kubeconfig", gardenKubeconfigPathOnNode,
				"-d", configDirOnNode,
			)
			Expect(err).NotTo(HaveOccurred())

			By("Remove the self-hosted shoot lease that must not be restored")
			_, _, err = execute(ctx, 0, "rm", "-f", configDirOnNode+"/lease-self-hosted-shoot-"+shootName+".yaml")
			Expect(err).NotTo(HaveOccurred())

			By("Verify 'gardenadm discover existing' exported the resources needed for restore")
			stdOut, _, err := execute(ctx, 0, "ls", configDirOnNode)
			Expect(err).NotTo(HaveOccurred())
			discoveredFiles := string(stdOut.Contents())
			for _, kind := range []string{"backupbucket", "backupentry", "shoot", "shootstate"} {
				Expect(discoveredFiles).To(ContainSubstring(kind), "'gardenadm discover existing' should have exported a %s resource into %s", kind, configDirOnNode)
			}
		}, SpecTimeout(2*time.Minute))

		It("should copy the local etcd backup onto the recreated node", func(ctx SpecContext) {
			// The etcd backup lives on the host's local disk. Locate the '.../etcd-main/v2' backup directory (excluding
			// the garden bucket), copy the whole local-backupbuckets directory onto the node, and compute the on-node
			// --backup-data-path that 'gardenadm restore' reads from.
			localBackupBucketsOnHost := filepath.Join("..", "..", "..", "dev", "local-backupbuckets")

			By("Locate the etcd-main v2 backup directory on the host")
			var backupDataPathOnHost string
			Expect(filepath.WalkDir(localBackupBucketsOnHost, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() && filepath.Base(path) == "v2" && !strings.Contains(path, "garden") {
					backupDataPathOnHost = path
				}
				return nil
			})).To(Succeed())
			Expect(backupDataPathOnHost).NotTo(BeEmpty(), "expected to find an etcd-main v2 backup directory under %s", localBackupBucketsOnHost)
			log.Info("Found etcd backup data on host", "path", backupDataPathOnHost)

			By("Copy the local backup buckets onto the recreated node")
			_, _, err := dockerCommand(ctx, "cp", localBackupBucketsOnHost, machineContainerName(0)+":/local-backupbuckets")
			Expect(err).NotTo(HaveOccurred())

			// Translate the host path to the on-node path: strip the leading "<...>/dev/" and root it at "/", so
			// e.g. "dev/local-backupbuckets/<uid>/.../v2" becomes "/local-backupbuckets/<uid>/.../v2".
			relToBackupBuckets, err := filepath.Rel(localBackupBucketsOnHost, backupDataPathOnHost)
			Expect(err).NotTo(HaveOccurred())
			backupDataPathOnNode = filepath.Join("/local-backupbuckets", relToBackupBuckets)
			log.Info("Computed backup data path on node", "path", backupDataPathOnNode)
		}, SpecTimeout(2*time.Minute))

		It("should restore the control plane node", func(ctx SpecContext) {
			Expect(backupDataPathOnNode).NotTo(BeEmpty(), "backup data must have been copied onto the node")

			By("Run gardenadm restore")
			stdOut, _, err := execute(ctx, 0, "gardenadm", "restore",
				"-d", configDirOnNode,
				"--prior-node-name="+machineContainerName(0),
				"--backup-data-path="+backupDataPathOnNode,
				"--log-level=debug",
			)
			Expect(err).NotTo(HaveOccurred())
			log.Info("gardenadm restore finished", "output", string(stdOut.Contents()))
		}, SpecTimeout(10*time.Minute))

		It("should observe that all nodes are ready", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(shootClientSet.Client().List(ctx, nodeList)).To(Succeed())
				g.Expect(nodeList.Items).NotTo(BeEmpty())

				for _, node := range nodeList.Items {
					g.Expect(health.CheckNode(&node)).To(Succeed(), "node %q should be healthy", node.Name)
				}
			}).Should(Succeed())
		}, SpecTimeout(5*time.Minute))

		It("should verify the control plane restoration", func(ctx SpecContext) {
			// Assert the etcd data survived (the workload ConfigMap is still present with its original content) and the
			// Shoot's identity was preserved (the garden Shoot .status.uid matches the statusUID in shoot-info).
			By("Verify the default/experimental-configmap survived recovery")
			experimentalConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "experimental-configmap"}}
			Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(experimentalConfigMap), experimentalConfigMap)).To(Succeed(),
				"default/experimental-configmap should have survived recovery")
			Expect(experimentalConfigMap.Data).To(HaveKeyWithValue("content", "experimenting with control plane disaster recovery"),
				"default/experimental-configmap should have survived recovery with its original content")

			By("Read the Shoot UID from the garden cluster")
			shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
			Expect(gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			gardenUID := string(shoot.Status.UID)
			Expect(gardenUID).NotTo(BeEmpty(), "Shoot .status.uid should not be empty in the garden cluster")

			By("Read the statusUID from the kube-system/shoot-info ConfigMap on the self-hosted shoot")
			shootInfo := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "shoot-info"}}
			Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shootInfo), shootInfo)).To(Succeed())

			By("Verify the Shoot UID was preserved across recovery")
			Expect(shootInfo.Data).To(HaveKeyWithValue("statusUID", gardenUID),
				"the shoot-info statusUID should match the garden Shoot .status.uid after recovery")

			log.Info("The control plane Node was successfully restored", "shootUID", gardenUID)
		}, SpecTimeout(5*time.Minute))
	})
})
