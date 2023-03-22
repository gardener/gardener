// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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

var _ = BeforeEach(func() {
	parentCtx = context.Background()

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&componentbaseconfig.ClientConnectionConfiguration{Kubeconfig: os.Getenv("KUBECONFIG")}, nil, kubernetes.AuthTokenFile)
	Expect(err).NotTo(HaveOccurred())

	runtimeClient, err = client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
	Expect(err).NotTo(HaveOccurred())
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

func defaultGarden(backupSecret *corev1.Secret) *operatorv1alpha1.Garden {
	randomSuffix, err := utils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred())
	name := "garden-" + randomSuffix

	return &operatorv1alpha1.Garden{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: operatorv1alpha1.GardenSpec{
			RuntimeCluster: operatorv1alpha1.RuntimeCluster{
				Provider: operatorv1alpha1.Provider{
					Zones: []string{"0", "1", "2"},
				},
				Settings: &operatorv1alpha1.Settings{
					VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
						Enabled: pointer.Bool(true),
					},
				},
			},
			VirtualCluster: operatorv1alpha1.VirtualCluster{
				ControlPlane: &operatorv1alpha1.ControlPlane{
					HighAvailability: &operatorv1alpha1.HighAvailability{},
				},
				ETCD: &operatorv1alpha1.ETCD{
					Main: &operatorv1alpha1.ETCDMain{
						Backup: &operatorv1alpha1.Backup{
							Provider:   "local",
							BucketName: "gardener-operator/" + name,
							SecretRef: corev1.SecretReference{
								Name:      backupSecret.Name,
								Namespace: backupSecret.Namespace,
							},
						},
					},
				},
				Kubernetes: operatorv1alpha1.Kubernetes{
					Version: "1.26.3",
				},
				Maintenance: operatorv1alpha1.Maintenance{
					TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
						Begin: "220000+0100",
						End:   "230000+0100",
					},
				},
			},
		},
	}
}

func waitForGardenToBeReconciled(ctx context.Context, garden *operatorv1alpha1.Garden) {
	CEventually(ctx, func(g Gomega) []gardencorev1beta1.Condition {
		g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
		return garden.Status.Conditions
	}).WithPolling(2 * time.Second).Should(ContainCondition(OfType(operatorv1alpha1.GardenReconciled), WithStatus(gardencorev1beta1.ConditionTrue)))
}

func waitForGardenToBeDeleted(ctx context.Context, garden *operatorv1alpha1.Garden) {
	CEventually(ctx, func() error {
		return runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
	}).WithPolling(2 * time.Second).Should(BeNotFoundError())
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
