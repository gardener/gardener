// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Marshal transform RawState to []byte representation. It encodes the raw state data
func (trs *RawState) Marshal() ([]byte, error) {
	return json.Marshal(trs.encodeBase64())
}

// GetRawState returns the content of terraform state config map
func (t *terraformer) GetRawState(ctx context.Context) (*RawState, error) {
	configMap := &corev1.ConfigMap{}
	if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: t.stateName}, configMap); err != nil {
		return nil, err
	}
	return &RawState{
		Data:     configMap.Data[StateKey],
		Encoding: NoneEncoding,
	}, nil
}

// UnmarshalRawState transform passed rawState to RawState struct. It tries to decode the state
func UnmarshalRawState(rawState any) (*RawState, error) {
	var rawData []byte

	switch v := rawState.(type) {
	case *runtime.RawExtension:
		if v == nil {
			rawData = nil
		} else {
			rawData = v.Raw
		}
	case []byte:
		rawData = v
	case string:
		rawData = []byte(v)
	case nil:
		rawData = nil
	default:
		return nil, fmt.Errorf("unsupported type '%T' for unmarshalling Raw Terraform State", rawState)
	}

	terraformStateObj, err := buildRawState(rawData)
	if err != nil {
		return nil, err
	}
	return terraformStateObj.decode()
}

// buildRawState returns RawState from byte slice
func buildRawState(terraformRawState []byte) (*RawState, error) {
	trs := &RawState{
		Data:     "",
		Encoding: NoneEncoding,
	}

	if terraformRawState == nil {
		return trs, nil
	}

	if err := json.Unmarshal(terraformRawState, trs); err != nil {
		return nil, err
	}
	return trs, nil
}

// encodeBase64 encode the RawState.Data if it is not already base64 encoded
func (trs *RawState) encodeBase64() *RawState {
	if trs.Encoding != Base64Encoding {
		trs.Data = base64.StdEncoding.EncodeToString([]byte(trs.Data))
		trs.Encoding = Base64Encoding
	}
	return trs
}

// decode tries to decode RawState.Data
func (trs *RawState) decode() (*RawState, error) {
	switch trs.Encoding {
	case Base64Encoding:
		trsDec, err := base64.StdEncoding.DecodeString(trs.Data)
		if err != nil {
			return nil, err
		}
		trs.Data = string(trsDec)
		trs.Encoding = NoneEncoding
	case NoneEncoding:
		// do nothing
	default:
		return nil, fmt.Errorf("unrecognised encoding %q for RawState.Data", trs.Encoding)
	}

	return trs, nil
}
