package smbclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseArgs(t *testing.T) {
	c := &kubectlSmbClientCli{
		kubeconfig: "/tmp/my/kubeconfig",
		pod:        "smbclient-pod",
		namespace:  "foo",
	}
	var cmd []string
	cmd = c.baseArgs(Auth{"bob", "passw0rd"})
	assert.Equal(t,
		[]string{
			"kubectl",
			"--kubeconfig=/tmp/my/kubeconfig",
			"exec",
			"--namespace",
			"foo",
			"-it",
			"smbclient-pod",
			"--",
			"smbclient",
			"-Ubob%passw0rd",
		},
		cmd)
	cmd = c.baseArgs(Auth{"fred", ""})
	assert.Equal(t,
		[]string{
			"kubectl",
			"--kubeconfig=/tmp/my/kubeconfig",
			"exec",
			"--namespace",
			"foo",
			"-it",
			"smbclient-pod",
			"--",
			"smbclient",
			"-Ufred",
		},
		cmd)
}

func TestCmd(t *testing.T) {
	ctx := context.TODO()
	c := &kubectlSmbClientCli{
		kubeconfig: "/tmp/my/kubeconfig",
		pod:        "smbclient-pod",
		namespace:  "foo",
		prefix:     []string{"echo"},
	}
	share := Share{Host("localhost"), "Stuff"}
	cmd := c.cmd(
		ctx,
		Auth{"bob", "passw0rd"},
		[]string{share.String(), "-c", "ls"})
	assert.NotNil(t, cmd)
}

func TestCommand(t *testing.T) {
	ctx := context.TODO()
	c := &kubectlSmbClientCli{
		kubeconfig: "/tmp/my/kubeconfig",
		pod:        "smbclient-pod",
		namespace:  "foo",
		prefix:     []string{"echo"},
	}
	err := c.Command(
		ctx,
		Share{Host("localhost"), "Stuff"},
		Auth{"bob", "passw0rd"},
		[]string{"ls"})
	assert.NoError(t, err)

	c.prefix = []string{"/usr/bin/false"}
	err = c.Command(
		ctx,
		Share{Host("localhost"), "Stuff"},
		Auth{"bob", "passw0rd"},
		[]string{"ls"})
	assert.Error(t, err)
}
