package srv

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	fetcher  releaseFetcher
	mappings mappings
	index    *projectIndex
}

// NewDocSrv creates a new `docsrv` service with the default configuration
// and the given default organisation and github api key.
func NewDocSrv(apiKey, org string) (*DocSrv, error) {
	mappings, err := loadMappings(defaultMappingsFile)
	if err != nil {
		return nil, err
	}

	return &DocSrv{
		defaultOwner: org,
		baseFolder:   defaultBaseFolder,
		sharedFolder: defaultSharedFolder,
		fetcher:      newReleaseFetcher(apiKey, 0),
		mappings:     mappings,
		index:        newProjectIndex(),
	}, nil
}

// ensureIndexed checks if the project is indexed and if it's not, it indexes
// it.
func (s *DocSrv) ensureIndexed(owner, project string) error {
	if !s.index.isIndexed(owner, project) {
		if err := s.indexProject(owner, project); err != nil {
			return err
		}
	}
	return nil
}

// indexProject indexes the given project.
func (s *DocSrv) indexProject(owner, project string) error {
	releases, err := s.fetcher.releases(owner, project)
	if err != nil {
		return err
	}

	s.index.set(owner, project, releases)
	return nil
}

// refreshIndex refreshes the version index of the projects already
// installed.
func (s *DocSrv) refreshIndex() {
	for _, key := range s.index.getProjects() {
		parts := splitKey(key)
		if len(parts) != 2 {
			logrus.WithField("key", key).Error("not a valid project key")
			continue
		}
		owner, project := parts[0], parts[1]

		err := s.indexProject(owner, project)
		if err != nil {
			logrus.WithField("owner", owner).
				WithField("project", project).
				Errorf("error refreshing project: %s", err)
		}
	}
}

// ManageIndex is in charge of refreshing the index of projects every
// five minutes until the given context is cancelled.
func (s *DocSrv) ManageIndex(refreshInterval time.Duration, ctx context.Context) {
	for {
		select {
		case <-time.After(refreshInterval):
			s.refreshIndex()
		case <-ctx.Done():
			return
		}
	}
}

// projectVersions returns all the versions available for the given project.
func (s *DocSrv) projectVersions(req *http.Request, owner, project string) []*version {
	releases := s.index.forProject(owner, project)
	var versions []*version
	for _, r := range releases {
		versions = append(versions, &version{
			Text: r.tag,
			URL:  urlFor(req, r.tag, ""),
		})
	}
	return versions
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

type version struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// listVersions is an HTTP handler that will output a JSON with all the versions
// available for a project.
func (s *DocSrv) listVersions(w http.ResponseWriter, r *http.Request) {
	owner, project := s.projectInfo(r)
	log := logrus.WithField("project", project).
		WithField("owner", owner)

	if err := s.ensureIndexed(owner, project); err != nil {
		log.Error("error indexing project: %s", err)
		internalError(w, r)
		return
	}

	versions := s.projectVersions(r, owner, project)

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

	if v, ok := s.index.latestVersion(owner, project); ok {
		redirectToVersion(w, r, v)
		return
	}

	if err := s.ensureIndexed(owner, project); err != nil {
		log.Error("error indexing project: %s", err)
		internalError(w, r)
		return
	}

	releases := s.index.forProject(owner, project)
	if len(releases) == 0 {
		log.Warn("no releases found for project")
		notFound(w, r)
		return
	}

	latest := releases[len(releases)-1]
	s.index.setLatestVersion(owner, project, latest.tag)
	redirectToVersion(w, r, latest.tag)
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

	if err := s.ensureIndexed(owner, project); err != nil {
		log.Error("error indexing project: %s", err)
		internalError(w, r)
		return
	}

	if s.index.isInstalled(owner, project, version) {
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

	release := s.index.get(owner, project, version)
	if release == nil {
		notFound(w, r)
		return
	}

	s.index.trySetLatestVersion(owner, project, release.tag)
	host := strings.Split(r.Host, ":")[0]
	destination := filepath.Join(s.baseFolder, host, version)
	if err := os.MkdirAll(destination, 0740); err != nil {
		log.Errorf("could not build folder structure for project %s: %s", project, err)
		internalError(w, r)
		return
	}

	conf := buildConfig{
		tarballURL:   release.url,
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

	s.index.install(owner, project, version)

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
