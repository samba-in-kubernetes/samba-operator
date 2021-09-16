package smbclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
)

// Listing of services from smbclient.
type Listing []string

// Host name or ip address for a share.
type Host string

func (h Host) String() string {
	return "//" + string(h)
}

// Share represents the host and name of an smb share.
type Share struct {
	Host Host
	Name string
}

func (s Share) String() string {
	return fmt.Sprintf("%s/%s", s.Host, s.Name)
}

// Auth represents the values needed to authenticate to a share.
type Auth struct {
	Username string
	Password string
}

// SmbClient is an interface that covers common methods for interacting
// with smbclient when testing.
type SmbClient interface {
	List(ctx context.Context, host Host, auth Auth) (Listing, error)
	Command(ctx context.Context, share Share, auth Auth, cmd []string) error
	CommandOutput(ctx context.Context, share Share, auth Auth, cmd []string) ([]byte, error)
	CacheFlush(ctx context.Context) error
}

// CommandError represents a failed command.
type CommandError struct {
	Desc       string
	Command    []string
	Err        error
	Output     string
	ErrOutput  string
	ExitStatus int
}

func (ce CommandError) Error() string {
	var qcmd string
	// NOTE: this is not "safely" quoted. This is quoted only for the
	// convenience of a human reader.
	if len(ce.Command) > 0 {
		qs := make([]string, len(ce.Command))
		for i := range ce.Command {
			qs[i] = "'" + ce.Command[i] + "'"
		}
		qcmd = fmt.Sprintf("[%s]: ", strings.Join(qs, " "))
	}
	return fmt.Sprintf("%s: %s%s [exit: %d; stdout: %s; stderr: %s]",
		ce.Desc,
		qcmd,
		ce.Err,
		ce.ExitStatus,
		ce.Output,
		ce.ErrOutput,
	)
}

// Unwrap returns the error that generated the CommandError.
func (ce CommandError) Unwrap() error {
	return ce.Err
}

func smbclientWithAuth(auth Auth) []string {
	cmd := []string{"smbclient"}
	if auth.Username != "" && auth.Password != "" {
		cmd = append(cmd, fmt.Sprintf("-U%s%%%s", auth.Username, auth.Password))
	} else if auth.Username != "" {
		cmd = append(cmd, fmt.Sprintf("-U%s", auth.Username))
	}
	return cmd
}

func addSmbClientShare(currCmd []string, share Share) []string {
	return append(currCmd, share.String())
}

func addSmbClientShareCommands(currCmd, shareCmd []string) []string {
	cstring := strings.Join(shareCmd, "; ")
	return append(currCmd, "-c", cstring)
}

type execer interface {
	Call(kube.PodCommand, kube.CommandHandler) error
}

// MustPodExec returns an SmbClient set up to execute commands on the given pod
// and container, using the given kube client.
func MustPodExec(
	tclient *kube.TestClient, namespace, pod, container string) SmbClient {
	// ---
	return &podExecSmbClientCli{
		texec:     kube.NewTestExec(tclient),
		namespace: namespace,
		pod:       pod,
		container: container,
	}
}

type podExecSmbClientCli struct {
	texec     execer
	namespace string
	pod       string
	container string
}

func (c *podExecSmbClientCli) cmdOnPod(
	cmd []string) (*kube.BufferedCommandHandler, error) {
	// ---
	req := kube.PodCommand{
		Command:       cmd,
		Namespace:     c.namespace,
		PodName:       c.pod,
		ContainerName: c.container,
	}
	handler := kube.NewBufferedCommandHandler()
	return handler, c.texec.Call(req, handler)
}

func (*podExecSmbClientCli) List(
	_ context.Context, host Host, auth Auth) (Listing, error) {
	// ---
	cmd := append(smbclientWithAuth(auth), "--list", host.String())
	return nil, fmt.Errorf("not implemented: %v", cmd)
}

func (c *podExecSmbClientCli) Command(
	_ context.Context, share Share, auth Auth, cmd []string) error {
	// ---
	scmd := smbclientWithAuth(auth)
	scmd = addSmbClientShare(scmd, share)
	scmd = addSmbClientShareCommands(scmd, cmd)

	handler, err := c.cmdOnPod(scmd)
	if err != nil {
		exitStatus, _ := kube.ExitCode(err)
		return CommandError{
			Desc:       "failed to execute smbclient command",
			Err:        err,
			Command:    scmd,
			ExitStatus: exitStatus,
			Output:     string(handler.GetStdout()),
			ErrOutput:  string(handler.GetStderr()),
		}
	}
	return nil
}

func (c *podExecSmbClientCli) CommandOutput(
	_ context.Context, share Share, auth Auth, cmd []string) ([]byte, error) {
	// ---
	scmd := smbclientWithAuth(auth)
	scmd = addSmbClientShare(scmd, share)
	scmd = addSmbClientShareCommands(scmd, cmd)

	handler, err := c.cmdOnPod(scmd)
	if err != nil {
		exitStatus, _ := kube.ExitCode(err)
		return nil, CommandError{
			Desc:       "failed to execute smbclient command",
			Err:        err,
			Command:    scmd,
			ExitStatus: exitStatus,
			Output:     string(handler.GetStdout()),
			ErrOutput:  string(handler.GetStderr()),
		}
	}
	return handler.GetStdout(), nil
}

// CacheFlush removes any persistent caches used by smbclient.
func (c *podExecSmbClientCli) CacheFlush(
	_ context.Context) error {
	// ---
	cmd := []string{"rm", "-f", "/var/lib/samba/lock/gencache.tdb"}
	handler, err := c.cmdOnPod(cmd)
	if err != nil {
		exitStatus, _ := kube.ExitCode(err)
		return CommandError{
			Desc:       "failed to flush cache",
			Command:    cmd,
			Err:        err,
			ExitStatus: exitStatus,
			Output:     string(handler.GetStdout()),
			ErrOutput:  string(handler.GetStderr()),
		}
	}
	return nil
}
