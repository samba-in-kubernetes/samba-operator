package kube

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// I dislike how the controller runtime creeps into every aspect of our code,
// especially that since I've been using it it has made breaking API changes.
// However, the client is convenient and it is already imported by our "api"
// package and thus immediately part of our dependency tree upon import of the
// api package. This means making a great deal of effort to avoid using in the
// tests is somewhat wasted.
// That said, this file is written in a way to abstract/hide the fact that we
// are using the controller runtime client so that if we do make changes in the
// future to stop using it directly in the test code, we can do so more easily.
// -- JJM

// TypedClientScheme is the scheme specific to our typed object client.
var TypedClientScheme = runtime.NewScheme()

// APIObject can be used to stand in for any API object.
type APIObject interface {
	metav1.Object
	runtime.Object
}

// TypedObjectClient can be used to interact with non-core typed objects
// dynamically. Don't forget to register your types with our scheme.
type TypedObjectClient interface {
	Get(ctx context.Context, name types.NamespacedName, obj APIObject) error
	Create(ctx context.Context, obj APIObject) error
	Delete(ctx context.Context, obj APIObject) error
	Update(ctx context.Context, obj APIObject) error
}

type runtimeTypedObjectClient struct {
	rtc rtclient.Client
}

// Get an object.
func (c *runtimeTypedObjectClient) Get(
	ctx context.Context, name types.NamespacedName, obj APIObject) error {
	return c.rtc.Get(ctx, name, obj)
}

// Create an object.
func (c *runtimeTypedObjectClient) Create(
	ctx context.Context, obj APIObject) error {
	return c.rtc.Create(ctx, obj)
}

// Delete an object.
func (c *runtimeTypedObjectClient) Delete(
	ctx context.Context, obj APIObject) error {
	return c.rtc.Delete(ctx, obj)
}

// Update an existing object.
func (c *runtimeTypedObjectClient) Update(
	ctx context.Context, obj APIObject) error {
	return c.rtc.Update(ctx, obj)
}

// TypedObjectClient returns a client that can be easily used with go object
// types that are not native to kubernetes.
func (tc *TestClient) TypedObjectClient() TypedObjectClient {
	c, err := rtclient.New(tc.cfg, rtclient.Options{Scheme: TypedClientScheme})
	utilruntime.Must(err)
	return &runtimeTypedObjectClient{c}
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(TypedClientScheme))
}
