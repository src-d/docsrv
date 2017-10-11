# docsrv [![Build Status](https://travis-ci.org/src-d/docsrv.svg?branch=master)](https://travis-ci.org/src-d/docsrv)

`docsrv` is an app to serve versioned documentation for GitHub projects on demand.
Every time a documentation is requested for a version that is not on the server, it is fetched from a GitHub release and installed locally to be served by the Caddy webserver.

### Why?

This serves very specific needs, you probably just want to build your docs and push them to GitHub Pages.

### Accessing documentation

To access a documentation for a project, you will have to visit an URL with the following structure.

```
http(s)://{name}.yourdomain.tld/{version or latest}/{path}
```

That host will have a mapping to a GitHub project in the `config.toml` file so docsrv knows what project it must serve.

### Access list of versions for a project

```
http(s)://{name}.yourdomain.tld/versions.json
```

Will output something like this:

```json
[
        {"text": "v1.0.0", "url": "http://name.mydomain.tld/v1.0.0"},
        {"text": "v1.1.0", "url": "http://name.mydomain.tld/v1.1.0"},
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
        -e GITHUB_API_KEY="(optional) your github api key" \
        -e DOCSRV_REFRESH="(optional) number of minutes between refreshes" \
        -e DEBUG_LOG="(optional) true" \
        -e REFRESH_TOKEN="(optional) your_token" \
        -v /path/to/error/pages:/var/www/public/errors \
        -v /path/to/config/folder:/etc/docsrv/conf.d \
        -v /path/to/init/scripts:/etc/docsrv/init.d \
        docsrv
```

* You need to add a `config.toml` file in `/etc/docsrv/conf.d`, which you can do mounting a volume in that folder.
* The `DEBUG_LOG` env variable will output the really, really verbose messages on the log file. This is not enabled by default.
* The `DOCSRV_REFRESH` env variable will define how many minutes will have to pass for the service to refresh the releases of a project.
The default value is `5` minutes.
A higher number means less chances of getting GitHub rate limit. Unauthenticated rate is 60 reqs/hour, authenticated rate is 5000 reqs/hour, so if you have a lot of projects with a lot of releases you might want to set a higher value than the default and if you have a small amount of projects with few releases but want the refresh times to be smaller use a smaller value.
* If no `GITHUB_API_KEY` is provided, the requests will not be authenticated. That means harder rate limits (60 reqs / hour) and unability to fetch private repositories.
* To override the error pages, mount a volume on `/var/www/public/errors` with `404/index.html` and `500/index.html`. If any of these two files does not exist, they will be created when the container starts. You may use assets contained in the same errors folder as if they were on the root of the site.
* You can add custom init bash scripts by mounting a volume on `/etc/docsrv/init.d`. All `*.sh` files there will be executed. You can use this to install dependencies needed by your documentation build scripts. Take into account the container is an alpine linux.
* `REFRESH_TOKEN` can be used to enable refreshes of the cache before the time specified in `REFRESH_INTERVAL`. If your documentation takes a lot to build you probably want to build it ahead of time and leave it cached for your users so they don't have to wait for it to build. This mechanism is meant to be used in a CI when you make a release. Just ping `http://project.yourdomain.tld/refresh/${VERSION}/?token=${YOUR REFRESH TOKEN}` and the cache will be refreshed and this version built.

### Config file

In `/etc/docsrv/conf.d/config.toml` you need to put the configuration for docsrv, which is a mapping between hosts and project configurations.

Example `config.toml`

```
["bar.domain.tld"]
  repository = "foo/bar"
  min-version = "v1.1.0"

["baz.anotherdomain.tld"]
  repository = "foo/baz"
  min-version = "v1.0.0"
```

The host name must **not** contain the port.

The project configurations available for each host are `repository`, which is the GitHub repository whose docs will be served in that host in the format `${OWNER}/${PROJECT}` and `min-version`, the minimum version of the project for which docs can be built.

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
  |- conf.d/
    |- config.toml
  |- init.d/
    |- foo.sh
    |- bar.sh
```

Then mount it as a volume using `-v /path/to/mydir:/etc/docsrv` and `-v /path/to/mydir/error-pages:/var/www/public/errors` in the container.

### Order of precedence in serving requests

1. `/versions.json` without any version has the highest precedence.
2. `/` without any version has the second highest predecence and acts as if it was `/latest/`.
3. `/$VERSION/$PATH` has the lowest precedence.
4. `/var/www/public/errors/$PATH`

### Develop

You may use the `dotenv.example` file as a template for a `.env` file. There
you can uncomment and set some env variables.
