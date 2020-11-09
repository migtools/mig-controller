<<<<<<< HEAD
# Setup from scratch

1. [Install Go](https://golang.org/dl/).
    1. Ensure that your `GOBIN` directory (by default `$(go env GOPATH)/bin`)
    is in your `PATH`.
    1. Check it's working by running `go version`.
        * If it doesn't work, check the install location, usually
        `/usr/local/go`, is on your `PATH`.

1. Sign one of the
[contributor license agreements](#contributor-license-agreements) below.

1. Clone the repo:
    `git clone https://github.com/googleapis/google-cloud-go`

1. Change into the checked out source:
    `cd google-cloud-go`

1. Fork the repo and add your fork as a secondary remote (this is necessary in
   order to create PRs).

# Which module to release?

The Go client libraries have several modules. Each module does not strictly
correspond to a single library - they correspond to trees of directories. If a
file needs to be released, you must release the closest ancestor module.

To see all modules:

```
$ cat `find . -name go.mod` | grep module
module cloud.google.com/go
module cloud.google.com/go/bigtable
module cloud.google.com/go/firestore
module cloud.google.com/go/bigquery
module cloud.google.com/go/storage
module cloud.google.com/go/datastore
module cloud.google.com/go/pubsub
module cloud.google.com/go/spanner
module cloud.google.com/go/logging
```

The `cloud.google.com/go` is the repository root module. Each other module is
a submodule.

So, if you need to release a change in `bigtable/bttest/inmem.go`, the closest
ancestor module is `cloud.google.com/go/bigtable` - so you should release a new
version of the `cloud.google.com/go/bigtable` submodule.

If you need to release a change in `asset/apiv1/asset_client.go`, the closest
ancestor module is `cloud.google.com/go` - so you should release a new version
of the `cloud.google.com/go` repository root module. Note: releasing
`cloud.google.com/go` has no impact on any of the submodules, and vice-versa.
They are released entirely independently.

# Test failures

If there are any test failures in the Kokoro build, releases are blocked until
the failures have been resolved.

# How to release `cloud.google.com/go`

1. Check for failures in the
   [continuous Kokoro build](http://go/google-cloud-go-continuous). If there are any
   failures in the most recent build, address them before proceeding with the
   release.
1. Navigate to `~/code/gocloud/` and switch to master.
1. `git pull`
1. Run `git tag -l | grep -v beta | grep -v alpha` to see all existing releases.
   The current latest tag `$CV` is the largest tag. It should look something
   like `vX.Y.Z` (note: ignore all `LIB/vX.Y.Z` tags - these are tags for a
   specific library, not the module root). We'll call the current version `$CV`
   and the new version `$NV`.
1. On master, run `git log $CV...` to list all the changes since the last
   release. NOTE: You must manually visually parse out changes to submodules [1]
   (the `git log` is going to show you things in submodules, which are not going
   to be part of your release).
1. Edit `CHANGES.md` to include a summary of the changes.
1. `cd internal/version && go generate && cd -`
1. Commit the changes, push to your fork, and create a PR.
1. Wait for the PR to be reviewed and merged. Once it's merged, and without
   merging any other PRs in the meantime:
   a. Switch to master.
   b. `git pull`
   c. Tag the repo with the next version: `git tag $NV`.
   d. Push the tag to origin:
      `git push origin $NV`
2. Update [the releases page](https://github.com/googleapis/google-cloud-go/releases)
   with the new release, copying the contents of `CHANGES.md`.

# How to release a submodule

We have several submodules, including `cloud.google.com/go/logging`,
`cloud.google.com/go/datastore`, and so on.

To release a submodule:

(these instructions assume we're releasing `cloud.google.com/go/datastore` - adjust accordingly)

1. Check for failures in the
   [continuous Kokoro build](http://go/google-cloud-go-continuous). If there are any
   failures in the most recent build, address them before proceeding with the
   release. (This applies even if the failures are in a different submodule from the one
   being released.)
1. Navigate to `~/code/gocloud/` and switch to master.
1. `git pull`
1. Run `git tag -l | grep datastore | grep -v beta | grep -v alpha` to see all
   existing releases. The current latest tag `$CV` is the largest tag. It
   should look something like `datastore/vX.Y.Z`. We'll call the current version
   `$CV` and the new version `$NV`.
1. On master, run `git log $CV.. -- datastore/` to list all the changes to the
   submodule directory since the last release.
1. Edit `datastore/CHANGES.md` to include a summary of the changes.
1. `cd internal/version && go generate && cd -`
1. Commit the changes, push to your fork, and create a PR.
1. Wait for the PR to be reviewed and merged. Once it's merged, and without
   merging any other PRs in the meantime:
   a. Switch to master.
   b. `git pull`
   c. Tag the repo with the next version: `git tag $NV`.
   d. Push the tag to origin:
      `git push origin $NV`
1. Update [the releases page](https://github.com/googleapis/google-cloud-go/releases)
   with the new release, copying the contents of `datastore/CHANGES.md`.

# Appendix

1: This should get better as submodule tooling matures.
=======
# How to Create a New Release

## Prerequisites

Install [releasetool](https://github.com/googleapis/releasetool).

## Create a release

1. `cd` into the root directory, e.g., `~/go/src/cloud.google.com/go`
1. Checkout the master branch and ensure a clean and up-to-date state.
    ```
    git checkout master
    git pull --tags origin master
    ```
1. Run releasetool to generate a changelog from the last version. Note,
   releasetool will prompt if the new version is a major, minor, or patch
   version.
    ```
    releasetool start --language go
    ```
1. Format the output to match CHANGES.md.
1. Submit a CL with the changes in CHANGES.md. The commit message should look
   like this (where `v0.31.0` is instead the correct version number):
    ```
    all: Release v0.31.0
    ```
1. Wait for approval from all reviewers and then submit the CL.
1. Return to the master branch and pull the release commit.
    ```
    git checkout master
    git pull origin master
    ```
1. Tag the current commit with the new version (e.g., `v0.31.0`)
    ```
    releasetool tag --language go
    ```
1. Publish the tag to GoogleSource (i.e., origin):
    ```
    git push origin $NEW_VERSION
    ```
1. Visit the [releases page][releases] on GitHub and click the "Draft a new
   release" button. For tag version, enter the tag published in the previous
   step. For the release title, use the version (e.g., `v0.31.0`). For the
   description, copy the changes added to CHANGES.md.


[releases]: https://github.com/googleapis/google-cloud-go/releases
>>>>>>> cbc9bb05... fixup add vendor back
