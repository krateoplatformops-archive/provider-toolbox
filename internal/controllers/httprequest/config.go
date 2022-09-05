package httprequest

import (
	"context"
	"net/http"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/krateoplatformops/provider-toolbox/apis/v1alpha1"
	"github.com/krateoplatformops/provider-toolbox/internal/helpers"
	httphelper "github.com/krateoplatformops/provider-toolbox/internal/helpers/http"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func HttpClientFromProviderConfig(ctx context.Context, kc client.Client, mg resource.Managed) (*http.Client, error) {
	if mg.GetProviderConfigReference() == nil {
		return nil, errors.New("providerConfigRef is not given")
	}

	opts, err := useProviderConfig(ctx, kc, mg)
	if err != nil {
		return nil, err
	}

	return httphelper.ClientFromOpts(opts), nil
}

func useProviderConfig(ctx context.Context, kc client.Client, mg resource.Managed) (httphelper.ClientOpts, error) {
	pc := &v1alpha1.ProviderConfig{}
	err := kc.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc)
	if err != nil {
		return httphelper.ClientOpts{}, errors.Wrap(err, "cannot get referenced Provider")
	}

	t := resource.NewProviderConfigUsageTracker(kc, &v1alpha1.ProviderConfigUsage{})
	err = t.Track(ctx, mg)
	if err != nil {
		return httphelper.ClientOpts{}, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	return httphelper.ClientOpts{
		Verbose:  helpers.BoolValue(pc.Spec.Verbose),
		Insecure: helpers.BoolValue(pc.Spec.Insecure),
	}, nil
}
