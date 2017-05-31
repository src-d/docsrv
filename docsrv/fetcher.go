package docsrv

import (
	"context"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// release represents a project release with a tag name and an URL to the
// documentation asset.
type release struct {
	// tag of the release.
	tag string
	// url is the url to the .tar.gz file with the repo files.
	url string
}

// releaseFetcher fetches the releases for projects.
type releaseFetcher interface {
	// releases returns all the releases for a project.
	releases(owner, project string) ([]*release, error)
}

type githubFetcher struct {
	apiKey  string
	client  *github.Client
	perPage int
}

// newReleaseFetcher creates a new release fetcher service that will fetch
// releases from GitHub.
// Giving a `perPage` value of 0 or less will set the default perPage value,
// which is 100 items per page.
func newReleaseFetcher(apiKey string, perPage int) releaseFetcher {
	var client *github.Client
	if perPage <= 0 {
		perPage = 100
	}

	if apiKey != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
		client = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		client = github.NewClient(nil)
	}

	return &githubFetcher{apiKey, client, perPage}
}

func (g *githubFetcher) releases(owner, project string) ([]*release, error) {
	var result []*release
	page := 1
	for {
		releases, resp, err := g.client.Repositories.ListReleases(
			context.Background(),
			owner,
			project,
			&github.ListOptions{Page: page, PerPage: g.perPage},
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

func newRelease(r *github.RepositoryRelease) *release {
	if r == nil || maybeBool(r.Draft) || maybeBool(r.Prerelease) {
		return nil
	}

	return &release{
		tag: maybeStr(r.TagName),
		url: maybeStr(r.TarballURL),
	}
}

type byTag []*release

func (b byTag) Len() int      { return len(b) }
func (b byTag) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byTag) Less(i, j int) bool {
	vi := newVersion(b[i].tag)
	vj := newVersion(b[j].tag)
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
