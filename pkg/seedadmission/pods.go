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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenlogger "github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	var patch []jsonpatch.JsonPatchOperation
	// determine whether to perform mutation
	isMutationRequired, err := mutationRequired(ctx, c, namespace)
	if err != nil {
		return nil, err
	}

	acc, err := meta.Accessor(pod)
	if err != nil {
		return nil, err
	}

	if !isMutationRequired {
		logger.Debugf("Skipping mutation for %s/%s due to policy check", acc.GetNamespace(), acc.GetName())
		return nil, nil
	}

	originalAnnotations := acc.GetAnnotations()
	annotationsToAdd := map[string]string{
		"fluentbit.io/exclude": "true",
	}

	for key, value := range annotationsToAdd {
		if originalAnnotations == nil || originalAnnotations[key] == "" {
			patch = append(patch, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, jsonpatch.JsonPatchOperation{
				Operation: "replace",
				Path:      "/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
				Value:     value,
			})
		}
	}
	return patch, nil
}

func mutationRequired(ctx context.Context, c client.Client, namespace string) (bool, error) {
	cluster, err := extensionscontroller.GetCluster(ctx, c, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if cluster.Shoot == nil {
		return false, nil
	}

	if cluster.Shoot.Spec.Hibernation != nil && cluster.Shoot.Spec.Hibernation.Enabled != nil && *cluster.Shoot.Spec.Hibernation.Enabled {
		return true, nil
	}

	if cluster.Shoot.Spec.Purpose != nil && *cluster.Shoot.Spec.Purpose == gardencorev1beta1.ShootPurposeTesting {
		return true, nil
	}

	return false, nil
}
