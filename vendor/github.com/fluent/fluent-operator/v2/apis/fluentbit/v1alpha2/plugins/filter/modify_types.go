package filter

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Modify Filter plugin allows you to change records using rules and conditions. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/filters/modify**
type Modify struct {
	plugins.CommonParams `json:",inline"`
	// All conditions have to be true for the rules to be applied.
	Conditions []Condition `json:"conditions,omitempty"`
	// Rules are applied in the order they appear,
	// with each rule operating on the result of the previous rule.
	Rules []Rule `json:"rules,omitempty"`
}

// +kubebuilder:object:generate:=true

// The plugin supports the following conditions
type Condition struct {
	// Is true if KEY exists
	KeyExists string `json:"keyExists,omitempty"`
	// Is true if KEY does not exist
	KeyDoesNotExist map[string]string `json:"keyDoesNotExist,omitempty"`
	// Is true if a key matches regex KEY
	AKeyMatches string `json:"aKeyMatches,omitempty"`
	// Is true if no key matches regex KEY
	NoKeyMatches string `json:"noKeyMatches,omitempty"`
	// Is true if KEY exists and its value is VALUE
	KeyValueEquals map[string]string `json:"keyValueEquals,omitempty"`
	// Is true if KEY exists and its value is not VALUE
	KeyValueDoesNotEqual map[string]string `json:"keyValueDoesNotEqual,omitempty"`
	// Is true if key KEY exists and its value matches VALUE
	KeyValueMatches map[string]string `json:"keyValueMatches,omitempty"`
	// Is true if key KEY exists and its value does not match VALUE
	KeyValueDoesNotMatch map[string]string `json:"keyValueDoesNotMatch,omitempty"`
	// Is true if all keys matching KEY have values that match VALUE
	MatchingKeysHaveMatchingValues map[string]string `json:"matchingKeysHaveMatchingValues,omitempty"`
	// Is true if all keys matching KEY have values that do not match VALUE
	MatchingKeysDoNotHaveMatchingValues map[string]string `json:"matchingKeysDoNotHaveMatchingValues,omitempty"`
}

// +kubebuilder:object:generate:=true

// The plugin supports the following rules
type Rule struct {
	// Add a key/value pair with key KEY and value VALUE. If KEY already exists, this field is overwritten
	Set map[string]string `json:"set,omitempty"`
	// Add a key/value pair with key KEY and value VALUE if KEY does not exist
	Add map[string]string `json:"add,omitempty"`
	// Remove a key/value pair with key KEY if it exists
	Remove string `json:"remove,omitempty"`
	// Remove all key/value pairs with key matching wildcard KEY
	RemoveWildcard string `json:"removeWildcard,omitempty"`
	// Remove all key/value pairs with key matching regexp KEY
	RemoveRegex string `json:"removeRegex,omitempty"`
	// Rename a key/value pair with key KEY to RENAMED_KEY if KEY exists AND RENAMED_KEY does not exist
	Rename map[string]string `json:"rename,omitempty"`
	// Rename a key/value pair with key KEY to RENAMED_KEY if KEY exists.
	// If RENAMED_KEY already exists, this field is overwritten
	HardRename map[string]string `json:"hardRename,omitempty"`
	// Copy a key/value pair with key KEY to COPIED_KEY if KEY exists AND COPIED_KEY does not exist
	Copy map[string]string `json:"copy,omitempty"`
	// Copy a key/value pair with key KEY to COPIED_KEY if KEY exists.
	// If COPIED_KEY already exists, this field is overwritten
	HardCopy map[string]string `json:"hardCopy,omitempty"`
}

func (*Modify) Name() string {
	return "modify"
}

func (mo *Modify) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := mo.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	for _, c := range mo.Conditions {
		if c.KeyExists != "" {
			kvs.Insert("Condition", fmt.Sprintf("Key_exists    %s", c.KeyExists))
		}
		kvs.InsertStringMap(c.KeyDoesNotExist, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Key_does_not_exist    %s    %s", k, v)
		})
		if c.AKeyMatches != "" {
			kvs.Insert("Condition", fmt.Sprintf("A_key_matches    %s", c.AKeyMatches))
		}
		if c.NoKeyMatches != "" {
			kvs.Insert("Condition", fmt.Sprintf("No_key_matches    %s", c.NoKeyMatches))
		}
		kvs.InsertStringMap(c.KeyValueEquals, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Key_value_equals    %s    %s", k, v)
		})
		kvs.InsertStringMap(c.KeyValueDoesNotEqual, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Key_value_does_not_equal    %s    %s", k, v)
		})
		kvs.InsertStringMap(c.KeyValueMatches, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Key_value_matches    %s    %s", k, v)
		})
		kvs.InsertStringMap(c.KeyValueDoesNotMatch, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Key_value_does_not_match    %s    %s", k, v)
		})
		kvs.InsertStringMap(c.MatchingKeysHaveMatchingValues, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Matching_keys_have_matching_values    %s    %s", k, v)
		})
		kvs.InsertStringMap(c.MatchingKeysDoNotHaveMatchingValues, func(k, v string) (string, string) {
			return "Condition", fmt.Sprintf("Matching_keys_do_not_have_matching_values    %s    %s", k, v)
		})
	}
	for _, r := range mo.Rules {
		kvs.InsertStringMap(r.Set, func(k, v string) (string, string) {
			return "Set", fmt.Sprintf("%s    %s", k, v)
		})
		kvs.InsertStringMap(r.Add, func(k, v string) (string, string) {
			return "Add", fmt.Sprintf("%s    %s", k, v)
		})
		if r.Remove != "" {
			kvs.Insert("Remove", r.Remove)
		}
		if r.RemoveWildcard != "" {
			kvs.Insert("Remove_wildcard", r.RemoveWildcard)
		}
		if r.RemoveRegex != "" {
			kvs.Insert("Remove_regex", r.RemoveRegex)
		}
		kvs.InsertStringMap(r.Rename, func(k, v string) (string, string) {
			return "Rename", fmt.Sprintf("%s    %s", k, v)
		})
		kvs.InsertStringMap(r.HardRename, func(k, v string) (string, string) {
			return "Hard_rename", fmt.Sprintf("%s    %s", k, v)
		})
		kvs.InsertStringMap(r.Copy, func(k, v string) (string, string) {
			return "Copy", fmt.Sprintf("%s    %s", k, v)
		})
		kvs.InsertStringMap(r.HardCopy, func(k, v string) (string, string) {
			return "Hard_copy", fmt.Sprintf("%s    %s", k, v)
		})
	}
	return kvs, nil
}
