package altinn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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

var ErrNotFound = errors.New("not found in Altinn")

type ResourceRegistry interface {
	GetMaskinportenSchema(ctx context.Context, id string) (*MaskinportenSchema, error)
	SetMaskinportenSchemaPolicy(ctx context.Context, id string) error
}

// AccessTokenProvider gives the Altinn token a write needs.
type AccessTokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
	// Refused is told when Altinn refuses token, so the next call does not get it again.
	Refused(token string)
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
	tokens     AccessTokenProvider
}

var _ ResourceRegistry = (*Client)(nil)

// NewClient takes the tokens writes need from tokens. A client that only reads may pass nil.
func NewClient(baseURL string, httpClient *http.Client, tokens AccessTokenProvider) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient, tokens: tokens}
}

// GetMaskinportenSchema fails with ErrNotFound when Kartverket has no MaskinportenSchema with this id, also when Altinn
// has a resource with the id that is of another type or owned by another organisation.
func (c *Client) GetMaskinportenSchema(ctx context.Context, id string) (*MaskinportenSchema, error) {
	r, err := c.getResource(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.ResourceType != maskinportenSchemaType || r.HasCompetentAuthority.Organization != kartverketOrganization {
		return nil, fmt.Errorf("MaskinportenSchema %s: %w", id, ErrNotFound)
	}
	return r.toMaskinportenSchema(id), nil
}

// SetMaskinportenSchemaPolicy gives the MaskinportenSchema the policy every MaskinportenSchema gets, in place of any
// policy it has. It fails with ErrNotFound, and changes nothing, when Kartverket has no MaskinportenSchema with this id.
func (c *Client) SetMaskinportenSchemaPolicy(ctx context.Context, id string) error {
	if c.tokens == nil {
		return errors.New("setting a policy needs an AccessTokenProvider")
	}
	// Without this check, the policy could replace the one on another kind of Kartverket resource
	if _, err := c.GetMaskinportenSchema(ctx, id); err != nil {
		return err
	}

	policyURL, err := url.JoinPath(c.baseURL, resourcePath, id, "policy")
	if err != nil {
		return fmt.Errorf("building the policy URL of resource %s: %w", id, err)
	}
	body, contentType, err := policyForm(maskinportenSchemaPolicy(id))
	if err != nil {
		return fmt.Errorf("building the policy of resource %s: %w", id, err)
	}
	if err := c.write(ctx, http.MethodPut, policyURL, contentType, body, http.StatusCreated); err != nil {
		return fmt.Errorf("setting the policy of resource %s in Altinn: %w", id, err)
	}
	return nil
}

// Altinn takes the policy as a file in the form field policyFile
func policyForm(policy string) ([]byte, string, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="policyFile"; filename="policy.xml"`)
	header.Set("Content-Type", "application/xml")
	part, err := form.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write([]byte(policy)); err != nil {
		return nil, "", err
	}
	if err := form.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), form.FormDataContentType(), nil
}

func (c *Client) write(ctx context.Context, method, target, contentType string, body []byte, success int) error {
	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		return fmt.Errorf("getting a token for Altinn: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("calling Altinn: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == success {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		c.tokens.Refused(token)
	}
	return errors.New(responded(response))
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
