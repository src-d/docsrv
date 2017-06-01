package docsrv

import (
	"strings"
	"sync"
)

type projectIndex struct {
	releasesMut *sync.RWMutex
	// releases contains a map from ${owner}/${project}/${version} to a specific
	// release.
	releases map[string]*release

	projectsMut *sync.RWMutex
	// projects contains a list of releases for each project in the form of
	// ${owner}/${project}
	projects map[string][]*release

	installedMut *sync.RWMutex
	// installed is a set of installed versions in the format ${owner}/${project}/${version}.
	installed map[string]struct{}
}

func newProjectIndex() *projectIndex {
	return &projectIndex{
		releasesMut:  new(sync.RWMutex),
		releases:     make(map[string]*release),
		projectsMut:  new(sync.RWMutex),
		projects:     make(map[string][]*release),
		installedMut: new(sync.RWMutex),
		installed:    make(map[string]struct{}),
	}
}

func (p *projectIndex) getProjects() []string {
	p.projectsMut.Lock()
	defer p.projectsMut.Unlock()
	var projs []string
	for key := range p.projects {
		projs = append(projs, key)
	}
	return projs
}

func (p *projectIndex) set(owner, project string, releases []*release) {
	key := newKey(owner, project)
	p.projectsMut.Lock()
	p.projects[key] = releases
	p.projectsMut.Unlock()

	p.releasesMut.Lock()
	defer p.releasesMut.Unlock()
	for _, r := range releases {
		key := newKey(owner, project, r.tag)
		p.releases[key] = r
	}
}

func (p *projectIndex) get(owner, project, version string) *release {
	p.releasesMut.Lock()
	defer p.releasesMut.Unlock()
	return p.releases[newKey(owner, project, version)]
}

func (p *projectIndex) forProject(owner, project string) []*release {
	p.projectsMut.Lock()
	defer p.projectsMut.Unlock()
	return p.projects[newKey(owner, project)]
}

func (p *projectIndex) isIndexed(owner, project string) bool {
	p.projectsMut.Lock()
	defer p.projectsMut.Unlock()
	_, ok := p.projects[newKey(owner, project)]
	return ok
}

// isInstalled reports whether the given project version is installed or not.
func (p *projectIndex) isInstalled(owner, project, version string) bool {
	key := newKey(owner, project, version)
	p.installedMut.Lock()
	defer p.installedMut.Unlock()
	_, ok := p.installed[key]
	return ok
}

// install marks as installed the given project version.
func (p *projectIndex) install(owner, project, version string) {
	key := newKey(owner, project, version)
	p.installedMut.Lock()
	defer p.installedMut.Unlock()
	p.installed[key] = struct{}{}
}

// newKey creates a new key for using in the index from the given set of
// strings.
func newKey(strs ...string) string {
	return strings.Join(strs, "/")
}

// splitKey returns the list of strings used to create the index key.
func splitKey(key string) []string {
	return strings.Split(key, "/")
}
