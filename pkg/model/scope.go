package model

import (
	"fmt"

	"github.com/kartverket/altinnator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Scope struct {
	AltinnResource         v1alpha1.AltinnResource
	DummyData              map[string]string
	Descendants            []Descendant[client.Object]
	ValidationErrorMessage *string
}

type Descendant[T client.Object] struct {
	ID             string
	Object         T
	ErrorMessage   *string
	SuccessMessage *string
}

func (s *Scope) GetErrors() []string {
	var errs []string
	if s != nil {
		for _, d := range s.Descendants {
			if d.ErrorMessage != nil {
				errs = append(errs, *d.ErrorMessage)
			}
		}
	}
	return errs
}

func (s *Scope) ReplaceDescendant(
	obj client.Object,
	errorMessage *string,
	successMessage *string,
	resourceKind, resourceName string,
) {
	if s != nil {
		expectedID := GetID(resourceKind, resourceName)
		for i, d := range s.Descendants {
			if d.ID == expectedID {
				s.Descendants[i] = Descendant[client.Object]{
					ID:             expectedID,
					Object:         obj,
					ErrorMessage:   errorMessage,
					SuccessMessage: successMessage,
				}
				return
			}
		}
		s.Descendants = append(s.Descendants, Descendant[client.Object]{
			ID:             GetID(resourceKind, resourceName),
			Object:         obj,
			ErrorMessage:   errorMessage,
			SuccessMessage: successMessage,
		})
	}
}

func GetID(resourceKind, resourceName string) string {
	return fmt.Sprintf("%s-%s", resourceKind, resourceName)
}
