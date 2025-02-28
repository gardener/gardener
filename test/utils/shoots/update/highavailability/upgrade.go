// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailability

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/test/framework"
)

// UpgradeAndVerify runs the HA control-plane upgrade tests for an existing shoot cluster.
func UpgradeAndVerify(ctx context.Context, f *framework.ShootFramework, failureToleranceType gardencorev1beta1.FailureToleranceType) {
	verifyEnvoyFilterInIstioNamespace(ctx, f.SeedClient, f.ShootSeedNamespace(), false)
	By("Update Shoot control plane to HA with failure tolerance type " + string(failureToleranceType))
	Expect(f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
			HighAvailability: &gardencorev1beta1.HighAvailability{
				FailureTolerance: gardencorev1beta1.FailureTolerance{
					Type: failureToleranceType,
				},
			},
		}
		return nil
	})).To(Succeed())

	By("Verify Shoot's control plane components")
	verifyTopologySpreadConstraint(ctx, f.SeedClient, f.Shoot, f.ShootSeedNamespace())
	verifyEtcdAffinity(ctx, f.SeedClient, f.Shoot, f.ShootSeedNamespace())
	verifyEnvoyFilterInIstioNamespace(ctx, f.SeedClient, f.ShootSeedNamespace(), true)
}

func verifyTopologySpreadConstraint(ctx context.Context, seedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, namespace string) {
	for _, name := range []string{
		v1beta1constants.DeploymentNameGardenerResourceManager,
	} {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		Expect(seedClient.Client().Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed(), "trying to get deployment obj: "+deployment.Name+", but not succeeded.")

		Expect(deployment.Spec.Template.Spec.TopologySpreadConstraints).To(getTSCMatcherForFailureToleranceType(shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type), "for component "+deployment.Name)
	}
}

func getTSCMatcherForFailureToleranceType(failureToleranceType gardencorev1beta1.FailureToleranceType) gomegatypes.GomegaMatcher {
	var (
		nodeSpread = MatchFields(IgnoreExtras, Fields{
			"MaxSkew":           Equal(int32(1)),
			"TopologyKey":       Equal(corev1.LabelHostname),
			"WhenUnsatisfiable": Equal(corev1.DoNotSchedule),
		})
		zoneSpread = MatchFields(IgnoreExtras, Fields{
			"MaxSkew":           Equal(int32(1)),
			"TopologyKey":       Equal(corev1.LabelTopologyZone),
			"WhenUnsatisfiable": Equal(corev1.DoNotSchedule),
		})
	)

	switch failureToleranceType {
	case gardencorev1beta1.FailureToleranceTypeNode:
		return ConsistOf(nodeSpread)
	case gardencorev1beta1.FailureToleranceTypeZone:
		return ConsistOf(nodeSpread, zoneSpread)
	default:
		return BeNil()
	}
}

func verifyEtcdAffinity(ctx context.Context, seedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, namespace string) {
	for _, componentName := range []string{
		v1beta1constants.ETCDRoleEvents,
		v1beta1constants.ETCDRoleMain,
	} {
		numberOfZones := 1
		if v1beta1helper.IsMultiZonalShootControlPlane(shoot) {
			numberOfZones = 3
		}

		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-" + componentName,
			Namespace: namespace,
		}}
		Expect(seedClient.Client().Get(ctx, client.ObjectKeyFromObject(sts), sts)).To(Succeed(), "get StatefulSet "+sts.Name)

		Expect(sts.Spec.Template.Spec.Affinity).NotTo(BeNil(), "for component "+sts.Name)
		Expect(sts.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil(), "for component "+sts.Name)
		Expect(sts.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).NotTo(BeNil(), "for component "+sts.Name)
		Expect(sts.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(HaveLen(1), "for component "+sts.Name)
		Expect(sts.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions).To(HaveLen(1), "for component "+sts.Name)
		Expect(sts.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0]).To(MatchFields(IgnoreExtras, Fields{
			"Key":      Equal(corev1.LabelTopologyZone),
			"Operator": Equal(corev1.NodeSelectorOpIn),
			"Values":   HaveLen(numberOfZones),
		}), "for component "+sts.Name)
		Expect(sts.Spec.Template.Spec.Affinity.PodAntiAffinity).To(BeNil(), "for component "+sts.Name)
	}
}

