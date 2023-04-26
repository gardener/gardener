package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Forward is the protocol used by Fluentd to route messages between peers. <br />
// The forward output plugin allows to provide interoperability between Fluent Bit and Fluentd. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/forward**
type Forward struct {
	// Target host where Fluent-Bit or Fluentd are listening for Forward messages.
	Host string `json:"host,omitempty"`
	// TCP Port of the target service.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// Set timestamps in integer format, it enable compatibility mode for Fluentd v0.12 series.
	TimeAsInteger *bool `json:"timeAsInteger,omitempty"`
	// Always send options (with "size"=count of messages)
	SendOptions *bool `json:"sendOptions,omitempty"`
	// Send "chunk"-option and wait for "ack" response from server.
	// Enables at-least-once and receiving server can control rate of traffic.
	// (Requires Fluentd v0.14.0+ server)
	RequireAckResponse *bool `json:"requireAckResponse,omitempty"`
	// A key string known by the remote Fluentd used for authorization.
	SharedKey string `json:"sharedKey,omitempty"`
	// Use this option to connect to Fluentd with a zero-length secret.
	EmptySharedKey *bool `json:"emptySharedKey,omitempty"`
	// Specify the username to present to a Fluentd server that enables user_auth.
	Username *plugins.Secret `json:"username,omitempty"`
	// Specify the password corresponding to the username.
	Password *plugins.Secret `json:"password,omitempty"`
	// Default value of the auto-generated certificate common name (CN).
	SelfHostname string `json:"selfHostname,omitempty"`
	*plugins.TLS `json:"tls,omitempty"`
}

func (_ *Forward) Name() string {
	return "forward"
}

// implement Section() method
func (f *Forward) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if f.Host != "" {
		kvs.Insert("Host", f.Host)
	}
	if f.Port != nil {
		kvs.Insert("Port", fmt.Sprint(*f.Port))
	}
	if f.TimeAsInteger != nil {
		kvs.Insert("Time_as_Integer", fmt.Sprint(*f.TimeAsInteger))
	}
	if f.SendOptions != nil {
		kvs.Insert("Send_options", fmt.Sprint(*f.SendOptions))
	}
	if f.RequireAckResponse != nil {
		kvs.Insert("Require_ack_response", fmt.Sprint(*f.RequireAckResponse))
	}
	if f.SharedKey != "" {
		kvs.Insert("Shared_Key", f.SharedKey)
	}
	if f.EmptySharedKey != nil {
		kvs.Insert("Empty_Shared_Key", fmt.Sprint(*f.EmptySharedKey))
	}
	if f.Username != nil {
		u, err := sl.LoadSecret(*f.Username)
		if err != nil {
			return nil, err
		}
		kvs.Insert("Username", u)
	}
	if f.Password != nil {
		pwd, err := sl.LoadSecret(*f.Password)
		if err != nil {
			return nil, err
		}
		kvs.Insert("Password", pwd)
	}
	if f.SelfHostname != "" {
		kvs.Insert("Self_Hostname", f.SelfHostname)
	}
	if f.TLS != nil {
		tls, err := f.TLS.Params(sl)
		if err != nil {
			return nil, err
		}
		kvs.Merge(tls)
	}
	return kvs, nil
}
