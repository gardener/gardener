/*
SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
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
	if _, ok := data.(*emptyInfoData); ok {
		return nil
	}

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
