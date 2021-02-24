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

import (
	"errors"
)

// ComponentDescriptorList describes the v2 component descriptor containing
// components and their versions.
type ComponentDescriptorList struct {
	Metadata Metadata `json:"meta"`

	// Components contain all resolvable components with their dependencies
	Components []ComponentDescriptor `json:"components"`
}

// GetComponent return the component with a given name and version.
// It returns an error if no component with the name and version is defined.
func (c *ComponentDescriptorList) GetComponent(name, version string) (ComponentDescriptor, error) {
	for _, comp := range c.Components {
		if comp.GetName() == name && comp.GetVersion() == version {
			return comp, nil
		}
	}
	return ComponentDescriptor{}, errors.New("NotFound")
}

// GetComponent returns all components that match the given name.
func (c *ComponentDescriptorList) GetComponentByName(name string) []ComponentDescriptor {
	comps := make([]ComponentDescriptor, 0)
	for _, comp := range c.Components {
		if comp.GetName() == name {
			obj := comp
			comps = append(comps, obj)
		}
	}
	return comps
}
