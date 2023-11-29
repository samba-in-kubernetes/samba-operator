# Contribution Guide

1. [Prerequisites](#prerequisites)
2. [Build](#build)
3. [Check](#check)
4. [Testing](#testing)
5. [Pull-request](#pull-request)
6. [License](#license)

Thank you for your time and effort to help us improve samba-operator.
Here are a few steps to get started, with reference to some useful
resources.


## Prerequisites

Development effort takes place using Linux environment and requires at
minimum:

1. [Go 1.19](https://golang.org/dl/) installed
2. [GitHub](https://github.com/) account
3. Development tools: git, make, and podman or docker
4. Testing: [minikube](https://minikube.sigs.k8s.io)
5. [Quay](https://quay.io/) account (optional)

Development collaboration takes place via github's samba-in-kubernetes
([SINK](https://github.com/samba-in-kubernetes/)) pages and facilities.

The top-level [Makefile](../Makefile) is the entry point for various
build commands. Few utilities are required during the build process,
either for bootstrapping the project, building it or checking the
validity of the source code itself. If not present in your `$PATH`,
those tools may be installed locally to the project (under the
local `.bin` directory) using:

```sh
  $ make build-tools
  ...
  $ ls .bin/
```

## Build

The following section describes how to build and test samba-operator.
A more detailed description of this process may be found in the
[developer-notes](./developer-notes.md) document.

Building the samba-operator is a straightforward action with
`make build`. Upon successful build, the output binary is written to
`bin/manager`:

```sh
  $ make build
  ...
  $ ls bin/
  manager
```

Note that the manager binary alone is not typically executed in a
Kubernetes cluster. For that we generally build a container image. To
build an OCI container image including the manager binary, run:

```sh
  $ make image-build
```

The name of this image is controlled by the `IMG` and `TAG` variables
and may be customized to fit your needs. A developer may override those
as well as some other default Makefile settings via the `devel.mk` file
at the project's root directory:

```sh
  $ cp devel.mk.sample devel.mk
  $ vim devel.mk
```

## Check

Before submitting any pull request, a developer must ensure that
his code satisfies the following minimum requirements:

1. Go code conforms to the project's standards.
2. YAML files conform to the project's standards.
3. Unit-tests pass without errors.

Use the following commands to check and test your code:

```sh
  $ make check
  $ make test
```

If any of those make rules fail they should first be fixed before any
other developer will review your patches. Please note that those steps
are integrated into the project's CI workflow, and therefore must pass
successfully.

## Testing

Ideally, before submitting any pull request, a developer should test
any code changes locally. Local clusters are simpler to debug and, if
one has not had code merged to the project before, it does not require
approval from the project maintainers. A prerequisite to running the
samba-operator test suite is a running Kubernetes cluster. Typically
this will be [minikube](https://minikube.sigs.k8s.io) but other
Kubernetes implementations can suffice.

An example using minikube with a private container image follows:

```sh
  $ export IMG=quay.io/my-quay-username/samba-operator:latest
  $ echo IMG=${IMG} >> devel.mk
  $ make image-build
  $ make container-push
  ...
  $ minikube start \
      --driver=kvm2 \
      --bootstrapper kubeadm \
      --disk-size 32g \
      --memory 8192 \
      --cpus 2 \
      --insecure-registry "10.0.0.0/24" \
      --nodes=3 -\
      --extra-disks=2 \
      --feature-gates=EphemeralContainers=true
  ...
  $ # Ensure cluster has basic functionality:
  $ kubectl get nodes
  $ kubectl get pods -A
  ...
  $ # Deploy your image:
  $ make deploy
  $ kubectl get pods -n samba-operator-system
  ...
  $ # Start an AD DC inside k8s to support domain logins in test suite
  $ ./tests/test-deploy-ad-server.sh
  $ # Run the test-suite
  $ ./tests/test.sh
  ...
  $ # in case of failure:
  $ ./tests/post-test-info.sh
  ...
  $ make undeploy
```

## Pull-request
Finally, you may submit your changes via samba-operator github
[pull-request](https://github.com/samba-in-kubernetes/samba-operator/pulls).
Before submitting a pull request, make sure that you provided a valid
git commit message, which allows other developers to easily review your
code. In particular, ensure that each git commit include the
followings:

1. A short topic line (up to 72 characters) with the name of the
sub-component this commit refers to.

2. A commit-message body which explain why and how this change is
needed, in a clear and informative manner.

3. The author details, including a valid e-mail address and a
*Signed-off-by* entry.

Validate your commits with [gitlint](https://jorisroovers.com/gitlint/).
Even better, you may use the following make rule:
```sh
  $ make check-gitlint
```

## License

Samba-operator is an open-source project, developed under the
[Apache-2.0](https://www.apache.org/licenses/LICENSE-2.0) license. We
appreciate any contribution which helps us improve this code base.

