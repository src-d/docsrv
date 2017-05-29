# docsrv [![Build Status](https://travis-ci.org/src-d/docsrv.svg?branch=master)](https://travis-ci.org/src-d/docsrv)

`docsrv` is an app to serve versioned documentation for GitHub projects on demand.
Every time a documentation is requested for a version that is not on the server, it is fetched from a GitHub release and installed locally to be served by the Caddy webserver.

### Why?

This serves very specific needs, you probably just want to build your docs and push them to GitHub Pages.

### Accessing documentation

To access a documentation for a project, you will have to visit an URL with the following structure.

```
http(s)://{github project name}.yourdomain.tld/{version or latest}/{path}
```

The project name is taken from the subdomain of the host. (If you have more than one, let's say `foo.bar.domain.tld`, only `foo` will be used as project).

### Access list of versions for a project

```
http(s)://{github project name}.yourdomain.tld/versions.json
```

Will output something like this:

```json
[
        {"text": "v1.0.0", "url": "http://project.mydomain.tld/v1.0.0"},
        {"text": "v1.1.0", "url": "http://project.mydomain.tld/v1.1.0"},
]
```

### Release format

To build the documentation site of your project version, docsrv will download the tarball of the version with the contents your project had at that time. It is required to have a `Makefile` with a task named `docs`.

What docsrv will run to build your documentation is `make docs`, all the rest is handled by the makefile itself. Three parameters for the correct build of the documentation site are passed as environment variables.

* `DESTINATION_FOLDER`: root folder where the documentation site should be built by the makefile.
* `SHARED_FOLDER`: a shared folder where the makefile can store things (for example, to cache templates, etc).
* `BASE_URL`: the base url of the project site (e.g. `http://project.mydomain.tld/v1.0.0`).
* `VERSION_NAME`: version being built.
* `REPOSITORY`: repository name (e.g. `foo` for https://github.com/bar/foo).
* `REPOSITORY_OWNER`: repository owner name (e.g. `bar` for https://github.com/bar/foo).

### Release restrictions

A GitHub release can only be used with `docsrv` if is not a draft and is not a pre-release.

### Install and run

```
make build
docker build -t docsrv .
docker run -p 9090:9090 --name docsrv-instance \
        -e GITHUB_API_KEY "(optional) your github api key" \
        -e GITHUB_ORG "your github org/user name" \
        -v /path/to/host/logs:/var/log/docsrv \
        -v /path/to/error/pages:/var/www/public/errors \
        -v /path/to/init/scripts:/etc/docsrv/init.d \
        docsrv
```

**Notes:**

* The `GITHUB_ORG` will define the default user or organisation that owns the projects served by docsrv. For example, accessing `foo.yourdomain.tld` with `bar` as `GITHUB_ORG` will download and serve documentation for `https://github.com/bar/foo`.
* Specific hosts can have specific repositories mapped to them. You can do so by adding a mappings file at `/etc/docsrv/mappings.yml`.
* If not `GITHUB_API_KEY` is provided, the requests will not be authenticated. That means harder rate limits and unability to fetch private repositories.
* To override the error pages, mount a volume on `/var/www/public/errors` with `404.html` and `500.html`. If any of these two files does not exist, they will be created when the container starts. You may use assets contained in the same errors folder as if they were on the root of the site.
* You can add custom init bash scripts by mounting a volume on `/etc/docsrv/init.d`. All `*.sh` files there will be executed. You can use this to install dependencies needed by your documentation build scripts. Take into account the container is an alpine linux.

### Mappings file

In `/etc/docsrv/mappings.yml` you can optionally put a mappings file, which will contain a map from host to GitHub repos in the following format `${OWNER}/${REPO_NAME}`.

```
foo.mydomain.tld: myorg/foo
bar.mydomain.tld: otherorg/bar
```

### Recommended way to use and deploy docsrv

The recommended way to use and deploy docsrv is to have a repo/folder/something with all your configurations and mount all that as volumes in the docsrv container rather than creating your own dockerfile on top of docsrv's.

For example, something like this:

```
mydir/
  |- error-pages/
    |- 404.html
    |- 500.html
    |- css/
      |- style.css
  |- mappings.yml
  |- init.d/
    |- foo.sh
    |- bar.sh
```

Then mount it as a volume using `-v /path/to/mydir:/etc/docsrv` and `-v /path/to/mydir/error-pages:/var/www/public/errors` in the container.
