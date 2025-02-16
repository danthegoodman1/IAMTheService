package http_server

type AWSProxy struct {
	// TODO all these lookup providers, esp for the service, feel a bit strange
	// AWS Key id to secret
	keyLookupProvider LookupProvider[string, string]
	// incoming hostname to outgoing hostname
	hostLookupProvider LookupProvider[string, string]
	// incoming hostname to service provider
	serviceLookupProvider LookupProvider[string, AWSServiceProvider]
}

func NewAWSProxy(
	keyLookupProvider, hostLookupProvider LookupProvider[string, string],
	serviceLookupProvider LookupProvider[string, AWSServiceProvider],
) AWSProxy {
	panic("todo")
}

// Listen will listen on an interface:port pair
func (p *AWSProxy) Listen(addr string) error {
	panic("todo")
}
