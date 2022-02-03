package discover

type NodeDiscovery interface {
	Discover() error
	IsConfigured() bool
}
