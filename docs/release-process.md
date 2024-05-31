# samba-operator Release Process

## Preparation

The samba-operator project has a dedicated branch, called `release`, for
release versions. This is done to update files, in particular the tags of the
default containers in `internal/conf`, that control dependencies. Tags are
applied directly to this branch and only this branch.


Prior to tagging, we must update the `release` branch to "contain" all the
latest changes from the `master` branch. We do this by merging `master` into
`release`.
Example:

```
git checkout master
git pull --ff-only
git checkout release
git pull --ff-only
git merge master
# resolve any conflicts
```

Now we need to update the versions of the containers that the operator deploys
by default. Edit `internal/conf/config.go` to update the versions of
the samba-server, samba-metrics, and svcwatch containers if there are new
released versions for these images.

> [!IMPORTANT]
> The projects that the samba-operator includes/depends on must be released
> *before* the samba-operator. As a policy the "SINK" team releases the projects
> as a group. See the section below for guidance with regards to these projects
> and version numbers.

The direct dependencies of the samba-operator are container images from the
samba-container, svcwatch, and smbmetrics projects. These projects also have
dependencies. The resulting recommended release order for these projects are:
1. [sambacc](https://github.com/samba-in-kubernetes/sambacc) - a dependency of samba-container
2. [samba-container](https://github.com/samba-in-kubernetes/samba-container) - a dependency of samba-operator and smbmetrics
3. [smbmetrics](https://github.com/samba-in-kubernetes/smbmetrics) - a dependency of samba-operator
4. [svcwatch](https://github.com/samba-in-kubernetes/svcwatch) - a dependency of samba-operator
5. samba-operator

When selecting a project for release, if we plan on releasing operator v0.6 we
first release, for example, samba-containers v0.6, and so on. If a project is
unchanged we do not need to make a release and the older version number will
continue to be used.  If we do need to make a release that was not part of the
previous group we may *skip* version numbers. For example if component X was
released as v0.3 in along with samba-operator v0.3 but was unchanged for
operator v0.4 we would release it as v0.5 along with samba-operator v0.5 if a
release is needed for that project.

Once the dependency image tags have been updated, commit the change. For example:
```
git commit -s -p -m 'release: use v0.5 tag for samba-server and samba-metrics images'
```

Run `git push` to push the change to the release branch on GitHub, initiating
a CI test run. Verify the release branch build passes the tests.

In the meantime, edit the file `config/manager/manager.yaml` to bump up
the version of the operator image tag (to match the upcoming tag). This file
is used when deploying the operator and must be configured to deploy
the intended release version. Commit the change with a command like so:
```
git commit -s -p -m 'release: use v0.5 tag for samba-operator by default'
```

### Tagging

Assuming the CI has passed the tests, apply a new tag to the `release` branch:
```
git tag -a -m 'Release v0.5' v0.5
```

This creates an annotated tag. Release tags must be annotated tags.

### Build

With the tag recorded in git we can now build the final image for this release.
This tag and the corresponding hash will be recorded in the binary that is
built.

It is very important to ensure that base images are up-to-date.
You can either purge your local system of cached container images or
inspect the `Dockerfile` and purge or explicitly pull each base image.

Run `make image-build`.

Verify that the tag was properly recorded by the binary in the container.
* Run `podman run --rm -it quay.io/samba.org/samba-operator:latest`
* The operator will fail to start because it is not running in Kubernetes.
  However, all you need to verify is that the first log line, with
  "Initializing Manager", contains JSON values that have the correct tag in the
  "Version" field.


For the image that was just built, apply a temporary pre-release tag
to it. Example:
```
podman tag quay.io/samba.org/samba-operator:{latest,v0.5pre1}
```

Log into quay.io.  Push the images to quay.io using the temporary tag. Example:
```
podman push quay.io/samba.org/samba-operator:{latest,v0.5pre1}
```

Wait for the security scan to complete. There shouldn't be any issues if you
properly updated the base images before building. If there are issues and you
are sure you used the newest base images, check the base images on quay.io and
make sure that the number of issues are identical. The security scan can take
some time, while it runs you may want to do other things.


## GitHub Release

When you are satisfied that the tagged version is suitable for release, you
can push the tag to the public repo:
```
git push --follow-tags
```

Draft a new set of release notes. Select the recently pushed tag. Start with
the auto-generated release notes from GitHub (activate the `Generate release
notes` button/link). Add an introductory section (see previous notes for an
example). Add a "Highlights" section if there are any notable features or fixes
in the release. The Highlights section can be skipped if the content of the
release is unremarkable (e.g. few changes occurred since the previous release).

Each release provides a YAML file that can be used to deploy the operator
on a Kubernetes cluster. Generate this file by running:
```
# replace example version with correct version number
kustomize build config/default/ > samba-operator-v0.5-default.yaml
```

Prepare a "downloads" section for the release notes.
Use the following partial snippet as an example. The container details must
reference the image that was pushed to quay.io in an earlier step.
```
## Download

The samba-operator image can be acquired from the quay.io image registry:

* By tag: quay.io/samba.org/samba-operator:v0.5
* By digest: quay.io/samba.org/samba-operator@sha256:040307f53c3f3fd6a5935306f9898858b6c46b7e9c2ae46244e79c2bc42fef0d

### Deploying the operator

This operator can be deployed using the example file samba-operator-v0.5-default.yaml file, attached to this release. Example:

kubectl apply -f samba-operator-v0.5-default.yaml

This is equivalent to checking out the v0.5 tag from the git repository and using the default configuration. Example:

git clone -b v0.5 https://github.com/samba-in-kubernetes/samba-operator
cd samba-operator
kubectl apply -k config/default

```

The tag is pretty obvious - it should match the image tag (minus any pre-release
marker). You can get the digest from the tag using the quay.io UI (do not use
any local digest hashes). Click on the SHA256 link and then copy the full
manifest hash using the UI widget that appears.

Update the "Deploying the operator" section to use the correct version number
and ensure that the YAML file is uploaded to GitHub.

Perform a final round of reviews, as needed, for the release notes and then
publish the release.

Once the release notes are drafted and then either immediately before or after
publishing them, use the quay.io UI to copy each pre-release tag to the "latest"
tag and a final "vX.Y" tag. Delete the temporary pre-release tags using the
quay.io UI as they are no longer needed.


## Announcements

As the samba-operator is typically the final release in the group of "SINK"
projects that are released now is a good time to announce the releases.
Currently the team sends email to the `samba` and `samba-technical` lists at
lists.samba.org.  See the list archives for previous examples.
