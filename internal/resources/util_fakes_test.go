// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// These various do-nothing objects meet some of the commonly used interfaces
// that support the manager type.

// fakeLogger does nothing.
type fakeLogger struct {
}

func (*fakeLogger) Info(string, ...interface{}) {
}

func (*fakeLogger) Error(error, string, ...interface{}) {
}

// fakeClient does nothing. It just fits in the hole we call the controller
// runtime client interface.  You can use it directly or reuse it as a base for
// your own test cases.
type fakeClient struct {
	scheme *runtime.Scheme
}

func (*fakeClient) Create(
	_ context.Context,
	_ rtclient.Object,
	_ ...rtclient.CreateOption) error {
	return nil
}

func (*fakeClient) Update(
	_ context.Context,
	_ rtclient.Object,
	_ ...rtclient.UpdateOption) error {
	return nil
}

func (*fakeClient) Delete(
	_ context.Context,
	_ rtclient.Object,
	_ ...rtclient.DeleteOption) error {
	return nil
}

func (*fakeClient) DeleteAllOf(
	_ context.Context,
	_ rtclient.Object,
	_ ...rtclient.DeleteAllOfOption) error {
	return nil
}

func (*fakeClient) Get(
	_ context.Context,
	_ types.NamespacedName,
	_ rtclient.Object) error {
	return nil
}

func (*fakeClient) List(
	_ context.Context,
	_ rtclient.ObjectList,
	_ ...rtclient.ListOption) error {
	return nil
}

func (*fakeClient) Patch(
	_ context.Context,
	_ rtclient.Object,
	_ rtclient.Patch,
	_ ...rtclient.PatchOption) error {
	return nil
}

func (*fakeClient) Status() rtclient.StatusWriter {
	return nil
}

func (*fakeClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (c *fakeClient) Scheme() *runtime.Scheme {
	return c.scheme
}
