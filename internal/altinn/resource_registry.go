package altinn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	resourcePath           = "/resourceregistry/api/v1/resource"
	maskinportenSchemaType = "MaskinportenSchema"
	maskinportenScopeType  = "MaskinportenScope"
	kartverketOrganization = "971040238"
)

// Altinn's rule for an identifier. Altinn gets it in the path, so it also keeps "/" and ".." out of the URL
var resourceIDPattern = regexp.MustCompile(`^[a-z0-9_-]{4,}$`)

// Altinn's own routes next to the resources, which win over a resource with the same id
var fixedRoutes = []string{"resourcelist", "export", "search", "updated", "changes", "orgs"}

var (
	ErrNotFound = errors.New("not found in Altinn")
	ErrTaken    = errors.New("taken by another resource in Altinn")
)

type ResourceRegistry interface {
	GetMaskinportenSchema(ctx context.Context, id string) (*MaskinportenSchema, error)
}

type MaskinportenSchema struct {
	ResourceID            string
	Scopes                []string
	Title                 MultilingualText
	Description           MultilingualText
	DelegationDescription MultilingualText
	Status                string
}

type MultilingualText struct {
	En string `json:"en"`
	Nb string `json:"nb"`
	Nn string `json:"nn"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

var _ ResourceRegistry = (*Client)(nil)

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

// GetMaskinportenSchema fails with ErrNotFound when Altinn has no resource with this id, and with ErrTaken when the
// resource with this id is of another type or owned by another organisation.
func (c *Client) GetMaskinportenSchema(ctx context.Context, id string) (*MaskinportenSchema, error) {
	r, err := c.getResource(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.ResourceType != maskinportenSchemaType || r.HasCompetentAuthority.Organization != kartverketOrganization {
		return nil, fmt.Errorf("resource %s is a %s owned by %s: %w",
			id, r.ResourceType, r.HasCompetentAuthority.Organization, ErrTaken)
	}
	return r.toMaskinportenSchema(id), nil
}

func (c *Client) getResource(ctx context.Context, id string) (*resource, error) {
	if !resourceIDPattern.MatchString(id) {
		return nil, fmt.Errorf("expected a resource id of 4 or more of a-z, 0-9, _ and -, but was %q", id)
	}
	if slices.Contains(fixedRoutes, id) {
		return nil, fmt.Errorf("resource id %q is one of Altinn's routes, so Altinn can't return the resource", id)
	}

	resourceURL, err := url.JoinPath(c.baseURL, resourcePath, id)
	if err != nil {
		return nil, fmt.Errorf("building the URL of resource %s: %w", id, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building the request for resource %s: %w", id, err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("getting resource %s from Altinn: %w", id, err)
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("resource %s: %w", id, ErrNotFound)
	default:
		return nil, fmt.Errorf("getting resource %s from Altinn: %s", id, responded(response))
	}

	var r resource
	if err := json.NewDecoder(response.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("parsing resource %s from Altinn: %w", id, err)
	}
	return &r, nil
}

// The body is capped, so an error page in HTML does not flood the log
func responded(response *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if strings.TrimSpace(string(body)) == "" {
		return fmt.Sprintf("responded %d", response.StatusCode)
	}
	return fmt.Sprintf("responded %d: %s", response.StatusCode, body)
}

type resource struct {
	ResourceType          string              `json:"resourceType"`
	Title                 MultilingualText    `json:"title"`
	Description           MultilingualText    `json:"description"`
	RightDescription      MultilingualText    `json:"rightDescription"`
	Status                string              `json:"status"`
	ResourceReferences    []resourceReference `json:"resourceReferences"`
	HasCompetentAuthority competentAuthority  `json:"hasCompetentAuthority"`
}

type resourceReference struct {
	Reference     string `json:"reference"`
	ReferenceType string `json:"referenceType"`
}

type competentAuthority struct {
	Organization string `json:"organization"`
}

func (r *resource) toMaskinportenSchema(id string) *MaskinportenSchema {
	var scopes []string
	for _, reference := range r.ResourceReferences {
		if reference.ReferenceType == maskinportenScopeType {
			scopes = append(scopes, reference.Reference)
		}
	}
	return &MaskinportenSchema{
		ResourceID:            id,
		Scopes:                scopes,
		Title:                 r.Title,
		Description:           r.Description,
		DelegationDescription: r.RightDescription,
		Status:                r.Status,
	}
}
