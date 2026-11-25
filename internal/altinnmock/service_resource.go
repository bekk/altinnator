package altinnmock

type serviceResourceField struct {
	name string
	// Altinn's value types are the only fields with a default
	defaultValue any
	// The name in Altinn's message when a PUT lacks the field. On a POST, Altinn does not check it
	requiredOnUpdate string
}

// ServiceResource's fields, in Altinn's order. versionId, createdAt and updatedAt are Altinn's to set
var serviceResourceFields = []serviceResourceField{
	{name: "identifier"},
	{name: "version"},
	{name: "title", requiredOnUpdate: "Title"},
	{name: "description", requiredOnUpdate: "Description"},
	{name: "rightDescription"},
	{name: "homepage"},
	{name: "status"},
	{name: "spatial"},
	{name: "contactPoints", requiredOnUpdate: "ContactPoints"},
	{name: "produces"},
	{name: "isPartOf"},
	{name: "thematicAreas"},
	{name: "resourceReferences"},
	{name: "delegable", defaultValue: true},
	{name: "visible", defaultValue: true},
	{name: "hasCompetentAuthority", requiredOnUpdate: "HasCompetentAuthority"},
	{name: "keywords"},
	{name: "accessListMode", defaultValue: "Disabled"},
	{name: "selfIdentifiedUserEnabled", defaultValue: false},
	{name: "enterpriseUserEnabled", defaultValue: false},
	{name: "resourceType", defaultValue: "Default"},
	{name: "availableForType"},
	{name: "authorizationReference"},
	{name: "consentTemplate"},
	{name: "consentText"},
	{name: "consentMetadata"},
	{name: "isOneTimeConsent", defaultValue: false},
}

// What Altinn keeps of a resource it is sent: the fields of its ServiceResource class, without nulls, and with the
// default of a value type that was left out. Altinn also takes field names in any case and enums as numbers, and
// refuses a value of the wrong type. The mock does not, since Altinnator sends what Altinn's API describes
func keepAsAltinn(body map[string]any) map[string]any {
	result := map[string]any{}
	for _, field := range serviceResourceFields {
		if value := body[field.name]; value != nil {
			result[field.name] = value
		} else if field.defaultValue != nil {
			result[field.name] = field.defaultValue
		}
	}
	return result
}

type validationMessages map[string][]string

func (m validationMessages) add(key, message string) { m[key] = append(m[key], message) }

func text(object map[string]any, field string) string {
	value, _ := object[field].(string)
	return value
}

func objects(object map[string]any, field string) []map[string]any {
	items, _ := object[field].([]any)
	var result []map[string]any
	for _, item := range items {
		if itemObject, ok := item.(map[string]any); ok {
			result = append(result, itemObject)
		}
	}
	return result
}
