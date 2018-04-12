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

import "k8s.io/client-go/rest"

// MachineV1alpha1 creates a RESTClient request object for the given <verb> and the given <machineClassPlural>.
func (c *Client) MachineV1alpha1(verb, resource, namespace string) *rest.Request {
	return c.RESTClient().Verb(verb).Prefix("apis", "machine.sapcloud.io", "v1alpha1").Namespace(namespace).Resource(resource)
}
