package kube

import (
	"context"
	"errors"
	"io"
	"os"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// InputSource interfaces are used to specify k8s resources.
type InputSource interface {
	Open() (io.ReadCloser, error)
	GetNamespace() string
}

// FileSource selects a file path and an optional namespace where
// the file contains one or more k8s resource.
type FileSource struct {
	Path      string
	Namespace string
}

// Open returns a file opened for reading.
func (f FileSource) Open() (io.ReadCloser, error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// GetNamespace returns the specified namespace.
func (f FileSource) GetNamespace() string {
	return f.Namespace
}

// DirectSource interfaces are used to specify k8s resources directly
// from a ReadCloser stream.
type DirectSource struct {
	Source    io.ReadCloser
	Namespace string
}

// Open returns the source.
func (d DirectSource) Open() (io.ReadCloser, error) {
	return d.Source, nil
}

// GetNamespace returns the specified namespace.
func (d DirectSource) GetNamespace() string {
	return d.Namespace
}

// CreateFromFile creates new resources given a (yaml) file input.
// It returns an error if the resource already exists.
func (tc *TestClient) CreateFromFile(
	ctx context.Context, src InputSource) ([]types.NamespacedName, error) {
	// ---
	n := []types.NamespacedName{}
	objs, err := getUnstructuredObjects(src)
	if err != nil {
		return n, err
	}
	for _, u := range objs {
		newu, err := tc.dynCreate(ctx, src.GetNamespace(), u)
		if err != nil {
			return n, err
		}
		n = append(n, types.NamespacedName{
			Namespace: newu.GetNamespace(),
			Name:      newu.GetName(),
		})
	}
	return n, nil
}

// CreateFromFileIfMissing creates new resources given a (yaml) file input.
// It does not return an error if the resource already exists.
func (tc *TestClient) CreateFromFileIfMissing(
	ctx context.Context, src InputSource) ([]types.NamespacedName, error) {
	// ---
	n := []types.NamespacedName{}
	objs, err := getUnstructuredObjects(src)
	if err != nil {
		return n, err
	}
	for _, u := range objs {
		newu, err := tc.dynCreate(ctx, src.GetNamespace(), u)
		if kerrors.IsAlreadyExists(err) {
			continue
		}
		if err != nil {
			return n, err
		}
		n = append(n, types.NamespacedName{
			Namespace: newu.GetNamespace(),
			Name:      newu.GetName(),
		})
	}
	return n, nil
}

// DeleteResourceMatchingFile deletes a resource given a (yaml) file input.
// The resource with the matching group-version-kind and name will be
// removed if it exists.
func (tc *TestClient) DeleteResourceMatchingFile(
	ctx context.Context, src InputSource) error {
	// ---
	objs, err := getUnstructuredObjects(src)
	if err != nil {
		return err
	}
	for _, u := range objs {
		err := tc.dynDelete(ctx, src.GetNamespace(), u)
		if kerrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func getUnstructuredObjects(src InputSource) (objects []*unstructured.Unstructured, err error) {
	r, err := src.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil && err == nil {
			// it is unfortunate that error-on-close is only captured if
			// no other errors occur, but I don't want to lose what is
			// likely to be the more interesting error. And fmt.Errorf
			// doesn't support multiple %w's and I don't want to pull in
			// dependencies or write a lot of code for just this.
			err = closeErr
		}
	}()

	objects = []*unstructured.Unstructured{}
	dec := yaml.NewYAMLOrJSONDecoder(r, 1024)
	for {
		obj := &unstructured.Unstructured{}
		err = dec.Decode(obj)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func (tc *TestClient) dynClient() (
	dynamic.Interface, *restmapper.DeferredDiscoveryRESTMapper, error) {
	// ---
	dc, err := discovery.NewDiscoveryClientForConfig(tc.cfg)
	if err != nil {
		return nil, nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(
		memory.NewMemCacheClient(dc))
	dyn, err := dynamic.NewForConfig(tc.cfg)
	if err != nil {
		return nil, nil, err
	}
	return dyn, mapper, nil
}

func (tc *TestClient) dynCreate(
	ctx context.Context, ns string, u *unstructured.Unstructured) (
	*unstructured.Unstructured, error) {
	// ---
	dyn, mapper, err := tc.dynClient()
	if err != nil {
		return nil, err
	}

	gvk := u.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	dr := dynamicResource(dyn, mapping, chooseNamespace(ns, u.GetNamespace()))
	newu, err := dr.Create(
		ctx,
		u,
		metav1.CreateOptions{FieldManager: "samba-operator-tests"},
	)
	return newu, err
}

func (tc *TestClient) dynDelete(
	ctx context.Context, ns string, u *unstructured.Unstructured) error {
	// ---
	dyn, mapper, err := tc.dynClient()
	if err != nil {
		return err
	}

	gvk := u.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	dr := dynamicResource(dyn, mapping, chooseNamespace(ns, u.GetNamespace()))
	err = dr.Delete(
		ctx,
		u.GetName(),
		metav1.DeleteOptions{},
	)
	return err
}

func chooseNamespace(ns ...string) string {
	var n string
	for _, n = range ns {
		if n != "" {
			break
		}
	}
	if n == "" {
		return "default"
	}
	return n
}

func dynamicResource(dyn dynamic.Interface, mapping *meta.RESTMapping, namespace string) dynamic.ResourceInterface {
	dr := dyn.Resource(mapping.Resource)
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		return dr.Namespace(namespace)
	}
	return dr
}
