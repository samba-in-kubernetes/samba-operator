package smbclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

type kubectlSmbClientCli struct {
	kubeconfig string
	pod        string
	namespace  string
	prefix     []string
}

func (ksc *kubectlSmbClientCli) kubectlExecArgs() []string {
	cmd := []string{"kubectl"}
	if ksc.kubeconfig != "" {
		cmd = append(cmd, fmt.Sprintf("--kubeconfig=%s", ksc.kubeconfig))
	}
	cmd = append(cmd,
		"exec",
		"--namespace",
		ksc.namespace,
		"-it",
		ksc.pod,
		"--",
	)
	return cmd
}

func (ksc *kubectlSmbClientCli) baseArgs(auth Auth) []string {
	cmd := ksc.kubectlExecArgs()
	cmd = append(cmd, "smbclient")
	if auth.Username != "" && auth.Password != "" {
		cmd = append(cmd, fmt.Sprintf("-U%s%%%s", auth.Username, auth.Password))
	} else if auth.Username != "" {
		cmd = append(cmd, fmt.Sprintf("-U%s", auth.Username))
	}
	return cmd
}

func (ksc *kubectlSmbClientCli) smbclientCmd(
	ctx context.Context, auth Auth, args []string) *exec.Cmd {
	// ---
	argv := append(ksc.prefix, ksc.baseArgs(auth)...)
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = nil // avoid blocking on any input
	return cmd
}

func (ksc *kubectlSmbClientCli) Command(
	ctx context.Context, share Share, auth Auth, shareCmd []string) error {
	// ---
	cstring := strings.Join(shareCmd, "; ")
	cmd := ksc.smbclientCmd(
		ctx, auth, []string{share.String(), "-c", cstring})
	oe, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute smbclient command: %v: %w [stdio: %s]",
			cmd.Args, err, string(oe))
	}
	return nil
}

func (ksc *kubectlSmbClientCli) CommandOutput(
	ctx context.Context, share Share, auth Auth, shareCmd []string) ([]byte, error) {
	// ---
	cstring := strings.Join(shareCmd, "; ")
	cmd := ksc.smbclientCmd(
		ctx, auth, []string{share.String(), "-c", cstring})
	o, err := cmd.Output()
	if err != nil {
		return o, fmt.Errorf("failed to execute smbclient command: %v: %w",
			cmd.Args, err)
	}
	return o, nil
}

func (ksc *kubectlSmbClientCli) List(
	ctx context.Context, host Host, auth Auth) (Listing, error) {
	// ---
	cmd := ksc.smbclientCmd(
		ctx, auth, []string{"--list", host.String()})
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to execute smbclient command: %v: %w",
			cmd.Args, err)
	}
	lst := Listing{} // TODO: put the actual data here
	return lst, nil
}

// CacheFlush removes any persistent caches used by smbclient.
func (ksc *kubectlSmbClientCli) CacheFlush(ctx context.Context) error {
	argv := append(ksc.prefix, ksc.kubectlExecArgs()...)
	//argv = append(argv, "net", "cache", "flush")
	argv = append(argv, "rm", "-f", "/var/lib/samba/lock/gencache.tdb")
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = nil // avoid blocking on any input
	err := cmd.Run()
	return err
}

// MustPodClient returns an SmbClient based on the given pod name and the
// test environment. It panics if the environment is not set up.
func MustPodClient(namespace, pod string) SmbClient {
	// this is a tad hacky, but in an effort not to boil the ocean at this
	// very minute I'd rather do this than build a lot more comprehensive
	// configuration for the test utilities.
	kc := os.Getenv("KUBECONFIG")
	if pod == "" {
		panic(fmt.Errorf("pod is unset"))
	}
	return &kubectlSmbClientCli{
		kubeconfig: kc,
		pod:        pod,
		namespace:  namespace,
	}
}
