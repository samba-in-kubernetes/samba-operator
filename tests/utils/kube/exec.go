package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	corev1api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	podsResourceName   = "pods"
	execSubResource    = "exec"
	containerParamName = "container"
)

// PodCommand identifies a pod and a container and a command to run.
type PodCommand struct {
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
}

// BufferedCommandHandler uses byte buffers to save command output.
type BufferedCommandHandler struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// NewBufferedCommandHandler returns a new BufferedCommandHandler.
func NewBufferedCommandHandler() *BufferedCommandHandler {
	b1 := make([]byte, 0, 2048)
	b2 := make([]byte, 0, 2048)
	return &BufferedCommandHandler{
		stdout: bytes.NewBuffer(b1),
		stderr: bytes.NewBuffer(b2),
	}
}

// Stdout returns the stdout writer for this request.
func (bpc *BufferedCommandHandler) Stdout() io.Writer {
	return bpc.stdout
}

// Stderr returns the stderr writer for this request.
func (bpc *BufferedCommandHandler) Stderr() io.Writer {
	return bpc.stderr
}

// GetStdout returns the stdout data.
func (bpc *BufferedCommandHandler) GetStdout() []byte {
	return bpc.stdout.Bytes()
}

// GetStderr returns the stderr data.
func (bpc *BufferedCommandHandler) GetStderr() []byte {
	return bpc.stderr.Bytes()
}

// CommandHandler interfaces provide parameters for running a command on a pod.
type CommandHandler interface {
	Stdout() io.Writer
	Stderr() io.Writer
}

// TestExec is used to execute CommandRequests.
type TestExec struct {
	tclient *TestClient
}

// NewTestExec returns a new TestExec pointer.
func NewTestExec(tclient *TestClient) *TestExec {
	return &TestExec{tclient}
}

// Call executes the requested command on the pod.
func (te *TestExec) Call(pc PodCommand, ch CommandHandler) error {
	execOpts := corev1api.PodExecOptions{
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
		Container: pc.ContainerName,
		Command:   pc.Command,
	}
	streamOpts := remotecommand.StreamOptions{
		Stdout: ch.Stdout(),
		Stderr: ch.Stderr(),
	}

	// create the request that will be used to produce a url for spdy
	req := te.tclient.Clientset().CoreV1().RESTClient().
		Post().
		Resource(podsResourceName).
		Namespace(pc.Namespace).
		Name(pc.PodName).
		SubResource(execSubResource).
		Param(containerParamName, pc.ContainerName).
		VersionedParams(&execOpts, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(
		te.tclient.cfg,
		http.MethodPost,
		req.URL())
	if err != nil {
		return fmt.Errorf(
			"failed to set up executor (pod:%s/%s container:%s): %w",
			pc.Namespace,
			pc.PodName,
			pc.ContainerName,
			err)
	}
	err = exec.Stream(streamOpts)
	if err != nil {
		return fmt.Errorf(
			"failed executing command (pod:%s/%s container:%s): %w",
			pc.Namespace,
			pc.PodName,
			pc.ContainerName,
			err)
	}
	return nil
}

// ExitCode returns an non-successful exit code value for a given error, and a
// boolean set to true if the exit code is based on the error itself.
func ExitCode(err error) (int, bool) {
	var xe interface {
		error
		ExitStatus() int
	}
	if ok := errors.As(err, &xe); ok {
		return xe.ExitStatus(), ok
	}
	return 1, false
}
