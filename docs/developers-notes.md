
# Developer's Guide & Tips

## Running a custom operator

As noted in the [README](../README.md) the operator can be deployed using a custom image. This section elaborates on that.

The makefile is aware of two variables (env vars or directly used by `make`):
* TAG - specify a custom tag for your container image
* IMG - specify a custom image (repository & tag) for your image

In the following examples, we assume you will be testing using your own
container repository and thus will use a fully specified `IMG` variable.

```bash
# set the IMG var for subsequent commands
export IMG=registry.example.com/myuser/samba-operator:test
# build the container image
make image-build
# push the image to a container registry
make container-push
# populate k8s with CRDs and launches the operator.
# assumes kubectl is set up and works
make deploy
```

Behind the scenes this makefile uses `kustomize` and loading resources into
the kubernetes cluster is handled via the YAML files in `./config`.
There is a special makefile target `set-image` that runs kustomize commands
in order to set a YAML file in that directory to use *your* container image
rather than a default one. This target is automatically used by `make deploy`
but can be used manually if needed.

Please do not check changes made by kustomize to kustomization.yaml files
in to git history.

## Testing with a custom operator

To verify the test scripts are testing the right image, a rule checks that
the deployed operator in the kubernetes cluster matches what the test
expects. The test's expectation is controlled via an environment variable
`SMBOP_TEST_EXPECT_MANAGER_IMG`. To ensure the tests match the custom
container image you used this variable should also be set. Example:

```bash
# configure the tests to check for a given container
export SMBOP_TEST_EXPECT_MANAGER_IMG="${IMG}"
# Run the tests
./tests/test.sh
```

## Using a custom samba server container image

The operator itself will create pods running various samba-server container
images. Certain aspects of the operator, such as the container image to use
for the samba server are configurable. There are a few ways to configure the
operator, as it can read it's config from TOML or YAML files as well as
it's command line or environment variables. The following example uses
environment variables set in the operator's own pod spec.

We will set the environment variables using kustomize rules in the file
`./config/manager/kustomization.yaml`. Add the following to that file:

```
patches:
- patch: |-
    - op: add
      path: /spec/template/spec/containers/0/env/-
      value:
        name: "SAMBA_OP_SMBD_CONTAINER_IMAGE"
        value: "registry.example.com/myuser/samba-server:experiment"
  target:
    kind: Deployment
```

For multiple values, more than the "op" in the embedded yaml patch can be
specified. Using "add" with "/spec/template/spec/containers/0/env/-" means to
append the value to the end of the list at "env".

You can also add your own "kustomize' patches and other rules.  See the
[kustomize docs](https://kubectl.docs.kubernetes.io/references/kustomize/) for
more information on using kustomize. You can also set other environment
variables in a similar manner.


Please do not check changes made by kustomize to kustomization.yaml files
in to git history.


## Debugging the samba containers

Similar to using a custom container image the operator accepts a configuration
value for samba debugging that will be passed on to the containers the
operator creates. This parameter is `samba-debug-level` in configuration
files and `SAMBA_OP_SAMBA_DEBUG_LEVEL` in the evnironment. The value should
be a numeral 0 through 10 specified as a *string*.

Example setting the variable via the `./config/manager/kustomization.yaml` file:

```
patches:
- patch: |-
    - op: add
      path: /spec/template/spec/containers/0/env/-
      value:
        name: "SAMBA_OP_SAMBA_DEBUG_LEVEL"
        value: "8"
  target:
    kind: Deployment
```