// DeployZeroDownTimeValidatorJob deploys a Job into the cluster which ensures
// zero downtime by continuously checking the kube-apiserver's health.
// This job fails once a health check fails. Its associated pod results in error status.
func DeployZeroDownTimeValidatorJob(ctx context.Context, c client.Client, testName, namespace, token string) (*batchv1.Job, error) {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zero-down-time-validator-" + testName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "validator",
							Image: "quay.io/curl/curl",
							Command: []string{"/bin/sh", "-ec",
								// To avoid flakiness, consider downtime when curl fails consecutively back-to-back three times.
								"failed=0; threshold=3; " +
									"while [ $failed -lt $threshold ]; do " +
									"if curl -m 2 -k https://kube-apiserver/healthz -H 'Authorization: " + token + "' -s -f -o /dev/null ; then " +
									"echo $(date +'%Y-%m-%dT%H:%M:%S.%3N%z') INFO: kube-apiserver is healthy.; failed=0; " +
									"else failed=$((failed+1)); " +
									"echo $(date +'%Y-%m-%dT%H:%M:%S.%3N%z') ERROR: kube-apiserver is unhealthy and retrying.; " +
									"fi; " +
									"sleep 10; " +
									"done; " +
									"echo $(date +'%Y-%m-%dT%H:%M:%S.%3N%z') ERROR: kube-apiserver is still unhealthy after $failed attempts. Considered as downtime.; " +
									"exit 1; "},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			BackoffLimit: ptr.To[int32](0),
		},
	}
	return &job, c.Create(ctx, &job)
}

func verifyEnvoyFilterInIstioNamespace(ctx context.Context, seedClient kubernetes.Interface, shootName string, checkLabels bool) {
	var filteredList []*istionetworkingv1alpha3.EnvoyFilter
	// TODO(oliver-goetz): Remove `shootName` from the set when Gardener v1.113 is released.
	shootFilterNames := sets.New(shootName, shootName+"-apiserver-proxy", shootName+"-istio-tls-termination")
	CEventually(ctx, func(g Gomega) {
		envoyFilters := &istionetworkingv1alpha3.EnvoyFilterList{}
		g.Expect(seedClient.Client().List(ctx, envoyFilters)).To(Succeed(), "trying to list envoy filters, but did not succeed.")
		filteredList = []*istionetworkingv1alpha3.EnvoyFilter{}
		for _, filter := range envoyFilters.Items {
			if !shootFilterNames.Has(filter.Name) {
				continue
			}
			// Old Gardener releases do not manage the envoy filter and hence we may end up with one managed and one unmanaged filter
			if checkLabels && filter.Labels["resources.gardener.cloud/managed-by"] != "gardener" {
				continue
			}
			filteredList = append(filteredList, filter)
		}
		// We expect 2 envoy filters when Istio TLS termination is active and 1 if it is not.
		// It is considered to be active when `kube-apiserver-mtls` service exists in the shoot namespace.
		expectedEnvoyFilters := 2
		err := seedClient.Client().Get(ctx, client.ObjectKey{Name: "kube-apiserver-mtls", Namespace: shootName}, &corev1.Service{})
		if errors.IsNotFound(err) {
			expectedEnvoyFilters = 1
		} else {
			g.Expect(err).To(Not(HaveOccurred()))
		}
		g.Expect(filteredList).To(HaveLen(expectedEnvoyFilters))
	}).Should(Succeed())
	Expect(filteredList[0].Namespace).To(HavePrefix("istio-ingress"))
}
