package docsrv

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

// Options contains all the options available for creating a new DocSrv
// service.
type Options struct {
	// GitHubAPIKey is the API key used to retrieve releases from GitHub.
	// If api key is empty the requests will be made without authentication.
	GitHubAPIKey string
	// DefaultOwner is the GitHub username or organization that will be used
	// in case no mapping is found for a host. It's particularly useful if the
	// documentations will always be for a specific organization, which would
	// avoid requiring any mappings at all.
	DefaultOwner string
	// BaseFolder is the path to the root folder of the webserver.
	BaseFolder string
	// SharedFolder is the path to the folder used to store all the common
	// assets for building the documentations.
	SharedFolder string
	// Mappings contains all the mappings between hosts and a repository
	// in the form ${owner}/${project} (in GitHub).
	Mappings Mappings
}

// Service is the main docsrv service.
type Service struct {
	opts    Options
	fetcher releaseFetcher
	index   *projectIndex
}

// New creates a new DocSrv service with the given options.
func New(opts Options) *Service {
	if opts.Mappings == nil {
		opts.Mappings = make(Mappings)
	}

	return &Service{
		opts:    opts,
		fetcher: newReleaseFetcher(opts.GitHubAPIKey, 0),
		index:   newProjectIndex(),
	}
}

// ensureIndexed checks if the project is indexed and if it's not, it indexes
// it.
func (s *Service) ensureIndexed(owner, project string) error {
	if !s.index.isIndexed(owner, project) {
		if err := s.indexProject(owner, project); err != nil {
			return err
		}
	}
	return nil
}

// indexProject indexes the given project.
func (s *Service) indexProject(owner, project string) error {
	releases, err := s.fetcher.releases(owner, project)
	if err != nil {
		return err
	}

	s.index.set(owner, project, releases)
	return nil
}

// refreshIndex refreshes the version index of the projects already installed.
func (s *Service) refreshIndex() {
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
func (s *Service) ManageIndex(refreshInterval time.Duration, ctx context.Context) {
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
func (s *Service) projectVersions(req *http.Request, owner, project string) []*version {
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
func (s *Service) projectInfo(r *http.Request) (owner, project string) {
	if owner, project, ok := s.opts.Mappings.forHost(r.Host); ok {
		return owner, project
	}

	return s.opts.DefaultOwner, projectNameFromReq(r)
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer recoverFromPanic(w, r)
	logrus.WithField("path", r.URL.Path).Debug("new request received")

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
func (s *Service) listVersions(w http.ResponseWriter, r *http.Request) {
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
func (s *Service) redirectToLatest(w http.ResponseWriter, r *http.Request) {
	owner, project := s.projectInfo(r)
	log := logrus.WithField("project", project).
		WithField("owner", owner)
	defer log.Debug("correctly redirected to latest version")

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
func (s *Service) prepareVersion(w http.ResponseWriter, r *http.Request) {
	var (
		owner, project = s.projectInfo(r)
		version        = versionFromReq(r)
		log            = logrus.WithField("project", project).
				WithField("owner", owner).
				WithField("version", version)
	)

	log.Debug("preparing version")
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

		log.Debug("release was already installed but the request made it to docsrv and not the webserver")

		// if docs for this version are installed but the request made it here
		// it means the document being requested does not exist.
		notFound(w, r)
		return
	}

	release := s.index.get(owner, project, version)
	if release == nil {
		log.Debug("release was not found")
		notFound(w, r)
		return
	}

	s.index.trySetLatestVersion(owner, project, release.tag)
	host := strings.Split(r.Host, ":")[0]
	destination := filepath.Join(s.opts.BaseFolder, host, version)
	if err := os.MkdirAll(destination, 0740); err != nil {
		log.Errorf("could not build folder structure for project %s: %s", project, err)
		internalError(w, r)
		return
	}

	conf := buildConfig{
		tarballURL:   release.url,
		baseURL:      urlFor(r, version, ""),
		destination:  destination,
		sharedFolder: s.opts.SharedFolder,
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

	log.Debug("version successfully installed and prepared")
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
