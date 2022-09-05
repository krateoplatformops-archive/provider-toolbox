package helpers

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A SecretKeySelector is a reference to a secret key in an arbitrary namespace.
type SecretKeySelector struct {
	// Name of the secret.
	Name string

	// Namespace of the secret.
	Namespace string

	// The key to select.
	Key string
}

func SetSecretValue(ctx context.Context, kc client.Client, ref SecretKeySelector, val string) error {
	cm := &corev1.Secret{}
	cm.Name = ref.Name
	cm.Namespace = ref.Namespace

	err := kc.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}, cm)
	if err != nil {
		if kerrors.IsNotFound(err) {
			cm.StringData = map[string]string{
				ref.Key: val,
			}
			return kc.Create(ctx, cm, &client.CreateOptions{})
		}
		return err
	}

	cm.StringData[ref.Key] = val
	return kc.Update(ctx, cm, &client.UpdateOptions{})
}

func GetSecretValue(ctx context.Context, kc client.Client, ref SecretKeySelector) (string, error) {
	s := &corev1.Secret{}
	if err := kc.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, s); err != nil {
		if kerrors.IsNotFound(err) {
			return "", nil
		}
		return "", errors.Wrapf(err, "cannot get %s secret", ref.Name)
	}

	return string(s.Data[ref.Key]), nil
}

func DeleteSecretValue(ctx context.Context, kc client.Client, ref SecretKeySelector) error {
	cm := &corev1.Secret{}
	err := kc.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, cm)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot get %s secret", ref.Name)
	}

	delete(cm.Data, ref.Key)

	if len(cm.Data) > 0 {
		return kc.Update(ctx, cm, &client.UpdateOptions{})
	}

	return kc.Delete(ctx, cm, &client.DeleteOptions{})
}
