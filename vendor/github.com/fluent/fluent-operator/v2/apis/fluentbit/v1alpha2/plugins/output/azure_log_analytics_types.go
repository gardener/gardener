package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Azure Log Analytics is the Azure Log Analytics output plugin, allows you to ingest your records into Azure Log Analytics Workspace. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/azure**
type AzureLogAnalytics struct {
	// Customer ID or Workspace ID
	CustomerID *plugins.Secret `json:"customerID"`
	// Specify the primary or the secondary client authentication key
	SharedKey *plugins.Secret `json:"sharedKey"`
	// Name of the event type.
	LogType string `json:"logType,omitempty"`
	// Specify the name of the key where the timestamp is stored.
	TimeKey string `json:"timeKey,omitempty"`
	// If set, overrides the timeKey value with the `time-generated-field` HTTP header value.
	TimeGenerated *bool `json:"timeGenerated,omitempty"`
}

// Name implement Section() method
func (_ *AzureLogAnalytics) Name() string {
	return "azure"
}

// Params implement Section() method
func (o *AzureLogAnalytics) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if o.CustomerID != nil {
		u, err := sl.LoadSecret(*o.CustomerID)
		if err != nil {
			return nil, err
		}
		kvs.Insert("Customer_ID", u)
	}
	if o.SharedKey != nil {
		u, err := sl.LoadSecret(*o.SharedKey)
		if err != nil {
			return nil, err
		}
		kvs.Insert("Shared_Key", u)
	}
	if o.LogType != "" {
		kvs.Insert("Log_Type", o.LogType)
	}
	if o.TimeKey != "" {
		kvs.Insert("Time_Key", o.TimeKey)
	}
	if o.TimeGenerated != nil {
		kvs.Insert("Time_Generated", fmt.Sprint(*o.TimeGenerated))
	}
	return kvs, nil
}
