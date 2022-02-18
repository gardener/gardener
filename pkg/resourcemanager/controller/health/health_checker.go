// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"context"
	"fmt"

	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// healthCheckScheme is a dedicated healthCheckScheme for CheckHealth which contains all API types, that can be checked
// by CheckHealth. Needed for converting unstructured objects to structured objects.
var healthCheckScheme *runtime.Scheme

func init() {
	healthCheckScheme = runtime.NewScheme()
	utilruntime.Must(kubernetesscheme.AddToScheme(healthCheckScheme))
	apiextensionsinstall.Install(healthCheckScheme)
}

// CheckHealth checks whether the given object is healthy.
// It returns a bool indicating whether the object was actually checked and an error if any health check failed.
func CheckHealth(ctx context.Context, c client.Client, obj runtime.Object) (bool, error) {
	// We must not rely on TypeMeta to be set in objects as decoder clears apiVersion and kind fields, see
	// https://github.com/kubernetes/kubernetes/issues/80609 and https://github.com/gardener/gardener/issues/5357#issuecomment-1040150204.
	// Instead of using GetObjectKind(), we use a scheme to figure out the GroupVersionKind which works for both typed
	// and unstructured objects.
	gvk, err := apiutil.GVKForObject(obj, healthCheckScheme)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			// types that this function runs health checks on must be registered in healthCheckScheme
			// if the healthCheckScheme doesn't recognize it the object, skip the check as we don't have a health check for it
			return false, nil
		}
		return false, fmt.Errorf("failed to determine GVK of object: %w", err)
	}

	// Note: we can't do client-side conversions from one version to another, because conversion code is not exported
	// to k8s.io/api (apiextensions API is an exception). The only conversion we can do, is from unstructured objects to
	// typed objects in the same version. This is what most of the following healthCheckScheme.Convert calls do.
	switch gvk.GroupKind() {
	case apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition").GroupKind():
		crdObj := obj
		if gvk.Version == apiextensionsv1beta1.SchemeGroupVersion.Version {
			// Convert to internal version first if v1beta1 because converter cannot convert from external -> external version.
			crd := &apiextensions.CustomResourceDefinition{}
			if err := healthCheckScheme.Convert(crdObj, crd, nil); err != nil {
				return false, err
			}
			crdObj = crd
		}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := healthCheckScheme.Convert(crdObj, crd, nil); err != nil {
			return false, err
		}
		return true, health.CheckCustomResourceDefinition(crd)
	case appsv1.SchemeGroupVersion.WithKind("DaemonSet").GroupKind():
		ds := &appsv1.DaemonSet{}
		if err := healthCheckScheme.Convert(obj, ds, nil); err != nil {
			return false, err
		}
		return true, health.CheckDaemonSet(ds)
	case appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():
		deploy := &appsv1.Deployment{}
		if err := healthCheckScheme.Convert(obj, deploy, nil); err != nil {
			return false, err
		}
		return true, health.CheckDeployment(deploy)
	case batchv1.SchemeGroupVersion.WithKind("Job").GroupKind():
		job := &batchv1.Job{}
		if err := healthCheckScheme.Convert(obj, job, nil); err != nil {
			return false, err
		}
		return true, health.CheckJob(job)
	case corev1.SchemeGroupVersion.WithKind("Pod").GroupKind():
		pod := &corev1.Pod{}
		if err := healthCheckScheme.Convert(obj, pod, nil); err != nil {
			return false, err
		}
		return true, health.CheckPod(pod)
	case appsv1.SchemeGroupVersion.WithKind("ReplicaSet").GroupKind():
		rs := &appsv1.ReplicaSet{}
		if err := healthCheckScheme.Convert(obj, rs, nil); err != nil {
			return false, err
		}
		return true, health.CheckReplicaSet(rs)
	case corev1.SchemeGroupVersion.WithKind("ReplicationController").GroupKind():
		rc := &corev1.ReplicationController{}
		if err := healthCheckScheme.Convert(obj, rc, nil); err != nil {
			return false, err
		}
		return true, health.CheckReplicationController(rc)
	case corev1.SchemeGroupVersion.WithKind("Service").GroupKind():
		service := &corev1.Service{}
		if err := healthCheckScheme.Convert(obj, service, nil); err != nil {
			return false, err
		}
		return true, health.CheckService(ctx, healthCheckScheme, c, service)
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():
		statefulSet := &appsv1.StatefulSet{}
		if err := healthCheckScheme.Convert(obj, statefulSet, nil); err != nil {
			return false, err
		}
		return true, health.CheckStatefulSet(statefulSet)
	}

	return false, nil
}
