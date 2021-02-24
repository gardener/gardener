// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package v2

// DefaultComponent applies defaults to a component
func DefaultComponent(component *ComponentDescriptor) error {
	if component.Sources == nil {
		component.Sources = make([]Source, 0)
	}
	if component.ComponentReferences == nil {
		component.ComponentReferences = make([]ComponentReference, 0)
	}
	if component.Resources == nil {
		component.Resources = make([]Resource, 0)
	}

	DefaultResources(component)
	return nil
}

func DefaultList(list *ComponentDescriptorList) error {
	for i, comp := range list.Components {
		if len(comp.Metadata.Version) == 0 {
			list.Components[i].Metadata.Version = list.Metadata.Version
		}
	}
	return nil
}

// DefaultResources defaults a list of resources.
// The version of the component is defaulted for local resources that do not contain a version.
// adds the version as identity if the resource identity would clash otherwise.
func DefaultResources(component *ComponentDescriptor) {
	resourceIDs := map[string]struct{}{}
	for i, res := range component.Resources {
		if res.Relation == LocalRelation && len(res.Version) == 0 {
			component.Resources[i].Version = component.GetVersion()
		}

		id := string(res.GetIdentityDigest())
		if _, ok := resourceIDs[id]; ok {
			identity := res.ExtraIdentity
			identity[SystemIdentityVersion] = res.GetVersion()

			if id != string(identity.Digest()) {
				res.SetExtraIdentity(identity)
				id = string(res.GetIdentityDigest())
			}
		}
		resourceIDs[id] = struct{}{}
	}
}
