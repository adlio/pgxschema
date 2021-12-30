package pgxschema

import "github.com/ory/dockertest"

type ConnInfo struct {
	DockerRepo string
	DockerTag  string
	DSN        string
	Resource   *dockertest.Resource
}
