package algorand

type Algorand struct {
}

func NewAlgorand() (*Algorand, error) {
	// load a config or create a default one
	a := &Algorand{}
	return a, nil
}

func (a *Algorand) IsConfigured() bool {
	// if any of a.config.X == "" return false
	// else return true
	panic("implement me!")
}

func (a *Algorand) Discover() error {
	// check the config first
	// heavy lifting: checking the docker, extracting PID etc. and populating a.config
	// success is basically same as calling IsConfigured() again.
	panic("implement me!")
}
