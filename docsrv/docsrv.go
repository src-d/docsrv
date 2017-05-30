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
	defaultMappingsFile = "/etc/docsrv/mappings.yml"
)

// DocSrv is the main docsrv service.
type DocSrv struct {
	// defaultOwner will be used as the owner of a repository by default
	// unless some other is specified using the mappings.yml file.
	// This is useful if the docs will only be generated for a single org
	// or user, since there will be no need to use the mappings.yml file
	// at all.
	defaultOwner string
	// baseFolder is the root folder served by the webserver.
	baseFolder string
	// sharedFolder is the location of the folder with all the shared assets.
	sharedFolder string

	github   GitHub
	mappings mappings

	mut *sync.RWMutex
	// latestVersions contains a map from a ${owner}/${project} to a latest
	// version, which is a version name with the time when it was last
	// installed.
	latestVersions map[string]latestVersion

	installMut *sync.RWMutex
	// installed is a set of installed versions in the format ${owner}/${project}/${version}.
	installed map[string]struct{}

	versionMut *sync.RWMutex
	// versions contains a map of all versions available for a project.
	versions map[string][]*version
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

// isExpired reports whether this version is expired or not and should be re-checked.
func (v latestVersion) isExpired() bool {
	return v.cachedAt.Add(latestVersionLifetime).After(time.Now())
}

// latestVersionLifetime defines the time a latest version is valid.
const latestVersionLifetime = 1 * time.Hour

// NewDocSrv creates a new `docsrv` service with the default configuration
// and the given default organisation and github api key.
func NewDocSrv(apiKey, org string) (*DocSrv, error) {
	mappings, err := loadMappings(defaultMappingsFile)
	if err != nil {
		return nil, err
	}

	return &DocSrv{
		org,
		defaultBaseFolder,
		defaultSharedFolder,
		NewGitHub(apiKey),
		mappings,
		new(sync.RWMutex),
		make(map[string]latestVersion),
		new(sync.RWMutex),
		make(map[string]struct{}),
		new(sync.RWMutex),
		make(map[string][]*version),
	}, nil
}

// setLatestVersion will set the given version as the latest version for a
// project.
func (s *DocSrv) setLatestVersion(owner, project, version string) {
	key := filepath.Join(owner, project)
	s.mut.Lock()
	defer s.mut.Unlock()
	s.latestVersions[key] = latestVersion{time.Now(), version}
}

// latestVersion will return the latest version of a project and a boolean
// reporting whether or not that version exists.
// If the version is expired, it will return false.
func (s *DocSrv) latestVersion(owner, project string) (string, bool) {
	key := filepath.Join(owner, project)
	s.mut.Lock()
	defer s.mut.Unlock()
	v := s.latestVersions[key]
	if v.isExpired() {
		return v.version, true
	}
	return "", false
}

// trySetLatestVersion will set the latest version of a given project to the
// given one only if there is a previous version and is lower than the
// given one.
func (s *DocSrv) trySetLatestVersion(owner, project, version string) {
	if v, ok := s.latestVersion(owner, project); ok {
		v1 := newVersion(v)
		v2 := newVersion(version)

		if v1.LessThan(v2) {
			s.setLatestVersion(owner, project, version)
		}
	}
}

// isInstalled reports whether the given project version is installed or not.
func (s *DocSrv) isInstalled(owner, project, version string) bool {
	key := filepath.Join(owner, project, version)
	s.installMut.Lock()
	defer s.installMut.Unlock()
	_, ok := s.installed[key]
	return ok
}

// install marks as installed the given project version.
func (s *DocSrv) install(owner, project, version string) {
	key := filepath.Join(owner, project, version)
	s.installMut.Lock()
	defer s.installMut.Unlock()
	s.installed[key] = struct{}{}
}

// projectVersions returns all the versions installed for the given project.
func (s *DocSrv) projectVersions(owner, project string) []*version {
	key := filepath.Join(owner, project)
	s.versionMut.Lock()
	defer s.versionMut.Unlock()
	return s.versions[key]
}

// refreshProjectVersions retrieves all the versions available for a project
// and caches them.
func (s *DocSrv) refreshProjectVersions(req *http.Request, owner, project string) error {
	releases, err := s.github.Releases(owner, project)
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

	key := filepath.Join(owner, project)
	s.versions[key] = versions
	return nil
}

// projectInfo returns the owner and the project for the host in the given
// http request.
// If there is a mapping for that host, the mapping will be used. Otherwise,
// the default owner and the project name from the host will be used.
func (s *DocSrv) projectInfo(r *http.Request) (owner, project string) {
	if owner, project, ok := s.mappings.forHost(r.Host); ok {
		return owner, project
	}

	return s.defaultOwner, projectNameFromReq(r)
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

// listVersions is an HTTP handler that will output a JSON with all the versions
// available for a project.
func (s *DocSrv) listVersions(w http.ResponseWriter, r *http.Request) {
	owner, project := s.projectInfo(r)
	versions := s.projectVersions(owner, project)
	log := logrus.WithField("project", project).
		WithField("owner", owner)

	// if versions is nil, project versions haven't been refreshed yet
	// so refresh them and then serve them
	if versions == nil {
		if err := s.refreshProjectVersions(r, owner, project); err != nil {
			log.Errorf("error refreshing project versions: %s", err)
			internalError(w, r)
			return
		}

		versions = s.projectVersions(owner, project)
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

// redirectToLatest is an HTTP service that will redirect to the latest version
// of the project preserving the path it had in the original request.
func (s *DocSrv) redirectToLatest(w http.ResponseWriter, r *http.Request) {
	owner, project := s.projectInfo(r)
	log := logrus.WithField("project", project).
		WithField("owner", owner)

	if v, ok := s.latestVersion(owner, project); ok {
		redirectToVersion(w, r, v)
		return
	}

	latest, err := s.github.Latest(owner, project)
	if err == errNotFound {
		log.Warn("no releases found for project")
		notFound(w, r)
		return
	} else if err != nil {
		log.Errorf("could not find latest release for project: %s", err)
		internalError(w, r)
		return
	}

	s.setLatestVersion(owner, project, latest.Tag)
	redirectToVersion(w, r, latest.Tag)
}

// prepareVersion is an HTTP handler that will fetch, download and build the
// documentation site for the specified project version if it was not already
// built and then redirect the user to the same visit so the webserver can
// serve the static documentation.
func (s *DocSrv) prepareVersion(w http.ResponseWriter, r *http.Request) {
	var (
		owner, project = s.projectInfo(r)
		version        = versionFromReq(r)
		log            = logrus.WithField("project", project).
				WithField("owner", owner).
				WithField("version", version)
	)

	if s.isInstalled(owner, project, version) {
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
		if err := s.refreshProjectVersions(r, owner, project); err != nil {
			log.Error(err.Error())
		}

		wg.Done()
	}()

	release, err := s.github.Release(owner, project, version)
	if err == errNotFound {
		notFound(w, r)
		return
	} else if err != nil {
		log.Errorf("could not find release for project: %s", err)
		internalError(w, r)
		return
	}

	s.trySetLatestVersion(owner, project, release.Tag)
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
		owner:        owner,
	}
	if err := buildDocs(conf); err != nil {
		log.Errorf("could not build docs for project %s: %s", project, err)
		internalError(w, r)
		return
	}

	wg.Wait()
	s.install(owner, project, version)

	http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
}

func ensureEndingSlash(url string) string {
	if strings.HasSuffix(url, "/") {
		return url
	}
	return url + "/"
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/404.html", http.StatusTemporaryRedirect)
}

func internalError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/500.html", http.StatusTemporaryRedirect)
}

func redirectToVersion(w http.ResponseWriter, r *http.Request, version string) {
	path := strings.Replace(r.URL.Path, "/latest/", "", 1)
	url := urlFor(r, version, path)
	if path == "" {
		url = ensureEndingSlash(url)
	}
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func urlFor(r *http.Request, version, path string) string {
	return reqScheme(r) + "://" + filepath.Join(r.Host, version, path)
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
