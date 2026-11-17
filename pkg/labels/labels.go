package labels

const (
	ManagedByLabelKey  = "app.kubernetes.io/managed-by"
	ControllerLabelKey = "altinnator.kartverket.no/controller"
	TypeLabelKey       = "type"

	ManagedByLabelValue                = "altinnator"
	AltinnResourceControllerLabelValue = "altinnresource"
	TypeLabelValue                     = "altinnator.kartverket.no"
)

// AltinnResourceStandardLabels returns the set of labels applied to every resource created by Altinnator for the
// AltinnResource controller.
func AltinnResourceStandardLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:  ManagedByLabelValue,
		ControllerLabelKey: AltinnResourceControllerLabelValue,
		TypeLabelKey:       TypeLabelValue,
	}
}
