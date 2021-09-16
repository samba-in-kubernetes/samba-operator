package smbclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
)

func TestCommandBuilders(t *testing.T) {
	var cmd []string
	cmd = smbclientWithAuth(Auth{"bob", "passw0rd"})
	assert.Equal(t,
		[]string{
			"smbclient",
			"-Ubob%passw0rd",
		},
		cmd)
	cmd = smbclientWithAuth(Auth{"fred", ""})
	assert.Equal(t,
		[]string{
			"smbclient",
			"-Ufred",
		},
		cmd)

	cmd = addSmbClientShare(cmd, Share{Host("localhost"), "foo"})
	assert.Equal(t,
		[]string{
			"smbclient",
			"-Ufred",
			"//localhost/foo",
		},
		cmd)

	cmd = addSmbClientShareCommands(cmd, []string{"ls", "put cat.jpg"})
	assert.Equal(t,
		[]string{
			"smbclient",
			"-Ufred",
			"//localhost/foo",
			"-c",
			"ls; put cat.jpg",
		},
		cmd)
}

func TestCommandError(t *testing.T) {
	var ce CommandError
	t.Run("minimal", func(t *testing.T) {
		ce = CommandError{
			Desc: "foobar",
			Err:  fmt.Errorf("baz"),
		}
		assert.Equal(t,
			"foobar: baz [exit: 0; stdout: ; stderr: ]",
			ce.Error())
	})
	t.Run("stdout", func(t *testing.T) {
		ce = CommandError{
			Desc:   "foobar",
			Err:    fmt.Errorf("baz"),
			Output: "quux blat",
		}
		assert.Equal(t,
			"foobar: baz [exit: 0; stdout: quux blat; stderr: ]",
			ce.Error())
	})
	t.Run("stderr", func(t *testing.T) {
		ce = CommandError{
			Desc:      "foobar",
			Err:       fmt.Errorf("baz"),
			ErrOutput: "womble woo",
		}
		assert.Equal(t,
			"foobar: baz [exit: 0; stdout: ; stderr: womble woo]",
			ce.Error())
	})
	t.Run("everything", func(t *testing.T) {
		ce = CommandError{
			Desc:       "foobar",
			Command:    []string{"smbclient", "-Usambauser%samba", "//localhost/foo"},
			Err:        fmt.Errorf("baz"),
			Output:     "quux blat",
			ErrOutput:  "womble woo",
			ExitStatus: 2,
		}
		assert.Equal(t,
			"foobar: ['smbclient' '-Usambauser%samba' '//localhost/foo']:"+
				" baz [exit: 2; stdout: quux blat; stderr: womble woo]",
			ce.Error())
	})
	t.Run("unwrap", func(t *testing.T) {
		e := fmt.Errorf("its me")
		ce = CommandError{
			Desc: "foobar",
			Err:  e,
		}
		assert.Equal(t, e, ce.Unwrap())
	})
}

func TestPodExecList(t *testing.T) {
	pe := &podExecSmbClientCli{}
	ctx := context.TODO()
	_, err := pe.List(
		ctx,
		Host("localhost"),
		Auth{"bob", "passw0rd"},
	)
	assert.Error(t, err)
}

type phonyExecer struct {
	err       error
	output    string
	errOutput string
}

func (p phonyExecer) Call(_ kube.PodCommand, h kube.CommandHandler) error {
	_, _ = h.Stdout().Write([]byte(p.output))
	_, _ = h.Stderr().Write([]byte(p.errOutput))
	return p.err
}

func TestPodExecCommand(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		pe := &podExecSmbClientCli{
			namespace: "default",
			pod:       "smbclient",
			container: "",
			texec: phonyExecer{
				err:       nil,
				output:    "great",
				errOutput: "",
			}}
		ctx := context.TODO()
		err := pe.Command(
			ctx,
			Share{Host("localhost"), "Stuff"},
			Auth{"bob", "passw0rd"},
			[]string{"ls"},
		)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		pe := &podExecSmbClientCli{
			namespace: "default",
			pod:       "smbclient",
			container: "",
			texec: phonyExecer{
				err:       fmt.Errorf("kaboom"),
				output:    "sorry",
				errOutput: "I broke",
			}}
		ctx := context.TODO()
		err := pe.Command(
			ctx,
			Share{Host("localhost"), "Stuff"},
			Auth{"bob", "passw0rd"},
			[]string{"ls"},
		)
		assert.Error(t, err)
		assert.Equal(t,
			`failed to execute smbclient command:`+
				` ['smbclient' '-Ubob%passw0rd' '//localhost/Stuff' '-c' 'ls']:`+
				` kaboom [exit: 1; stdout: sorry; stderr: I broke]`,
			err.Error())
	})
}

func TestPodExecCommandOutput(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		pe := &podExecSmbClientCli{
			namespace: "default",
			pod:       "smbclient",
			container: "",
			texec: phonyExecer{
				err:       nil,
				output:    "great",
				errOutput: "",
			}}
		ctx := context.TODO()
		o, err := pe.CommandOutput(
			ctx,
			Share{Host("localhost"), "Stuff"},
			Auth{"bob", "passw0rd"},
			[]string{"ls"},
		)
		assert.NoError(t, err)
		assert.Equal(t, "great", string(o))
	})

	t.Run("error", func(t *testing.T) {
		pe := &podExecSmbClientCli{
			namespace: "default",
			pod:       "smbclient",
			container: "",
			texec: phonyExecer{
				err:       fmt.Errorf("kaboom"),
				output:    "sorry",
				errOutput: "I broke",
			}}
		ctx := context.TODO()
		o, err := pe.CommandOutput(
			ctx,
			Share{Host("localhost"), "Stuff"},
			Auth{"bob", "passw0rd"},
			[]string{"ls"},
		)
		assert.Error(t, err)
		assert.Equal(t,
			`failed to execute smbclient command:`+
				` ['smbclient' '-Ubob%passw0rd' '//localhost/Stuff' '-c' 'ls']:`+
				` kaboom [exit: 1; stdout: sorry; stderr: I broke]`,
			err.Error())
		assert.Nil(t, o)
	})
}

func TestPodExecCacheFlush(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		pe := &podExecSmbClientCli{
			namespace: "default",
			pod:       "smbclient",
			container: "",
			texec: phonyExecer{
				err:       nil,
				output:    "great",
				errOutput: "",
			}}
		ctx := context.TODO()
		err := pe.CacheFlush(ctx)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		pe := &podExecSmbClientCli{
			namespace: "default",
			pod:       "smbclient",
			container: "",
			texec: phonyExecer{
				err:       fmt.Errorf("kaboom"),
				output:    "sorry",
				errOutput: "I broke",
			}}
		ctx := context.TODO()
		err := pe.CacheFlush(ctx)
		assert.Error(t, err)
		assert.Equal(t,
			`failed to flush cache:`+
				` ['rm' '-f' '/var/lib/samba/lock/gencache.tdb']:`+
				` kaboom [exit: 1; stdout: sorry; stderr: I broke]`,
			err.Error())
	})
}
