package srv

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v1"
)

// mappings represents a mapping from hosts to github repositories in the
// format "${OWNER}/${REPO_NAME}".
type mappings map[string]string

// forHost returns the owner and repo for a host and reports if such mapping
// could be found.
// If the mapping does not have a valid format it will act as if the mapping
// hadn't been found.
func (m mappings) forHost(host string) (owner, repo string, ok bool) {
	mapping, ok := m[host]
	if !ok {
		return "", "", false
	}

	parts := strings.Split(mapping, "/")
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// loadMappings loads the mappings at the given file.
func loadMappings(mappingFile string) (mappings, error) {
	var mappings = make(mappings)
	f, err := os.Open(mappingFile)
	if os.IsNotExist(err) {
		return mappings, nil
	} else if err != nil {
		return nil, fmt.Errorf("error opening mappings file: %s", err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("unable to read mappings file: %s", err)
	}

	if err := yaml.Unmarshal(data, &mappings); err != nil {
		return nil, fmt.Errorf("unable to unmarshal yaml from mappings file: %s", err)
	}

	return mappings, nil
}
