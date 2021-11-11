// Package dnsclient helps check on the state of DNS from within a pod
package dnsclient

import (
	"fmt"
	"strings"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

// CommandError from smbclient. XXX Hack alert! Clean this up.
type CommandError = smbclient.CommandError

type execer interface {
	Call(kube.PodCommand, kube.CommandHandler) error
}

// DNSClient can be used to resolve DNS queries from a particular
// viewpoint (pod).
type DNSClient interface {
	HostAddress(string) (string, error)
}

// MustPodExec returns a DNSClient set up to resolve queries from
// the specified pod & container.
func MustPodExec(
	tclient *kube.TestClient, namespace, pod, container string) DNSClient {
	// ---
	return &podDNSClient{
		texec:     kube.NewTestExec(tclient),
		namespace: namespace,
		pod:       pod,
		container: container,
	}
}

type podDNSClient struct {
	texec     execer
	namespace string
	pod       string
	container string
}

func (z *podDNSClient) HostAddress(h string) (string, error) {
	cmd := []string{"getent", "hosts", h}
	req := kube.PodCommand{
		Command:       cmd,
		Namespace:     z.namespace,
		PodName:       z.pod,
		ContainerName: z.container,
	}
	handler := kube.NewBufferedCommandHandler()
	err := z.texec.Call(req, handler)
	if err != nil {
		exitStatus, _ := kube.ExitCode(err)
		return "", CommandError{
			Desc:       "failed to execute getent command",
			Err:        err,
			Command:    cmd,
			ExitStatus: exitStatus,
			Output:     string(handler.GetStdout()),
			ErrOutput:  string(handler.GetStderr()),
		}
	}
	s := string(handler.GetStdout())
	parts := strings.Split(s, " ")
	if len(parts) < 1 {
		return "", CommandError{
			Desc:    "unexpected getent output",
			Err:     fmt.Errorf("invalid output"),
			Command: cmd,
			Output:  s,
		}
	}
	return parts[0], nil
}
