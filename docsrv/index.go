package docsrv

import (
	"strings"
	"sync"
	"time"
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

	latestMut *sync.RWMutex
	// latestVersions contains a map from a ${owner}/${project} to a latest
	// version, which is a version name with the time when it was last
	// installed.
	latestVersions map[string]latestVersion

	installedMut *sync.RWMutex
	// installed is a set of installed versions in the format ${owner}/${project}/${version}.
	installed map[string]struct{}
}

func newProjectIndex() *projectIndex {
	return &projectIndex{
		releasesMut:    new(sync.RWMutex),
		releases:       make(map[string]*release),
		projectsMut:    new(sync.RWMutex),
		projects:       make(map[string][]*release),
		latestMut:      new(sync.RWMutex),
		latestVersions: make(map[string]latestVersion),
		installedMut:   new(sync.RWMutex),
		installed:      make(map[string]struct{}),
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

// setLatestVersion will set the given version as the latest version for a
// project.
func (p *projectIndex) setLatestVersion(owner, project, version string) {
	key := newKey(owner, project)
	p.latestMut.Lock()
	defer p.latestMut.Unlock()
	p.latestVersions[key] = latestVersion{time.Now(), version}
}

// latestVersion will return the latest version of a project and a boolean
// reporting whether or not that version exists.
// If the version is expired, it will return false.
func (p *projectIndex) latestVersion(owner, project string) (string, bool) {
	key := newKey(owner, project)
	p.latestMut.Lock()
	defer p.latestMut.Unlock()
	v := p.latestVersions[key]
	if v.isExpired() {
		return v.version, true
	}
	return "", false
}

// trySetLatestVersion will set the latest version of a given project to the
// given one only if there is a previous version and is lower than the
// given one.
func (p *projectIndex) trySetLatestVersion(owner, project, version string) {
	if v, ok := p.latestVersion(owner, project); ok {
		v1 := newVersion(v)
		v2 := newVersion(version)

		if v1.LessThan(v2) {
			p.setLatestVersion(owner, project, version)
		}
	}
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

// latestVersion is a version name with the time it was inserted in the cache.
type latestVersion struct {
	cachedAt time.Time
	version  string
}

// isExpired reports whether this version is expired or not and should be re-checked.
func (v latestVersion) isExpired() bool {
	return v.cachedAt.Add(latestVersionLifetime).After(time.Now())
}

// latestVersionLifetime defines the time a latest version is valid.
const latestVersionLifetime = 1 * time.Hour

// newKey creates a new key for using in the index from the given set of
// strings.
func newKey(strs ...string) string {
	return strings.Join(strs, "/")
}

// splitKey returns the list of strings used to create the index key.
func splitKey(key string) []string {
	return strings.Split(key, "/")
}
