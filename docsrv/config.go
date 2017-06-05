package docsrv

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver"
)

// Config is a map from hosts to project configurations.
type Config map[string]ProjectConfig

// ProjectForHost will returne the owner and repository name of the project
// in the given host. Will also report whether or not the project could be found
// with a boolean.
func (c Config) ProjectForHost(host string) (owner, repo string, ok bool) {
	proj, ok := c[host]
	if !ok {
		return "", "", false
	}

	parts := strings.Split(proj.Repository, "/")
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// MinVersionForHost will return the minimum version for a project at the
// given host.
// It will return nil if no such host can be found or if the version is not
// valid or is missing.
func (c Config) MinVersionForHost(host string) *semver.Version {
	project, ok := c[host]
	if !ok {
		return nil
	}

	return newVersion(project.MinVersion)
}

// ProjectConfig represents a single project configuration.
type ProjectConfig struct {
	// Repository is the repository this project maps to in the format "${OWNER}/${PROJECT}".
	Repository string `toml:"repository"`
	// MinVersion is the minimum version of this project for which documentation
	// sites can be built.
	MinVersion string `toml:"min-version"`
}

// LoadConfig loads the config from the given file.
func LoadConfig(file string) (Config, error) {
	var config = make(Config)
	data, err := ioutil.ReadFile(file)
	if os.IsNotExist(err) {
		return config, nil
	} else if err != nil {
		return nil, fmt.Errorf("error reading config file: %s", err)
	}

	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to unmarshal yaml from config file: %s", err)
	}

	return config, nil
}
