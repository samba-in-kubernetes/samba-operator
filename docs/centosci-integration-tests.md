### Run multinode kubernetes cluster tests with experimental CTDB support

For every pull request on samba-operator repository, an integration test run
is triggered on a [Jenkins](https://jenkins-samba.apps.ocp.cloud.ci.centos.org/view/SINK/)
based pipeline from CentOS CI infrastructure which is supposed to show up in
the list of GitHub checks along the following format:

*centos-ci/sink-clustered/mini-k8s-latest*

- Use the following text as GitHub comment to trigger a test run manually:

  `/test centos-ci/sink-clustered/mini-k8s-latest`

- Or if you want to re-run:

  `/retest centos-ci/sink-clustered/mini-k8s-latest`

- With more tests added to the matrix one could trigger everything at once with:

  `/test all` or `/retest all`
