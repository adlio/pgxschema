package pgxschema

import "github.com/ory/dockertest"

type TestDB struct {
	DockerRepo string
	DockerTag  string
	DSN        string
	Resource   *dockertest.Resource
}
