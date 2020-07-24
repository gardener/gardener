// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedadmission

import (
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenlogger "github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MutatePod mutates pod if mutation conditions are met
func MutatePod(ctx context.Context, c client.Client, logger *logrus.Logger, request *admissionv1beta1.AdmissionRequest) ([]jsonpatch.JsonPatchOperation, error) {
	// Ignore all resources other than our expected ones
	switch request.Resource {
	case
		metav1.GroupVersionResource{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: "pods"}:
	default:
		return nil, nil
	}

	obj, err := getRequestObject(ctx, c, request)
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	entryLogger := gardenlogger.
		NewFieldLogger(logger, "resource", fmt.Sprintf("%s/%s/%s", request.Kind.Group, request.Kind.Version, request.Kind.Kind)).
		WithField("operation", request.Operation).
		WithField("namespace", request.Namespace)

	entryLogger.Info("Handling request")
	patch, err := mutateShootControlPlanePodAnnotations(ctx, c, entryLogger, obj, request.Namespace)
	if err != nil {
		logger.Errorf("Error mutating pod %s/%s: %v", request.Name, request.Namespace, err)
		return nil, err
	}

	return patch, nil
}

// mutateShootControlPlanePodAnnotations checks if the object mutation is allowed and return the needed patches .
func mutateShootControlPlanePodAnnotations(ctx context.Context, c client.Client, logger *logrus.Entry, pod runtime.Object, namespace string) ([]jsonpatch.JsonPatchOperation, error) {
	desiredAnnotations := map[string]string{"fluentbit.io/exclude": "true"}

	acc, err := meta.Accessor(pod)
	if err != nil {
		return nil, err
	}

	shoot, err := extractShoot(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	if shoot == nil {
		logger.Debugf("Skipping mutation for %s/%s because shoot could not be extracted", acc.GetNamespace(), acc.GetName())
		return nil, nil
	}

	if mustAddOrReplaceAnnotations(shoot) {
		return computeAddOrReplacePatch(acc.GetAnnotations(), desiredAnnotations), nil
	}

	if mustRemoveAnnotations(acc.GetAnnotations(), desiredAnnotations) {
		return computeRemovePatch(acc.GetAnnotations(), desiredAnnotations), nil
	}

	logger.Debugf("Skipping mutation for %s/%s due to policy check", acc.GetNamespace(), acc.GetName())
	return nil, nil
}

func extractShoot(ctx context.Context, c client.Client, namespace string) (*gardencorev1beta1.Shoot, error) {
	cluster, err := extensions.GetCluster(ctx, c, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return cluster.Shoot, nil
}

func mustAddOrReplaceAnnotations(shoot *gardencorev1beta1.Shoot) bool {
	if shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled {
		return true
	}

	if shoot.Spec.Purpose != nil && *shoot.Spec.Purpose == gardencorev1beta1.ShootPurposeTesting {
		return true
	}

	return false
}

func computeAddOrReplacePatch(existingAnnotations, desiredAnnotations map[string]string) []jsonpatch.JsonPatchOperation {
	if len(existingAnnotations) == 0 {
		return []jsonpatch.JsonPatchOperation{
			{
				Operation: "add",
				Path:      "/metadata/annotations",
				Value:     desiredAnnotations,
			},
		}
	}

	var patch []jsonpatch.JsonPatchOperation

	for key, value := range desiredAnnotations {
		operation := "add"
		if _, ok := existingAnnotations[key]; ok {
			operation = "replace"
		}

		patch = append(patch, jsonpatch.JsonPatchOperation{
			Operation: operation,
			Path:      "/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
			Value:     value,
		})
	}

	return patch
}

func mustRemoveAnnotations(existingAnnotations, desiredAnnotations map[string]string) bool {
	for key, value := range desiredAnnotations {
		if val, ok := existingAnnotations[key]; ok && val == value {
			return true
		}
	}

	return false
}

func computeRemovePatch(existingAnnotations, desiredAnnotations map[string]string) []jsonpatch.JsonPatchOperation {
	var patch []jsonpatch.JsonPatchOperation

	for key := range desiredAnnotations {
		if _, ok := existingAnnotations[key]; !ok {
			continue
		}

		patch = append(patch, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      "/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
		})
	}

	return patch
}
