package httprequest

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/krateoplatformops/provider-toolbox/apis/httprequest"
	v1alpha1 "github.com/krateoplatformops/provider-toolbox/apis/httprequest/v1alpha1"
	httphelper "github.com/krateoplatformops/provider-toolbox/internal/helpers/http"

	"github.com/krateoplatformops/provider-toolbox/internal/helpers"
)

const (
	errInvalidCRD = "managed resource is not an GetRequest custom resource"

	reasonCannotCreate = "CannotCreateExternalResource"
	reasonCreated      = "CreatedExternalResource"
	reasonDeleted      = "DeletedExternalResource"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.HttpRequestGroupKind)

	log := o.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.HttpRequestGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:     mgr.GetClient(),
			log:      log,
			recorder: recorder,
		}),
		managed.WithLogger(log),
		managed.WithRecorder(event.NewAPIRecorder(recorder)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.HttpRequest{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube       client.Client
	log        logging.Logger
	recorder   record.EventRecorder
	httpClient *http.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.HttpRequest)
	if !ok {
		return nil, errors.New(errInvalidCRD)
	}

	httpClient, err := HttpClientFromProviderConfig(ctx, c.kube, cr)
	if err != nil {
		c.log.Info(fmt.Sprintf("%s: using default HTTP client", err.Error()))
		httpClient = httphelper.ClientFromOpts(httphelper.ClientOpts{Verbose: false, Insecure: false})
	}

	return &external{
		kube: c.kube,
		log:  c.log,
		cli:  httpClient,
		rec:  c.recorder,
	}, nil
}

type external struct {
	kube client.Client
	log  logging.Logger
	cli  *http.Client
	rec  record.EventRecorder
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.HttpRequest)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errInvalidCRD)
	}

	spec := cr.Spec.ForProvider.DeepCopy()

	ref := helpers.ConfigMapKeySelector{
		Key:       spec.WriteResponseToConfigMap.Key,
		Name:      spec.WriteResponseToConfigMap.Name,
		Namespace: spec.WriteResponseToConfigMap.Namespace,
	}
	val, err := helpers.GetConfigMapValue(ctx, e.kube, ref)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if len(val) == 0 {
		e.log.Debug("ConfigMap value does not exist", "cmref", ref, "op", "observe")

		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	inpHash, err := computeSHA1(val)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot compute SHA1: %w", err)
	}
	e.log.Debug("ConfigMap value exist", "cmref", ref, "sha1", inpHash, "op", "observe")

	doer := httprequest.NewDoer(e.kube, e.cli)
	res, err := doer.Do(ctx, spec)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot fetch content: %w", err)
	}

	expHash, err := computeSHA1(string(res))
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot compute SHA1: %w", err)
	}
	e.log.Debug("HTTP remote content digest calculated", "url", spec.URL, "sha1", expHash, "op", "observe")

	if inpHash != expHash {
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	cr.Status.AtProvider = generateObservation(spec)
	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.HttpRequest)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errInvalidCRD)
	}

	cr.SetConditions(xpv1.Creating())

	spec := cr.Spec.ForProvider.DeepCopy()

	doer := httprequest.NewDoer(e.kube, e.cli)
	res, err := doer.Do(ctx, spec)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	mimeType := http.DetectContentType(res)
	e.log.Debug("HTTP remote content fetched", "url", spec.URL, "mimeType", mimeType, "op", "create")
	e.rec.Eventf(cr, corev1.EventTypeNormal, reasonCreated,
		"HTTP remote content fetched (url: %s, mimeType: %s)", spec.URL, mimeType)

	val := string(res)
	ref := helpers.ConfigMapKeySelector{
		Name:      spec.WriteResponseToConfigMap.Name,
		Namespace: spec.WriteResponseToConfigMap.Namespace,
		Key:       spec.WriteResponseToConfigMap.Key,
	}
	err = helpers.SetConfigMapValue(ctx, e.kube, ref, val)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	e.log.Debug("HTTP remote content stored in configMap", "cmRef", ref, "op", "create")
	e.rec.Eventf(cr, corev1.EventTypeNormal, reasonCreated, "HTTP remote content stored in configMap (url: %s, name: %s, namespace: %s, key: %s)",
		spec.URL, ref.Name, ref.Namespace, ref.Key)

	//meta.SetExternalName(cr, fmt.Sprintf("getrequest/%s", hash))

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.HttpRequest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errInvalidCRD)
	}

	spec := cr.Spec.ForProvider.DeepCopy()

	doer := httprequest.NewDoer(e.kube, e.cli)
	res, err := doer.Do(ctx, spec)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	mimeType := http.DetectContentType(res)
	e.log.Debug("HTTP remote content fetched", "url", spec.URL, "mimeType", mimeType, "op", "update")
	e.rec.Eventf(cr, corev1.EventTypeNormal, reasonCreated,
		"HTTP remote content fetched (url: %s, mimeType: %s)", spec.URL, mimeType)

	val := string(res)
	ref := helpers.ConfigMapKeySelector{
		Name:      spec.WriteResponseToConfigMap.Name,
		Namespace: spec.WriteResponseToConfigMap.Namespace,
		Key:       spec.WriteResponseToConfigMap.Key,
	}
	err = helpers.SetConfigMapValue(ctx, e.kube, ref, val)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	e.log.Debug("HTTP remote content stored in configMap", "configMapRef", ref, "op", "create")
	e.rec.Eventf(cr, corev1.EventTypeNormal, reasonCreated,
		"HTTP remote content stored in configMap (url: %s, name: %s, namespace: %s, key: %s)",
		spec.URL, ref.Name, ref.Namespace, ref.Key)

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.HttpRequest)
	if !ok {
		return errors.New(errInvalidCRD)
	}

	cr.SetConditions(xpv1.Deleting())

	spec := cr.Spec.ForProvider.DeepCopy()

	return helpers.DeleteConfigMapValue(ctx, e.kube, helpers.ConfigMapKeySelector{
		Key:       spec.WriteResponseToConfigMap.Key,
		Name:      spec.WriteResponseToConfigMap.Name,
		Namespace: spec.WriteResponseToConfigMap.Namespace,
	})
}

func generateObservation(e *v1alpha1.HttpRequestParams) v1alpha1.HttpRequestObservation {
	return v1alpha1.HttpRequestObservation{
		Target:    helpers.StringPtr("ConfigMap"),
		Name:      helpers.StringPtr(e.WriteResponseToConfigMap.Name),
		Namespace: helpers.StringPtr(e.WriteResponseToConfigMap.Namespace),
		Key:       helpers.StringPtr(e.WriteResponseToConfigMap.Key),
	}
}

func computeSHA1(val string) (string, error) {
	h := sha1.New()
	_, err := h.Write([]byte(val))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
