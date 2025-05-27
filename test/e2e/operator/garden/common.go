// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

func defaultBackupSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "virtual-garden-etcd-main-backup",
			Namespace: v1beta1constants.GardenNamespace,
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
							Region:     ptr.To("local"),
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
