# Podman Container Tools Contributing Guide

Please first read our organization wide contributing guide here: https://github.com/podman-container-tools/community/blob/main/CONTRIBUTING.md.

Sections specific to Skopeo are listed below.

## container-libs testing

When new PRs for [podman-container-tools/container-libs](https://github.com/podman-container-tools/container-libs) break `skopeo` (i.e. `podman-container-tools/container-libs` ci tests fail in `image-skopeo`):

- create out a new branch in your `skopeo` checkout and switch to it
- find out the version of `podman-container-tools/container-libs` you want to use and note its commit ID. You might also want to use a fork of `podman-container-tools/container-libs`, in that case note its repo
- use `go get -d github.com/$REPO/container-libs/image/v5@$COMMIT_ID` to download the right version. The command will fetch the dependency and then fail because of a conflict in `go.mod`, this is expected. Note the pseudo-version (eg. `v5.13.1-0.20210707123201-50afbf0a326`)
- use `go mod edit -replace=go.podman.io/image/v5=github.com/$REPO/container-libs/image/v5@$PSEUDO_VERSION` to add a replacement line to `go.mod` (e.g. `replace go.podman.io/image/v5 => github.com/moio/container-libs/image/v5 v5.13.1-0.20210707123201-50afbf0a3262`)
- run `make vendor`
- make any other necessary changes in the skopeo repo (e.g. add other dependencies now required by `podman-container-tools/container-libs`, or update skopeo for changed `podman-container-tools/container-libs` API)
- optionally add new integration tests to the skopeo repo
- submit the resulting branch as a skopeo PR, marked “DO NOT MERGE”
- iterate until tests pass and the PR is reviewed
- then the original `podman-container-tools/container-libs` PR can be merged, disregarding its `make test-skopeo` failure
- as soon as possible after that, in the skopeo PR, use `go mod edit -dropreplace=go.podman.io/image/v5` to remove the `replace` line in `go.mod`
- run `make vendor`
- update the skopeo PR with the result, drop the “DO NOT MERGE” marking
- after tests complete successfully again, merge the skopeo PR
