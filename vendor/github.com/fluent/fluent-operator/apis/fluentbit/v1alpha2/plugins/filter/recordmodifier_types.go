package filter

import (
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Record Modifier Filter plugin allows to append fields or to exclude specific fields.
// RemoveKeys and WhitelistKeys are exclusive.
type RecordModifier struct {
	plugins.CommonParams `json:",inline"`
	// Append fields. This parameter needs key and value pair.
	Records []string `json:"records,omitempty"`
	// If the key is matched, that field is removed.
	RemoveKeys []string `json:"removeKeys,omitempty"`
	// If the key is not matched, that field is removed.
	WhitelistKeys []string `json:"whitelistKeys,omitempty"`
}

func (_ *RecordModifier) Name() string {
	return "record_modifier"
}

func (rm *RecordModifier) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := rm.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	for _, record := range rm.Records {
		kvs.Insert("Record", record)
	}
	for _, key := range rm.RemoveKeys {
		kvs.Insert("Remove_key", key)
	}
	for _, key := range rm.WhitelistKeys {
		kvs.Insert("Whitelist_key", key)
	}
	return kvs, nil
}
