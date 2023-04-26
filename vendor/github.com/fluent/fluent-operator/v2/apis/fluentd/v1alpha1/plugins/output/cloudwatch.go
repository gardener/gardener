package output

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins"
)

// CloudWatch defines the parametes for out_cloudwatch output plugin
type CloudWatch struct {
	//
	AutoCreateStream *bool `json:"autoCreateStream,omitempty"`
	//
	AwsKeyId *plugins.Secret `json:"awsKeyId,omitempty"`
	//
	AwsSecKey *plugins.Secret `json:"awsSecKey,omitempty"`
	//
	AwsUseSts *bool `json:"awsUseSts,omitempty"`
	//
	AwsStsRoleARN *string `json:"awsStsRoleArn,omitempty"`
	//
	AwsStsSessionName *string `json:"awsStsSessionName,omitempty"`
	//
	AwsStsExternalId *string `json:"awsStsExternalId,omitempty"`
	//
	AwsStsPolicy *string `json:"awsStsPolicy,omitempty"`
	//
	AwsStsDurationSeconds *string `json:"awsStsDurationSeconds,omitempty"`
	//
	AwsStsEndpointUrl *string `json:"awsStsEndpointUrl,omitempty"`
	//
	AwsEcsAuthentication *bool `json:"awsEcsAuthentication,omitempty"`
	//
	Concurrency *int `json:"concurrency,omitempty"`
	// Specify an AWS endpoint to send data to.
	Endpoint *string `json:"endpoint,omitempty"`
	//
	SslVerifyPeer *bool `json:"sslVerifyPeer,omitempty"`
	//
	HttpProxy *string `json:"httpProxy,omitempty"`
	//
	IncludeTimeKey *bool `json:"includeTimeKey,omitempty"`
	//
	JsonHandler *string `json:"jsonHandler,omitempty"`
	//
	Localtime *bool `json:"localtime,omitempty"`
	//
	LogGroupAwsTags *string `json:"logGroupAwsTags,omitempty"`
	//
	LogGroupAwsTagsKey *string `json:"logGroupAwsTagsKey,omitempty"`
	//
	LogGroupName *string `json:"logGroupName,omitempty"`
	//
	LogGroupNameKey *string `json:"logGroupNameKey,omitempty"`
	//
	LogRejectedRequest *string `json:"logRejectedRequest,omitempty"`
	//
	LogStreamName *string `json:"logStreamName,omitempty"`
	//
	LogStreamNameKey *string `json:"logStreamNameKey,omitempty"`
	//
	MaxEventsPerBatch *string `json:"maxEventsPerBatch,omitempty"`
	//
	MaxMessageLength *string `json:"maxMessageLength,omitempty"`
	//
	MessageKeys *string `json:"messageKeys,omitempty"`
	//
	PutLogEventsDisableRetryLimit *bool `json:"putLogEventsDisableRetryLimit,omitempty"`
	//
	PutLogEventsRetryLimit *string `json:"putLogEventsRetryLimit,omitempty"`
	//
	PutLogEventsRetryWait *string `json:"putLogEventsRetryWait,omitempty"`
	// The AWS region.
	Region *string `json:"region,omitempty"`
	//
	RemoveLogGroupAwsTagsKey *bool `json:"removeLogGroupAwsTagsKey,omitempty"`
	//
	RemoveLogGroupNameKey *bool `json:"removeLogGroupNameKey,omitempty"`
	//
	RemoveLogStreamNameKey *bool `json:"removeLogStreamNameKey,omitempty"`
	//
	RemoveRetentionInDaysKey *bool `json:"removeRetentionInDaysKey,omitempty"`
	//
	RetentionInDays *string `json:"retentionInDays,omitempty"`
	//
	RetentionInDaysKey *string `json:"retentionInDaysKey,omitempty"`
	//
	UseTagAsGroup *string `json:"useTagAsGroup,omitempty"`
	//
	UseTagAsStream *string `json:"useTagAsStream,omitempty"`
	// ARN of an IAM role to assume (for cross account access).
	RoleARN *string `json:"roleArn,omitempty"`
	// Role Session name
	RoleSessionName *string `json:"roleSessionName,omitempty"`
	// Web identity token file
	WebIdentityTokenFile *string `json:"webIdentityTokenFile,omitempty"`
	//
	Policy *string `json:"policy,omitempty"`
	//
	DurationSeconds *string `json:"durationSeconds,omitempty"`
}
