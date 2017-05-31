package srv

import (
	"context"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Release represents a project release with a tag name and an URL to the
// documentation asset.
type Release struct {
	// Tag of the release.
	Tag string
	// URL is the URL to the .tar.gz file with the repo files.
	URL string
}

// releaseFetcher fetches the releases for projects.
type releaseFetcher interface {
	// Releases returns all the releases for a project.
	Releases(owner, project string) ([]*Release, error)
}

type githubFetcher struct {
	apiKey string
	client *github.Client
}

// newReleaseFetcher creates a new release fetcher service that will fetch
// releases from GitHub.
func newReleaseFetcher(apiKey string) releaseFetcher {
	var client *github.Client

	if apiKey != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
		client = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		client = github.NewClient(nil)
	}

	return &githubFetcher{apiKey, client}
}

func (g *githubFetcher) Releases(owner, project string) ([]*Release, error) {
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
