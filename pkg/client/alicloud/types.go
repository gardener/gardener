package alicloud

// ClientInterface is an interface which must be implemented by Alicloud clients.
type ClientInterface interface {
	GetCIDR(vpcID string) (string, error)
	//Return NatGatewayID, SnatTableID
	GetNatGatewayInfo(vpcID string) (string, string, error)
}
