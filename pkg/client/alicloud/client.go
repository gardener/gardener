package alicloud

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
)

type client struct {
	vpcCli *vpc.Client
}

// NewClient creates a new Client for the given Alicloud credentials <accessKeyID>, <accessKeySecret>, and
// the region <region>.
func NewClient(accessKeyID, accessKeySecret, region string) (ClientInterface, error) {
	var vpcCli *vpc.Client
	var err error
	if accessKeyID != "" && accessKeySecret != "" && region != "" {
		vpcCli, err = vpc.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	} else {
		err = errors.New("alicloudAccessKeyID or alicloudAccessKeySecret can't be empty")
	}

	return &client{vpcCli}, err
}

//GetCIDR gets CIDR of the VPC specified by vpcID
func (c *client) GetCIDR(vpcID string) (string, error) {
	req := vpc.CreateDescribeVpcsRequest()
	req.VpcId = vpcID

	resp, err := c.vpcCli.DescribeVpcs(req)
	if err != nil {
		return "", err
	}

	if len(resp.Vpcs.Vpc) != 1 {
		return "", fmt.Errorf("Can't get VPC via id: %s", vpcID)
	}
	vpc := resp.Vpcs.Vpc[0]

	return vpc.CidrBlock, err
}

//GetNatGatewayID gets NatGatewayID and SnatTableID of the VPC specified by vpcID
func (c *client) GetNatGatewayInfo(vpcID string) (string, string, error) {
	req := vpc.CreateDescribeNatGatewaysRequest()
	req.VpcId = vpcID

	resp, err := c.vpcCli.DescribeNatGateways(req)
	if err != nil {
		return "", "", err
	}

	if len(resp.NatGateways.NatGateway) != 1 {
		return "", "", fmt.Errorf("Can't get NAT Gateway via id: %s", vpcID)
	}
	natgw := resp.NatGateways.NatGateway[0]

	return natgw.NatGatewayId, strings.Join(natgw.SnatTableIds.SnatTableId, ","), nil
}
