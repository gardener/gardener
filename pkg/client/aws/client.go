// Copyright 2018 The Gardener Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/sts"
)

// NewClient creates a new Client for the given AWS credentials <accessKeyID>, <secretAccessKey>, and
// the AWS region <region>.
// It initializes the clients for the various services like EC2, ELB, etc.
func NewClient(accessKeyID, secretAccessKey, region string) ClientInterface {
	awsConfig := &aws.Config{
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
	}
	sess := session.Must(session.NewSession(awsConfig))
	config := &aws.Config{Region: aws.String(region)}
	return &Client{
		AutoScaling: autoscaling.New(sess, config),
		EC2:         ec2.New(sess, config),
		ELB:         elb.New(sess, config),
		STS:         sts.New(sess, config),
	}
}

// GetAccountID returns the ID of the AWS account the Client is interacting with.
func (c *Client) GetAccountID() (string, error) {
	getCallerIdentityInput := &sts.GetCallerIdentityInput{}
	getCallerIdentityOutput, err := c.
		STS.
		GetCallerIdentity(getCallerIdentityInput)
	if err != nil {
		return "", err
	}
	return *getCallerIdentityOutput.Account, nil
}

// CheckIfVPCExists returns true if the VPC exists, and false otherwise.
func (c *Client) CheckIfVPCExists(vpcID string) (bool, error) {
	describeVpcsInput := &ec2.DescribeVpcsInput{
		VpcIds: []*string{
			aws.String(vpcID),
		},
	}

	_, err := c.
		EC2.
		DescribeVpcs(describeVpcsInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidVpcID.NotFound":
				return false, nil
			default:
				return false, aerr
			}
		}
		return false, err
	}
	return true, nil
}

// GetInternetGateway returns the ID of the internet gateway attached to the given VPC <vpcID>.
// If there is no internet gateway attached, the returned string will be empty.
func (c *Client) GetInternetGateway(vpcID string) (string, error) {
	describeInternetGatewaysInput := &ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("attachment.vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
		},
	}
	describeInternetGatewaysOutput, err := c.
		EC2.
		DescribeInternetGateways(describeInternetGatewaysInput)
	if err != nil {
		return "", err
	}

	if describeInternetGatewaysOutput.InternetGateways != nil {
		return *describeInternetGatewaysOutput.InternetGateways[0].InternetGatewayId, nil
	}
	return "", nil
}

// GetELB returns the AWS LoadBalancer object for the given name <loadBalancerName>.
func (c *Client) GetELB(loadBalancerName string) (*elb.DescribeLoadBalancersOutput, error) {
	describeLoadBalancersInput := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			aws.String(loadBalancerName),
		},
		PageSize: aws.Int64(1),
	}
	return c.
		ELB.
		DescribeLoadBalancers(describeLoadBalancersInput)
}

// UpdateELBHealthCheck updates the AWS LoadBalancer health check target protocol to SSL for a given
// LoadBalancer <loadBalancerName>.
func (c *Client) UpdateELBHealthCheck(loadBalancerName string, targetPort string) error {
	configureHealthCheckInput := &elb.ConfigureHealthCheckInput{
		HealthCheck: &elb.HealthCheck{
			Target:             aws.String("SSL:" + targetPort),
			Timeout:            aws.Int64(5),
			Interval:           aws.Int64(30),
			HealthyThreshold:   aws.Int64(2),
			UnhealthyThreshold: aws.Int64(6),
		},
		LoadBalancerName: aws.String(loadBalancerName),
	}
	_, err := c.
		ELB.
		ConfigureHealthCheck(configureHealthCheckInput)
	return err
}

// GetAutoScalingGroups returns a filtered list of AutoScaling groups (only for the given list of AutoScaling
// group names <autoscalingGroupNames>).
func (c *Client) GetAutoScalingGroups(autoscalingGroupNames []*string) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	describeAutoScalingGroupsInput := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: autoscalingGroupNames,
	}
	return c.
		AutoScaling.
		DescribeAutoScalingGroups(describeAutoScalingGroupsInput)
}
