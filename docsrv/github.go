package srv

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var errNotFound = errors.New("unable to find a release")

// Release represents a project release with a tag name and an URL to the
// documentation asset.
type Release struct {
	// Tag of the release.
	Tag string
	// URL is the URL to the .tar.gz file with the repo files.
	URL string
}

// GitHub is a service to retrieve information from GitHub.
type GitHub interface {
	// Releases returns all the releases for a project.
	Releases(owner, project string) ([]*Release, error)
	// Release returns the requested release of a project.
	Release(owner, project, tag string) (*Release, error)
	// Latest returns the latest non-draft, non-prerelease release of a project.
	Latest(owner, project string) (*Release, error)
}

type gitHub struct {
	apiKey string
	client *github.Client
}

// NewGitHub creates a new GitHub service.
func NewGitHub(apiKey string) GitHub {
	var client *github.Client

	if apiKey != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
		client = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		client = github.NewClient(nil)
	}

	return &gitHub{apiKey, client}
}

func (g *gitHub) Latest(owner, project string) (*Release, error) {
	release, resp, err := g.client.Repositories.GetLatestRelease(context.Background(), owner, project)

	// to the go-github, a 404 is an error, but we differentiate between a 404
	// and a 500
	if r := newRelease(release); r == nil || resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	} else if err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func (g *gitHub) Releases(owner, project string) ([]*Release, error) {
	var result []*Release
	page := 1
	for {
		releases, resp, err := g.client.Repositories.ListReleases(
			context.Background(),
			owner,
			project,
			&github.ListOptions{Page: page, PerPage: 100},
		)

		if err != nil {
			return nil, err
		}

		for _, r := range releases {
			release := newRelease(r)
			if release == nil {
				continue
			}
			result = append(result, release)
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	sort.Sort(byTag(result))
	return result, nil
}

func (g *gitHub) Release(owner, project, tag string) (*Release, error) {
	release, resp, err := g.client.Repositories.GetReleaseByTag(
		context.Background(),
		owner,
		project,
		tag,
	)

	// to the go-github, a 404 is an error, but we differentiate between a 404
	// and a 500
	if r := newRelease(release); r == nil || resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	} else if err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func newRelease(r *github.RepositoryRelease) *Release {
	if r == nil || maybeBool(r.Draft) || maybeBool(r.Prerelease) {
		return nil
	}

	return &Release{
		Tag: maybeStr(r.TagName),
		URL: maybeStr(r.TarballURL),
	}
}

type byTag []*Release

func (b byTag) Len() int      { return len(b) }
func (b byTag) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byTag) Less(i, j int) bool {
	vi := newVersion(b[i].Tag)
	vj := newVersion(b[j].Tag)
	return vi.LessThan(vj)
}

func maybeBool(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

func maybeStr(str *string) string {
	if str != nil {
		return *str
	}
	return ""
}

func newVersion(v string) *semver.Version {
	return semver.MustParse(v)
}
