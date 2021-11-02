
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

## Specifying custom configuration parameters

The operator supports a number of configuration parameters that
influence the behvaior of the operator itself. These parameters
can be specified via a configuration file in TOML or YAML formats,
via the operator's command line, or via environment variables.
Environment variables are the simplest approach and is discussed below.
These settings should not need to be changed for typical use, however
some of them can be useful when developing the operator or when
testing/debugging it.

Our stock deployment assumes environment variables stored in a ConfigMap
(base name "controller-cfg"). You can use kustomize to set these values
using a configMapGenerator. We recommend placing the generator in an
overlay kustomization.yaml file such as `config/default/kustomization.yaml`.
We also support a "shortcut" location for developers at
`config/developer/kustomization.yaml`. This location will be automatically
used by the Makefile when variable DEVELOPER is set, for example
`make DEVELOPER=1 deploy`. Files in the `config/developer` directory
are ignored by git and are a good place for setting changes that
are specific to you.

An example of custom configuration parameters using kustomize:

```
configMapGenerator:
- behavior: merge
  literals:
  - "SAMBA_OP_SAMBA_DEBUG_LEVEL=10"
  - "SAMBA_OP_CLUSTER_SUPPORT=ctdb-is-experimental"
  - "SOMETHING_ELSE=55"
  name: controller-cfg
  namespace: system
```

Append the above section to the appropriate kustomization.yaml file.  See the
kustomize documentation for more information on how you can set environment
variables in this config map.  See the
[kustomize docs](https://kubectl.docs.kubernetes.io/references/kustomize/)
for more information on how you can set environment variables in the ConfigMap
or how you can use kustomize in general.

Some specific examples follow. Remember that these examples as well as other
variables can be combined in a single ConfigMap.

### Using a custom samba server container image

The operator itself will create pods running various samba-server container
images. We will set the environment variables using kustomize to alter
the container image used for samba server instances:

```
configMapGenerator:
- behavior: merge
  literals:
  - "SAMBA_OP_SMBD_CONTAINER_IMAGE=registry.example.com/myuser/samba-server:experiment"
  name: controller-cfg
  namespace: system
```

### Debugging the samba containers

The operator accepts a configuration value for samba debugging that will be
passed on to the containers the operator creates. This parameter is
`samba-debug-level` in configuration files and `SAMBA_OP_SAMBA_DEBUG_LEVEL` in
the evnironment. The value should be a numeral 0 through 10 specified as a
*string*:


```
configMapGenerator:
- behavior: merge
  literals:
  - "SAMBA_OP_SAMBA_DEBUG_LEVEL=8"
  name: controller-cfg
  namespace: system
```
