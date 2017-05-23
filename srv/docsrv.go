package srv

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/Sirupsen/logrus"
)

const (
	defaultSharedFolder = "/etc/shared"
	defaultBaseFolder   = "/var/www/public"
)

type DocSrv struct {
	owner          string
	baseFolder     string
	sharedFolder   string
	github         GitHub
	mut            *sync.RWMutex
	latestVersions map[string]latestVersion
	installMut     *sync.RWMutex
	installed      map[string]struct{}
	versionMut     *sync.RWMutex
	versions       map[string][]*version
}

type version struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// latestVersion is a version name with the time it was inserted in the cache.
type latestVersion struct {
	cachedAt time.Time
	version  string
}

// latestVersionLifetime defines the time a latest version is valid.
const latestVersionLifetime = 1 * time.Hour

func NewDocSrv(apiKey, org string) *DocSrv {
	return &DocSrv{
		org,
		defaultBaseFolder,
		defaultSharedFolder,
		NewGitHub(apiKey, org),
		new(sync.RWMutex),
		make(map[string]latestVersion),
		new(sync.RWMutex),
		make(map[string]struct{}),
		new(sync.RWMutex),
		make(map[string][]*version),
	}
}

// setLatestVersion will set the given version as the latest version for a
// project.
func (s *DocSrv) setLatestVersion(project, version string) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.latestVersions[project] = latestVersion{time.Now(), version}
}

// latestVersion will return the latest version of a project and a boolean
// reporting whether or not that version exists.
// If the version is expired, it will return false.
func (s *DocSrv) latestVersion(project string) (string, bool) {
	s.mut.Lock()
	defer s.mut.Unlock()
	v := s.latestVersions[project]
	if v.cachedAt.Add(latestVersionLifetime).After(time.Now()) {
		return v.version, true
	}
	return "", false
}

// trySetLatestVersion will set the latest version of a given project to the
// given one only if there is a previous version and is lower than the
// given one.
func (s *DocSrv) trySetLatestVersion(project, version string) {
	if v, ok := s.latestVersion(project); ok {
		v1 := newVersion(v)
		v2 := newVersion(version)

		if v1.LessThan(v2) {
			s.setLatestVersion(project, version)
		}
	}
}

func (s *DocSrv) isInstalled(project, version string) bool {
	path := filepath.Join(project, version)
	s.installMut.Lock()
	defer s.installMut.Unlock()
	_, ok := s.installed[path]
	return ok
}

func (s *DocSrv) install(project, version string) {
	path := filepath.Join(project, version)
	s.installMut.Lock()
	defer s.installMut.Unlock()
	s.installed[path] = struct{}{}
}

func (s *DocSrv) projectVersions(project string) []*version {
	s.versionMut.Lock()
	defer s.versionMut.Unlock()
	return s.versions[project]
}

func (s *DocSrv) refreshProjectVersions(req *http.Request, project string) error {
	releases, err := s.github.Releases(project)
	if err != nil {
		return err
	}

	versions := []*version{}
	for _, r := range releases {
		versions = append(versions, &version{
			Text: r.Tag,
			URL:  urlFor(req, r.Tag, ""),
		})
	}

	s.versionMut.Lock()
	defer s.versionMut.Unlock()

	s.versions[project] = versions
	return nil
}

