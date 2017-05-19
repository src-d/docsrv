package srv

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/c4milo/unpackit"
)

var sharedRepo = os.Getenv("SHARED_REPO") != ""

const (
	sharedFolder      = "/etc/shared"
	defaultBaseFolder = "/var/www/public"
)

type DocSrv struct {
	baseFolder     string
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
		defaultBaseFolder,
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
	releases, err := s.github.Releases(project, true)
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
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		versions = s.projectVersions(project)
	}

	data, err := json.Marshal(versions)
	if err != nil {
		log.Errorf("error serving project versions: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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

	releases, err := s.github.Releases(project, false)
	if err != nil {
		log.Errorf("could not find releases for project: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(releases) == 0 {
		log.Warn("no releases found for project")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	latest := releases[len(releases)-1]
	s.setLatestVersion(project, latest.Tag)
	redirectToVersion(w, r, latest.Tag)
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

	if !s.isInstalled(project, version) {
		var done = make(chan struct{}, 0)
		go func() {
			if err := s.refreshProjectVersions(r, project); err != nil {
				log.Error(err.Error())
			}

			close(done)
		}()

		release, err := s.github.Release(project, version)
		if err != nil {
			log.Errorf("could not find release for project: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		s.trySetLatestVersion(project, release.Tag)
		host := strings.Split(r.Host, ":")[0]
		destination := filepath.Join(s.baseFolder, host, version)
		if err := os.MkdirAll(destination, 0740); err != nil {
			log.Errorf("could not build folder structure for project %s: %s", project, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		baseURL := urlFor(r, version, "")
		if err := buildDocs(release.Docs, baseURL, destination); err != nil {
			log.Errorf("could not build docs for project %s: %s", project, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		<-done
		s.install(project, version)
	}

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
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func buildDocs(docsURL, baseURL, destination string) error {
	resp, err := http.Get(docsURL)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "docsrv-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %s", err)
	}

	defer resp.Body.Close()
	dir, err := unpackit.Unpack(resp.Body, tmpDir)
	if err != nil {
		return fmt.Errorf("error untarring %q: %s", docsURL, err)
	}

	cmd := exec.Command(
		"make",
		"build",
	)
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		"BASE_URL="+baseURL,
		"DESTINATION_FOLDER="+destination,
		"SHARED_REPO_FOLDER="+sharedFolder,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running `make build` of docs folder at %q: %s. Full error: %s", dir, err, string(output))
	}

	if err := os.RemoveAll(dir); err != nil {
		logrus.Warnf("could not delete temp files at %q: %s", dir, err)
	}

	return nil
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
