package cloudproviders

// MetadataSearch is an interface to retrieve metadata from a cloud provider.
type MetadataSearch interface {
	// Name returns the providers name
	Name() string

	// IsRunningOn returns true if the agent is running on a specific provider.
	IsRunningOn() bool

	// Hostname returns the hostname as reported by the providers metadata remote store.
	Hostname() (string, error)
}
