// Copyright 2018 The Gardener Authors.
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
	apiResourceList []*metav1.APIResourceList
	version         string
	Config          *rest.Config
	ClientConfig    clientcmd.ClientConfig
	Clientset       *kubernetes.Clientset
	GardenClientset *gardenclientset.Clientset
	RESTClient      rest.Interface
}
