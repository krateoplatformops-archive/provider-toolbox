package helpers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A ConfigMapKeySelector is a reference to a configmap key in an arbitrary namespace.
type ConfigMapKeySelector struct {
	// Name of the configmap.
	Name string

	// Namespace of the configmap.
	Namespace string

	// The key to select.
	Key string
}

func (s ConfigMapKeySelector) String() string {
	return fmt.Sprintf("%s@%s.%s", s.Key, s.Namespace, s.Name)
}

func DeleteConfigMapValue(ctx context.Context, kc client.Client, ref ConfigMapKeySelector) error {
	cm := &corev1.ConfigMap{}
	err := kc.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, cm)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot get %s configmap", ref.Name)
	}

	delete(cm.Data, ref.Key)

	if len(cm.Data) > 0 {
		return kc.Update(ctx, cm, &client.UpdateOptions{})
	}

	return kc.Delete(ctx, cm, &client.DeleteOptions{})
}

func GetConfigMapValue(ctx context.Context, k client.Client, ref ConfigMapKeySelector) (string, error) {
	cm := &corev1.ConfigMap{}
	err := k.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, cm)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return "", nil
		}
		return "", errors.Wrapf(err, "cannot get %s configmap", ref.Name)
	}

	return string(cm.Data[ref.Key]), nil
}

func SetConfigMapValue(ctx context.Context, kc client.Client, ref ConfigMapKeySelector, val string) error {
	cm := &corev1.ConfigMap{}
	cm.Name = ref.Name
	cm.Namespace = ref.Namespace

	err := kc.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}, cm)
	if err != nil {
		if kerrors.IsNotFound(err) {
			cm.Data = map[string]string{
				ref.Key: val,
			}
			return kc.Create(ctx, cm, &client.CreateOptions{})
		}
		return err
	}

	cm.Data[ref.Key] = val
	return kc.Update(ctx, cm, &client.UpdateOptions{})
}
