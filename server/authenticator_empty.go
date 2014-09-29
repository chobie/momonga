package server

import (
	"github.com/chobie/momonga/configuration"
)

// EmptyAuthenticator allows anything.
type EmptyAuthenticator struct {
}

func (self *EmptyAuthenticator) Init(config *configuration.Config) {
}

func (self *EmptyAuthenticator) Authenticate(user_id, password []byte) (bool, error) {
	return true, nil
}

func (self *EmptyAuthenticator) Shutdown() {
}
