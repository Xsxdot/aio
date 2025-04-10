package sdk_v2

type Client struct {
	serviceInfo *ServiceInfo
}

func (o *ClientOptions) NewClient() *Client {

	return &Client{
		serviceInfo: o.serviceInfo,
	}
}
