package statusmanager

import (
	"context"

	"github.com/kartverket/altinnator/api/v1alpha1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateStatus updates the AltinnResource status in Kubernetes, with retry logic and metrics updates.
func UpdateStatus(
	ctx context.Context,
	k8sClient client.Client,
	altinnResource v1alpha1.AltinnResource,
) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &v1alpha1.AltinnResource{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&altinnResource), latest); err != nil {
			return err
		}
		latest.Status = altinnResource.Status
		return k8sClient.Status().Update(ctx, latest)
	})
}
