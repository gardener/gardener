package filter

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Record Modifier Filter plugin allows to append fields or to exclude specific fields. <br />
// RemoveKeys and WhitelistKeys are exclusive. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/filters/record-modifier**
type RecordModifier struct {
	plugins.CommonParams `json:",inline"`
	// Append fields. This parameter needs key and value pair.
	Records []string `json:"records,omitempty"`
	// If the key is matched, that field is removed.
	RemoveKeys []string `json:"removeKeys,omitempty"`
	// If the key is not matched, that field is removed.
	AllowlistKeys []string `json:"allowlistKeys,omitempty"`
	// An alias of allowlistKeys for backwards compatibility.
	WhitelistKeys []string `json:"whitelistKeys,omitempty"`
	// If set, the plugin appends uuid to each record. The value assigned becomes the key in the map.
	UUIDKeys []string `json:"uuidKeys,omitempty"`
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
	for _, key := range rm.AllowlistKeys {
		kvs.Insert("Allowlist_key", key)
	}
	for _, key := range rm.WhitelistKeys {
		kvs.Insert("Whitelist_key", key)
	}
	for _, key := range rm.UUIDKeys {
		kvs.Insert("Uuid_key", key)
	}
	return kvs, nil
}
