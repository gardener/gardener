/*
SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
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

type emptyInfoData struct{}

func (*emptyInfoData) Marshal() ([]byte, error) {
	return nil, nil
}

func (*emptyInfoData) TypeVersion() TypeVersion {
	return ""
}

// EmptyInfoData is an infodata which does not contain any information.
var EmptyInfoData = &emptyInfoData{}
