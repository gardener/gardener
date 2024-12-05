package helm

import (
	"bytes"
)

var nullLiteral = []byte(`null`)

// +k8s:deepcopy-gen=true

// Values represents any valid Helm values. Must be of type map[string]interface{}.
type Values struct {
	Raw []byte `json:"-" protobuf:"bytes,1,opt,name=raw"`
}

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (_ Values) OpenAPISchemaType() []string {
	return []string{"object"}
}

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (_ Values) OpenAPISchemaFormat() string { return "" }

// MarshalJSON implements json.Marshaler.
func (v Values) MarshalJSON() ([]byte, error) {
	if len(v.Raw) > 0 {
		return v.Raw, nil
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (v *Values) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && !bytes.Equal(data, nullLiteral) {
		v.Raw = append(v.Raw[0:0], data...)
	}
	return nil
}
