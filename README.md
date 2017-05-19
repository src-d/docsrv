# docsrv

`docsrv` is an app to serve versioned documentation for GitHub projects on demand.
Every time a documentation is requested for a version that is not on the server, it is fetched from a GitHub release and installed locally to be served by the Caddy webserver.

### Why?

This serves very specific needs, you probably just want to build your docs and push them to GitHub Pages.

### Accessing documentation

To access a documentation for a project, you will have to visit an URL with the following structure.

```
http://{github project name}.yourdomain.tld/{version or latest}/{path}
```

The project name is taken from the subdomain of the host. (If you have more than one, let's say `foo.bar.domain.tld`, only `foo` will be used as project).

### Release format

A GitHub release can only be used with `docsrv` if it contains a `docs.tar.gz` asset, is not a draft and is not a pre-release.

This `docs.tar.gz` will need to have a `Makefile` in its root with a rule named `build`.

The following environment variables will be available for the makefile to use and build itself.

* `BASE_URL`: base url of that project version e.g. `http://project.domain.tld/v1.0.0`.
* `DESTINATION_FOLDER`: folder where the docs should be built.
* `SHARED_REPO_FOLDER`: folder where the shared repo, if any, is.

### Install and run

```
make build
docker build -t docsrv .
docker run -p 9090:9090 --name docsrv-instance \
        -e GITHUB_API_KEY "(optional) your github api key" \
        -e GITHUB_ORG "your github org/user name" \
        -e SHARED_REPO "(optional) url of the the repo" \
        -v /path/to/host/logs:/var/log/docsrv \
        docsrv
```

**Notes:**

* `SHARED_REPO` will be downloaded at the start of the container. It can be used to download a repo that will contain files needed by the docs to be built.
* If not `GITHUB_API_KEY` is provided, the requests will not be authenticated. That means harder rate limits and unability to fetch private repositories.
