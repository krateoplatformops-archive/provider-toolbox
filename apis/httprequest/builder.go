package httprequest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/krateoplatformops/provider-toolbox/apis/httprequest/v1alpha1"
	"github.com/krateoplatformops/provider-toolbox/internal/helpers"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HttpRequestDoer interface {
	Do(ctx context.Context, spec *v1alpha1.HttpRequestParams) ([]byte, error)
}

var _ HttpRequestDoer = (*httpRequestDoer)(nil)

func NewDoer(kubeClient client.Client, httpClient *http.Client) HttpRequestDoer {
	return &httpRequestDoer{
		KubeClient: kubeClient,
		HttpClient: httpClient,
	}
}

type httpRequestDoer struct {
	KubeClient client.Client
	HttpClient *http.Client
	Spec       *v1alpha1.HttpRequestParams
}

func (r *httpRequestDoer) Do(ctx context.Context, spec *v1alpha1.HttpRequestParams) ([]byte, error) {
	uri, err := url.Parse(spec.URL)
	if err != nil {
		return nil, err
	}

	params := uri.Query()
	if len(spec.Params) > 0 {
		for _, el := range spec.Params {
			val, err := resolveNamedValue(ctx, r.KubeClient, el)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error resolving custom query string param (%s): %s\n", el.Name, err.Error())
				continue
			}
			params.Add(el.Name, val)
		}
		uri.RawQuery = params.Encode()
	}

	method := helpers.StringOrDefault(spec.Method, http.MethodGet)
	req, err := http.NewRequestWithContext(ctx, *method, uri.String(), nil)
	if err != nil {
		return nil, err
	}

	if len(spec.Headers) > 0 {
		for _, el := range spec.Headers {
			val, err := resolveNamedValue(ctx, r.KubeClient, el)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error resolving custom http header (%s): %s\n", el.Name, err.Error())
				continue
			}
			req.Header.Set(http.CanonicalHeaderKey(el.Name), val)
		}
	}

	rsp, err := r.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode < http.StatusOK || rsp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unknown error, status code: %d", rsp.StatusCode)
	}

	if rsp.Body == nil {
		return nil, nil
	}
	defer rsp.Body.Close()

	dat, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	return dat, nil
}

func resolveNamedValue(ctx context.Context, kc client.Client, ref v1alpha1.NamedValue) (val string, err error) {
	if ref.ConfigMapRef != nil {
		val, err = helpers.GetConfigMapValue(ctx, kc, helpers.ConfigMapKeySelector{
			Name:      ref.ConfigMapRef.Name,
			Namespace: ref.ConfigMapRef.Namespace,
			Key:       ref.ConfigMapRef.Key,
		})
	}

	if (len(val) == 0) && (ref.SecretRef != nil) {
		val, err = helpers.GetSecretValue(ctx, kc, helpers.SecretKeySelector{
			Name:      ref.SecretRef.Name,
			Namespace: ref.SecretRef.Namespace,
			Key:       ref.SecretRef.Key,
		})
	}

	if (len(val) == 0) && ref.Value != nil {
		val = helpers.StringValue(ref.Value)
	}

	if len(val) == 0 {
		return "", err
	}

	val = strings.TrimSpace(val)
	if ref.Format != nil {
		val = fmt.Sprintf(helpers.StringValue(ref.Format), val)
	}

	return val, err
}
