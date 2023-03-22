// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// healthCheckScheme is a dedicated scheme for CheckHealth containing the apiextensions types for converting
// CustomResourceDefinitions from v1beta1 and v1.
var healthCheckScheme *runtime.Scheme

func init() {
	healthCheckScheme = runtime.NewScheme()
	apiextensionsinstall.Install(healthCheckScheme)
}

// CheckHealth checks whether the given object is healthy.
// It returns a bool indicating whether the object was actually checked and an error if any health check failed.
func CheckHealth(obj client.Object) (bool, error) {
	if obj.GetAnnotations()[resourcesv1alpha1.SkipHealthCheck] == "true" {
		return false, nil
	}

	// Note: we can't do client-side conversions from one version to another, because conversion code is not exported
	// to k8s.io/api (apiextensions API is an exception). Hence, we only perform health checks for objects in well-known
	// and supported API versions (except CustomResourceDefinitions for backward-compatibility).
	// As we don't use unstructured objects in the health controller we don't need to convert to typed objects anymore and
	// can use the typed objects directly.

	// When adding new types for dedicated health checks here, make sure that they are registered in the scheme for the
	// target cluster client, see pkg/resourcemanager/cmd/target.go
	switch o := obj.(type) {
	case *apiextensionsv1.CustomResourceDefinition:
		return true, health.CheckCustomResourceDefinition(o)
	case *apiextensionsv1beta1.CustomResourceDefinition:
		// convert to v1 via internal version because converter cannot convert from external -> external version.
		crdInternal := &apiextensions.CustomResourceDefinition{}
		if err := healthCheckScheme.Convert(o, crdInternal, nil); err != nil {
			return false, err
		}

		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := healthCheckScheme.Convert(crdInternal, crd, nil); err != nil {
			return false, err
		}
		return true, health.CheckCustomResourceDefinition(crd)
	case *appsv1.DaemonSet:
		return true, health.CheckDaemonSet(o)
	case *appsv1.Deployment:
		return true, health.CheckDeployment(o)
	case *batchv1.Job:
		return true, health.CheckJob(o)
	case *corev1.Pod:
		return true, health.CheckPod(o)
	case *appsv1.ReplicaSet:
		return true, health.CheckReplicaSet(o)
	case *corev1.ReplicationController:
		return true, health.CheckReplicationController(o)
	case *corev1.Service:
		return true, health.CheckService(o)
	case *appsv1.StatefulSet:
		return true, health.CheckStatefulSet(o)
	}

	return false, nil
}

// FetchAdditionalFailureMessage fetches warning event messages for some objects as additional failure information.
func FetchAdditionalFailureMessage(ctx context.Context, c client.Client, obj client.Object) (string, error) {
	switch obj.(type) {
	case *corev1.Service:
		eventsMessage, err := kubernetes.FetchEventMessages(ctx, c.Scheme(), c, obj, corev1.EventTypeWarning, 5)
		if err != nil {
			return "", err
		}
		return eventsMessage, nil
	}
	return "", nil
}
