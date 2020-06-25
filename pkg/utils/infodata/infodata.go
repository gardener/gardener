/*
 * Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 *
 */

package infodata

import (
	"fmt"
	"sync"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"k8s.io/apimachinery/pkg/runtime"
)

var lock sync.Mutex
var types = map[TypeVersion]Unmarshaller{}

// Register is used to register new InfoData type versions
func Register(typeversion TypeVersion, unmarshaller Unmarshaller) {
	lock.Lock()
	defer lock.Unlock()
	types[typeversion] = unmarshaller
}

// GetUnmarshaller returns an Unmarshaller for the given typeName.
func GetUnmarshaller(typeName TypeVersion) Unmarshaller {
	lock.Lock()
	defer lock.Unlock()
	return types[typeName]
}

// Unmarshal unmarshals a GardenerResourceData into its respective Go struct representation
func Unmarshal(entry *gardencorev1alpha1.GardenerResourceData) (InfoData, error) {
	unmarshaller := GetUnmarshaller(TypeVersion(entry.Type))
	if unmarshaller == nil {
		return nil, fmt.Errorf("unknown info data type %q", entry.Type)
	}
	data, err := unmarshaller(entry.Data.Raw)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal data set %q of type %q: %s", entry.Name, entry.Type, err)
	}
	return data, nil
}

// GetInfoData retrieves the go representation of an object from the GardenerResourceDataList
func GetInfoData(resourceDataList gardencorev1alpha1helper.GardenerResourceDataList, name string) (InfoData, error) {
	resourceData := resourceDataList.Get(name)
	if resourceData == nil {
		return nil, nil
	}

	return Unmarshal(resourceData)
}

// UpsertInfoData updates or inserts an InfoData object into the GardenerResourceDataList
func UpsertInfoData(resourceDataList *gardencorev1alpha1helper.GardenerResourceDataList, name string, data InfoData) error {
	bytes, err := data.Marshal()
	if err != nil {
		return err
	}

	gardenerResourceData := &gardencorev1alpha1.GardenerResourceData{
		Name: name,
		Type: string(data.TypeVersion()),
		Data: runtime.RawExtension{Raw: bytes},
	}

	resourceDataList.Upsert(gardenerResourceData)
	return nil
}
