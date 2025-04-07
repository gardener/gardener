// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const namespace = "garden"

var (
	parentCtx     context.Context
	runtimeClient client.Client
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)).WithName("garden-test"))

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&componentbaseconfigv1alpha1.ClientConnectionConfiguration{Kubeconfig: os.Getenv("KUBECONFIG")}, nil, kubernetes.AuthTokenFile)
	Expect(err).NotTo(HaveOccurred())

	runtimeClient, err = client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	parentCtx = context.Background()
})

func defaultBackupSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "virtual-garden-etcd-main-backup",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"hostPath": []byte("/etc/gardener/local-backupbuckets")},
	}
}

func defaultGarden(backupSecret *corev1.Secret, specifyBackupBucket bool) *operatorv1alpha1.Garden {
	randomSuffix, err := utils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred())
	name := "garden-" + randomSuffix

	var bucketName *string
	if specifyBackupBucket {
		bucketName = ptr.To("gardener-operator/" + name)
	}

	return &operatorv1alpha1.Garden{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: operatorv1alpha1.GardenSpec{
			Extensions: []operatorv1alpha1.GardenExtension{{
				Type: "local-ext-shoot",
			}},
			RuntimeCluster: operatorv1alpha1.RuntimeCluster{
				Networking: operatorv1alpha1.RuntimeNetworking{
					Pods:     []string{"10.1.0.0/16"},
					Services: []string{"10.2.0.0/16"},
				},
				Ingress: operatorv1alpha1.Ingress{
					Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.runtime-garden.local.gardener.cloud"}},
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				Provider: operatorv1alpha1.Provider{
					Region: ptr.To("local"),
					Zones:  []string{"0", "1", "2"},
				},
				Settings: &operatorv1alpha1.Settings{
					VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
						Enabled: ptr.To(true),
					},
					TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
						Enabled: true,
					},
				},
			},
			VirtualCluster: operatorv1alpha1.VirtualCluster{
				ControlPlane: &operatorv1alpha1.ControlPlane{
					HighAvailability: &operatorv1alpha1.HighAvailability{},
				},
				DNS: operatorv1alpha1.DNS{
					Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
				},
				ETCD: &operatorv1alpha1.ETCD{
					Main: &operatorv1alpha1.ETCDMain{
						Backup: &operatorv1alpha1.Backup{
							Provider:   "local",
							BucketName: bucketName,
							SecretRef: corev1.LocalObjectReference{
								Name: backupSecret.Name,
							},
						},
					},
				},
				Gardener: operatorv1alpha1.Gardener{
					ClusterIdentity: "e2e-test",
					Dashboard: &operatorv1alpha1.GardenerDashboardConfig{
						Terminal: &operatorv1alpha1.DashboardTerminal{
							Container: operatorv1alpha1.DashboardTerminalContainer{Image: "busybox:latest"},
						},
					},
				},
				Kubernetes: operatorv1alpha1.Kubernetes{
					Version: "1.31.5",
				},
				Maintenance: operatorv1alpha1.Maintenance{
					TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
						Begin: "220000+0100",
						End:   "230000+0100",
					},
				},
				Networking: operatorv1alpha1.Networking{
					Services: []string{"100.64.0.0/13"},
				},
			},
		},
	}
}

func waitForGardenToBeReconciledAndHealthy(ctx context.Context, garden *operatorv1alpha1.Garden) {
	CEventually(ctx, func(g Gomega) bool {
		g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())

		completed, reason := gardenReconciliationSuccessful(garden)
		if !completed {
			logf.Log.Info("Waiting for reconciliation and healthiness", "lastOperation", garden.Status.LastOperation, "reason", reason)
		}
		return completed
	}).WithPolling(10 * time.Second).Should(BeTrue())
}

func waitForGardenToBeDeleted(ctx context.Context, garden *operatorv1alpha1.Garden) {
	CEventually(ctx, func() error {
		return runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
	}).WithPolling(2 * time.Second).Should(BeNotFoundError())
}

func waitForExtensionToReportDeletion(ctx context.Context, name string) {
	extension := &operatorv1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	CEventually(ctx, func(g Gomega) {
		g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
		g.Expect(extension.Status.Conditions).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("DeleteSuccessful"),
		))
	}).WithPolling(2 * time.Second).Should(Succeed())
}

func cleanupVolumes(ctx context.Context) {
	Expect(runtimeClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(namespace))).To(Succeed())

	CEventually(ctx, func(g Gomega) bool {
		pvList := &corev1.PersistentVolumeList{}
		g.Expect(runtimeClient.List(ctx, pvList)).To(Succeed())

		for _, pv := range pvList.Items {
			if pv.Spec.ClaimRef != nil &&
				pv.Spec.ClaimRef.APIVersion == "v1" &&
				pv.Spec.ClaimRef.Kind == "PersistentVolumeClaim" &&
				pv.Spec.ClaimRef.Namespace == namespace {
				return false
			}
		}

		return true
	}).WithPolling(2 * time.Second).Should(BeTrue())
}

func gardenReconciliationSuccessful(garden *operatorv1alpha1.Garden) (bool, string) {
	if garden.Generation != garden.Status.ObservedGeneration {
		return false, "garden generation did not equal observed generation"
	}
	if len(garden.Status.Conditions) == 0 && garden.Status.LastOperation == nil {
		return false, "no conditions and last operation present yet"
	}

	for _, condition := range garden.Status.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return false, fmt.Sprintf("condition type %s is not true yet, had message %s with reason %s", condition.Type, condition.Message, condition.Reason)
		}
	}

	if garden.Status.LastOperation != nil {
		if garden.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
			return false, "last operation state is not succeeded"
		}
	}

	return true, ""
}
