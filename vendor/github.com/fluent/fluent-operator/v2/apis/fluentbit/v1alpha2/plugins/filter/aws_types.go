package filter

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The AWS Filter Enriches logs with AWS Metadata. Currently the plugin adds the EC2 instance ID and availability zone to log records. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/filters/aws-metadata**
type AWS struct {
	plugins.CommonParams `json:",inline"`
	// Specify which version of the instance metadata service to use. Valid values are 'v1' or 'v2'.
	// +kubebuilder:validation:Enum:=v1;v2
	ImdsVersion string `json:"imdsVersion,omitempty"`
	// The availability zone; for example, "us-east-1a". Default is true.
	AZ *bool `json:"az,omitempty"`
	//The EC2 instance ID.Default is true.
	EC2InstanceID *bool `json:"ec2InstanceID,omitempty"`
	//The EC2 instance type.Default is false.
	EC2InstanceType *bool `json:"ec2InstanceType,omitempty"`
	//The EC2 instance private ip.Default is false.
	PrivateIP *bool `json:"privateIP,omitempty"`
	//The EC2 instance image id.Default is false.
	AmiID *bool `json:"amiID,omitempty"`
	//The account ID for current EC2 instance.Default is false.
	AccountID *bool `json:"accountID,omitempty"`
	//The hostname for current EC2 instance.Default is false.
	HostName *bool `json:"hostName,omitempty"`
	//The VPC ID for current EC2 instance.Default is false.
	VpcID *bool `json:"vpcID,omitempty"`
}

func (_ *AWS) Name() string {
	return "aws"
}

func (a *AWS) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := a.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	if a.ImdsVersion != "" {
		kvs.Insert("imds_version", a.ImdsVersion)
	}
	if a.AZ != nil {
		kvs.Insert("az", fmt.Sprint(*a.AZ))
	}
	if a.EC2InstanceID != nil {
		kvs.Insert("ec2_instance_id", fmt.Sprint(*a.EC2InstanceID))
	}
	if a.EC2InstanceType != nil {
		kvs.Insert("ec2_instance_type", fmt.Sprint(*a.EC2InstanceType))
	}
	if a.PrivateIP != nil {
		kvs.Insert("private_ip", fmt.Sprint(*a.PrivateIP))
	}
	if a.AmiID != nil {
		kvs.Insert("ami_id", fmt.Sprint(*a.AmiID))
	}
	if a.AccountID != nil {
		kvs.Insert("account_id", fmt.Sprint(*a.AccountID))
	}
	if a.HostName != nil {
		kvs.Insert("hostname", fmt.Sprint(*a.HostName))
	}
	if a.VpcID != nil {
		kvs.Insert("vpc_id", fmt.Sprint(*a.VpcID))
	}
	return kvs, nil
}
