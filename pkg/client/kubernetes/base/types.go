// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetesbase

import (
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	machineclientset "github.com/gardener/gardener/pkg/client/machine/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	propagationPolicy    = metav1.DeletePropagationForeground
	gracePeriodSeconds   = int64(60)
	defaultDeleteOptions = metav1.DeleteOptions{
		PropagationPolicy:  &propagationPolicy,
		GracePeriodSeconds: &gracePeriodSeconds,
	}
)

// Client is a struct containing the configuration for the respective Kubernetes
// cluster, the collection of Kubernetes clients <Clientset> containing all REST clients
// for the built-in Kubernetes API groups, and the GardenClientset which is a REST clientset
// for the Garden API group.
// The RESTClient itself is a normal HTTP client for the respective Kubernetes cluster,
// allowing requests to arbitrary URLs.
// The version string contains only the major/minor part in the form <major>.<minor>.
type Client struct {
	apiResourceList   []*metav1.APIResourceList
	config            *rest.Config
	clientConfig      clientcmd.ClientConfig
	clientset         *kubernetes.Clientset
	gardenClientset   *gardenclientset.Clientset
	machineClientset  *machineclientset.Clientset
	restClient        rest.Interface
	resourceAPIGroups map[string][]string
	version           string
}

const (
	// CronJobs is a constant for a Kubernetes resource with the same name.
	CronJobs = "cronjobs"

	// CustomResourceDefinitions is a constant for a Kubernetes resource with the same name.
	CustomResourceDefinitions = "customresourcedefinitions"

	// DaemonSets is a constant for a Kubernetes resource with the same name.
	DaemonSets = "daemonsets"

	// Deployments is a constant for a Kubernetes resource with the same name.
	Deployments = "deployments"

	// Ingresses is a constant for a Kubernetes resource with the same name.
	Ingresses = "ingresses"

	// Jobs is a constant for a Kubernetes resource with the same name.
	Jobs = "jobs"

	// Namespaces is a constant for a Kubernetes resource with the same name.
	Namespaces = "namespaces"

	// PersistentVolumeClaims is a constant for a Kubernetes resource with the same name.
	PersistentVolumeClaims = "persistentvolumeclaims"

	// Pods is a constant for a Kubernetes resource with the same name.
	Pods = "pods"

	// ReplicaSets is a constant for a Kubernetes resource with the same name.
	ReplicaSets = "replicasets"

	// ReplicationControllers is a constant for a Kubernetes resource with the same name.
	ReplicationControllers = "replicationcontrollers"

	// Services is a constant for a Kubernetes resource with the same name.
	Services = "services"

	// StatefulSets is a constant for a Kubernetes resource with the same name.
	StatefulSets = "statefulsets"
)
