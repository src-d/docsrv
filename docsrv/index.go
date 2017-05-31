package srv

import "sync"

type projectIndex struct {
	releasesMut *sync.RWMutex
	releases    map[string]*release

	projectsMut *sync.RWMutex
	projects    map[string][]*release
}

func newProjectIndex() *projectIndex {
	return &projectIndex{
		releasesMut: new(sync.RWMutex),
		releases:    make(map[string]*release),
		projectsMut: new(sync.RWMutex),
		projects:    make(map[string][]*release),
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
