// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package projectedtokenmount

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and configures volumes and mounts for projected ServiceAccount tokens in Pod
// resources.
type Handler struct {
	Logger            logr.Logger
	TargetReader      client.Reader
	ExpirationSeconds int64
}

// Default defaults the volumes and mounts for the projected ServiceAccount token of the provided pod.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected *corev1.Pod but got %T", obj)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	log := h.Logger.WithValues("pod", kubernetesutils.ObjectKeyForCreateWebhooks(pod, req))

	if len(pod.Spec.ServiceAccountName) == 0 || pod.Spec.ServiceAccountName == "default" {
		log.Info("Pod's service account name is empty or defaulted, nothing to be done", "serviceAccountName", pod.Spec.ServiceAccountName)
		return nil
	}

	serviceAccount := &corev1.ServiceAccount{}
	// We use `req.Namespace` instead of `pod.Namespace` due to https://github.com/kubernetes/kubernetes/issues/88282.
	if err := h.TargetReader.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: pod.Spec.ServiceAccountName}, serviceAccount); err != nil {
		log.Error(err, "Error getting service account", "serviceAccountName", pod.Spec.ServiceAccountName)
		return err
	}

	if serviceAccount.AutomountServiceAccountToken == nil || *serviceAccount.AutomountServiceAccountToken {
		log.Info("Pod's service account does not set .spec.automountServiceAccountToken=false, nothing to be done")
		return nil
	}

	if pod.Spec.AutomountServiceAccountToken != nil && !*pod.Spec.AutomountServiceAccountToken {
		log.Info("Pod explicitly disables auto-mount by setting .spec.automountServiceAccountToken to false, nothing to be done")
		return nil
	}

	for _, volume := range pod.Spec.Volumes {
		if strings.HasPrefix(volume.Name, serviceAccountVolumeNamePrefix) {
			log.Info("Pod already has a service account volume mount, nothing to be done")
			return nil
		}
	}

	expirationSeconds, err := tokenExpirationSeconds(pod.Annotations, h.ExpirationSeconds)
	if err != nil {
		log.Error(err, "Error getting the token expiration seconds")
		return err
	}

	log.Info("Pod meets requirements for auto-mounting the projected token")

	pod.Spec.Volumes = append(pod.Spec.Volumes, getVolume(expirationSeconds))
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, getVolumeMount())
	}
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].VolumeMounts = append(pod.Spec.InitContainers[i].VolumeMounts, getVolumeMount())
	}

	return nil
}

const (
	serviceAccountVolumeNamePrefix = "kube-api-access-"
	serviceAccountVolumeNameSuffix = "gardener"
)

func volumeName() string {
	return serviceAccountVolumeNamePrefix + serviceAccountVolumeNameSuffix
}

func tokenExpirationSeconds(annotations map[string]string, defaultExpirationSeconds int64) (int64, error) {
	if v, ok := annotations[resourcesv1alpha1.ProjectedTokenExpirationSeconds]; ok {
		return strconv.ParseInt(v, 10, 64)
	}
	return defaultExpirationSeconds, nil
}

func getVolume(expirationSeconds int64) corev1.Volume {
	return corev1.Volume{
		Name: volumeName(),
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				DefaultMode: ptr.To[int32](420),
				Sources: []corev1.VolumeProjection{
					{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							ExpirationSeconds: &expirationSeconds,
							Path:              "token",
						},
					},
					{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "kube-root-ca.crt",
							},
							Items: []corev1.KeyToPath{{
								Key:  "ca.crt",
								Path: "ca.crt",
							}},
						},
					},
					{
						DownwardAPI: &corev1.DownwardAPIProjection{
							Items: []corev1.DownwardAPIVolumeFile{{
								FieldRef: &corev1.ObjectFieldSelector{
									APIVersion: "v1",
									FieldPath:  "metadata.namespace",
								},
								Path: "namespace",
							}},
						},
					},
				},
			},
		},
	}
}

func getVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      volumeName(),
		MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
		ReadOnly:  true,
	}
}
