// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailability

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DeployZeroDownTimeValidatorJob deploys a Job into the cluster which ensures
// zero downtime by continuously checking the kube-apiserver's health.
// This job fails once a health check fails. Its associated pod results in error status.
func DeployZeroDownTimeValidatorJob(ctx context.Context, c client.Client, testName, namespace, token string) (*batchv1.Job, error) {
	job := EmptyZeroDownTimeValidatorJob(testName, namespace)
	job.Spec = batchv1.JobSpec{
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
	}
	return job, c.Create(ctx, job)
}

// EmptyZeroDownTimeValidatorJob returns a Job object with only metadata set.
func EmptyZeroDownTimeValidatorJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "zero-down-time-validator-" + name, Namespace: namespace}}
}
