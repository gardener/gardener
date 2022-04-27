// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package projectedtokenmount

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/rootcapublisher"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type handler struct {
	logger            logr.Logger
	targetClient      client.Reader
	decoder           *admission.Decoder
	expirationSeconds int64
}

// NewHandler returns a new handler.
func NewHandler(logger logr.Logger, targetClient client.Reader, expirationSeconds int64) admission.Handler {
	return &handler{
		logger:            logger,
		targetClient:      targetClient,
		expirationSeconds: expirationSeconds,
	}
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("only 'create' operation is handled")
	}

	pod := &corev1.Pod{}
	if err := h.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	log := h.logger.WithValues("pod", kutil.ObjectKeyForCreateWebhooks(pod))

	if len(pod.Spec.ServiceAccountName) == 0 || pod.Spec.ServiceAccountName == "default" {
		log.Info("Pod's service account name is empty or defaulted, nothing to be done", "serviceAccountName", pod.Spec.ServiceAccountName)
		return admission.Allowed("service account not specified or defaulted")
	}

	serviceAccount := &corev1.ServiceAccount{}
	// We use `req.Namespace` instead of `pod.Namespace` due to https://github.com/kubernetes/kubernetes/issues/88282.
	if err := h.targetClient.Get(ctx, kutil.Key(req.Namespace, pod.Spec.ServiceAccountName), serviceAccount); err != nil {
		log.Error(err, "Error getting service account", "serviceAccountName", pod.Spec.ServiceAccountName)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if serviceAccount.AutomountServiceAccountToken == nil || *serviceAccount.AutomountServiceAccountToken {
		log.Info("Pod's service account does not set .spec.automountServiceAccountToken=false, nothing to be done")
		return admission.Allowed("auto-mounting service account token is not disabled on ServiceAccount level")
	}

	if pod.Spec.AutomountServiceAccountToken != nil && !*pod.Spec.AutomountServiceAccountToken {
		log.Info("Pod explicitly disables auto-mount by setting .spec.automountServiceAccountToken to false, nothing to be done")
		return admission.Allowed("Pod explicitly disables a service account token mount")
	}

	for _, volume := range pod.Spec.Volumes {
		if strings.HasPrefix(volume.Name, serviceAccountVolumeNamePrefix) {
			log.Info("Pod already has a service account volume mount, nothing to be done")
			return admission.Allowed("pod already has service account volume mount")
		}
	}

	expirationSeconds, err := tokenExpirationSeconds(pod.Annotations, h.expirationSeconds)
	if err != nil {
		log.Error(err, "Error getting the token expiration seconds")
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, getVolume(expirationSeconds))
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, getVolumeMount())
	}
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].VolumeMounts = append(pod.Spec.InitContainers[i].VolumeMounts, getVolumeMount())
	}

	// Workaround https://github.com/kubernetes/kubernetes/issues/82573 - this got fixed with
	// https://github.com/kubernetes/kubernetes/pull/89193 starting with Kubernetes 1.19, however, we don't know to
	// which node the newly created Pod gets scheduled (it could be that the API server is already running on 1.19 while
	// the kubelets are still on 1.18). Hence, let's just unconditionally add this and remove this coding again once we
	// drop support for seed and shoot clusters < 1.19.
	{
		if pod.Spec.SecurityContext == nil {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}

		if pod.Spec.SecurityContext.FSGroup == nil {
			pod.Spec.SecurityContext.FSGroup = pointer.Int64(65534)
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("Pod meets requirements for auto-mounting the projected token")
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
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
				DefaultMode: pointer.Int32(420),
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
								Name: rootcapublisher.RootCACertConfigMapName,
							},
							Items: []corev1.KeyToPath{{
								Key:  rootcapublisher.RootCADataKey,
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
