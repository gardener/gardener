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

// TypeVersion is the potentially versioned type name of an InfoData representation.
type TypeVersion string

// Unmarshaller is a factory to create a dedicated InfoData object from a byte stream
type Unmarshaller func(data []byte) (InfoData, error)

// InfoData is an interface which allows
type InfoData interface {
	TypeVersion() TypeVersion
	Marshal() ([]byte, error)
}

// Loader is an interface which declares methods that can be used to extract InfoData from Kubernetes resources data.
// TODO: This interface can be removed in a later version after all resources have been synced to the ShootState.
type Loader interface {
	LoadFromSecretData(map[string][]byte) (InfoData, error)
}