func (s *DocSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer recoverFromPanic(w, r)

	if r.URL.Path == "/versions.json" {
		s.listVersions(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/latest/") {
		s.redirectToLatest(w, r)
	} else {
		s.prepareVersion(w, r)
	}
}

func (s *DocSrv) listVersions(w http.ResponseWriter, r *http.Request) {
	project := projectNameFromReq(r)
	versions := s.projectVersions(project)
	log := logrus.WithField("project", project)

	// if versions is nil, project versions haven't been refreshed yet
	// so refresh them and then serve them
	if versions == nil {
		if err := s.refreshProjectVersions(r, project); err != nil {
			log.Errorf("error refreshing project versions: %s", err)
			internalError(w, r)
			return
		}

		versions = s.projectVersions(project)
	}

	data, err := json.Marshal(versions)
	if err != nil {
		log.Errorf("error serving project versions: %s", err)
		internalError(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *DocSrv) redirectToLatest(w http.ResponseWriter, r *http.Request) {
	project := projectNameFromReq(r)
	log := logrus.WithField("project", project)

	if v, ok := s.latestVersion(project); ok {
		redirectToVersion(w, r, v)
		return
	}

	latest, err := s.github.Latest(project)
	if err == ErrNotFound {
		log.Warn("no releases found for project")
		notFound(w, r)
		return
	} else if err != nil {
		log.Errorf("could not find latest release for project: %s", err)
		internalError(w, r)
		return
	}

	s.setLatestVersion(project, latest.Tag)
	redirectToVersion(w, r, latest.Tag)
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/404.html", http.StatusTemporaryRedirect)
}

func internalError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/500.html", http.StatusTemporaryRedirect)
}

func redirectToVersion(w http.ResponseWriter, r *http.Request, version string) {
	path := strings.Replace(r.URL.Path, "/latest/", "", 1)
	http.Redirect(w, r, urlFor(r, version, path), http.StatusTemporaryRedirect)
}

func urlFor(r *http.Request, version, path string) string {
	return reqScheme(r) + "://" + filepath.Join(r.Host, version, path)
}

func (s *DocSrv) prepareVersion(w http.ResponseWriter, r *http.Request) {
	var (
		project = projectNameFromReq(r)
		version = versionFromReq(r)
		log     = logrus.WithField("project", project).
			WithField("version", version)
	)

	if s.isInstalled(project, version) {
		// If the version is not a version, it's probably a file, so send just a basic 404 status
		// code instead of the full not found page.
		if _, err := semver.NewVersion(version); err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// if docs for this version are installed but the request made it here
		// it means the document being requested does not exist.
		notFound(w, r)
		return
	}

	// refresh project versions in parallel to not block the other expensive
	// operation: actually downloading and building the docs.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := s.refreshProjectVersions(r, project); err != nil {
			log.Error(err.Error())
		}

		wg.Done()
	}()

	release, err := s.github.Release(project, version)
	if err == ErrNotFound {
		notFound(w, r)
		return
	} else if err != nil {
		log.Errorf("could not find release for project: %s", err)
		internalError(w, r)
		return
	}

	s.trySetLatestVersion(project, release.Tag)
	host := strings.Split(r.Host, ":")[0]
	destination := filepath.Join(s.baseFolder, host, version)
	if err := os.MkdirAll(destination, 0740); err != nil {
		log.Errorf("could not build folder structure for project %s: %s", project, err)
		internalError(w, r)
		return
	}

	conf := buildConfig{
		tarballURL:   release.URL,
		baseURL:      urlFor(r, version, ""),
		destination:  destination,
		sharedFolder: s.sharedFolder,
		version:      version,
		project:      project,
		owner:        s.owner,
	}
	if err := buildDocs(conf); err != nil {
		log.Errorf("could not build docs for project %s: %s", project, err)
		internalError(w, r)
		return
	}

	wg.Wait()
	s.install(project, version)

	http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
}

func projectNameFromReq(r *http.Request) string {
	return strings.Split(r.Host, ".")[0]
}

func versionFromReq(r *http.Request) string {
	return strings.Split(strings.TrimLeft(r.URL.Path, "/"), "/")[0]
}

func recoverFromPanic(w http.ResponseWriter, req *http.Request) {
	if r := recover(); r != nil {
		logrus.WithField("URL", req.URL.String()).
			Errorf("recovered from panic: %v", r)
		internalError(w, req)
	}
}

func reqScheme(r *http.Request) string {
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = r.Header.Get("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "http"
		}
	}
	return scheme
}
