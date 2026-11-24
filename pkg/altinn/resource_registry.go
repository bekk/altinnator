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

	"github.com/kartverket/altinnator/internal/altinn/auth"
)

const (
	resourcePath           = "/resourceregistry/api/v1/resource"
	maskinportenSchemaType = "MaskinportenSchema"
	maskinportenScopeType  = "MaskinportenScope"
	statusWithdrawn        = "Withdrawn"
)

// Altinn's rule for an identifier. Altinn gets it in the path, so it also keeps "/" and ".." out of the URL
var resourceIDPattern = regexp.MustCompile(`^[a-z0-9_-]{4,}$`)

// Altinn's own routes next to the resources, which win over a resource with the same id
var fixedRoutes = []string{"resourcelist", "export", "search", "updated", "changes", "orgs"}

var (
	ErrNotFound = errors.New("not found in Altinn")
	ErrTaken    = errors.New("taken by another resource in Altinn")
	ErrRejected = errors.New("rejected by Altinn")
)

type ResourceRegistry interface {
	GetMaskinportenSchema(ctx context.Context, id string) (*MaskinportenSchema, error)
	CreateMaskinportenSchema(ctx context.Context, id string, content MaskinportenSchemaContent) error
	WithdrawMaskinportenSchema(ctx context.Context, id string) error

	SetMaskinportenSchemaPolicy(ctx context.Context, id string) error
}

type AccessTokenProvider interface {
	Token(ctx context.Context) (auth.Token, error)
	// Refused is told when Altinn refuses token, so the next call does not get it again.
	Refused(token auth.Token)
}

type MaskinportenSchema struct {
	ResourceID            string
	Scopes                []string
	Title                 MultilingualText
	Description           MultilingualText
	DelegationDescription MultilingualText
	Status                string
}

type MaskinportenSchemaContent struct {
	Scopes                []string
	Title                 MultilingualText
	Description           MultilingualText
	DelegationDescription MultilingualText
	Homepage              string
	ContactPoints         []ContactPoint
	Keywords              []Keyword
}

type MultilingualText struct {
	En string `json:"en"`
	Nb string `json:"nb"`
	Nn string `json:"nn"`
}

type ContactPoint struct {
	Category    string `json:"category"`
	Email       string `json:"email"`
	Telephone   string `json:"telephone"`
	ContactPage string `json:"contactPage"`
}

type Keyword struct {
	Word     string `json:"word"`
	Language string `json:"language"`
}

type Client struct {
	baseURL       string
	httpClient    *http.Client
	orgNumber     string
	orgCode       string
	tokenProvider AccessTokenProvider
}

var _ ResourceRegistry = (*Client)(nil)

func NewClient(
	baseURL string, httpClient *http.Client, orgNumber, orgCode string, tokenProvider AccessTokenProvider,
) *Client {
	return &Client{
		baseURL:       baseURL,
		httpClient:    httpClient,
		orgNumber:     orgNumber,
		orgCode:       orgCode,
		tokenProvider: tokenProvider,
	}
}

// GetMaskinportenSchema fails with ErrNotFound when Altinn has no resource with this id, and with ErrTaken when the
// resource with this id is of another type or owned by another organisation.
func (c *Client) GetMaskinportenSchema(ctx context.Context, id string) (*MaskinportenSchema, error) {
	r, err := c.getOwnedMaskinportenSchema(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toMaskinportenSchema(id), nil
}

func (c *Client) getOwnedMaskinportenSchema(ctx context.Context, id string) (*resource, error) {
	r, err := c.getResource(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.ResourceType != maskinportenSchemaType || r.HasCompetentAuthority.Organization != c.orgNumber {
		return nil, fmt.Errorf("resource %s is a %s owned by %s: %w",
			id, r.ResourceType, r.HasCompetentAuthority.Organization, ErrTaken)
	}
	return r, nil
}

// CreateMaskinportenSchema creates the MaskinportenSchema in Altinn, owned by the organisation, delegable and visible.
// It fails with ErrTaken when Altinn has a resource with this id, and with ErrRejected when Altinn will not take it.
func (c *Client) CreateMaskinportenSchema(ctx context.Context, id string, content MaskinportenSchemaContent) error {
	if c.tokenProvider == nil {
		return errors.New("creating a resource needs an AccessTokenProvider")
	}
	// Altinn would take an id that is one of its routes, but the resource could then never be read or changed
	if err := checkID(id); err != nil {
		return err
	}

	body, err := json.Marshal(c.newResourceFrom(id, content))
	if err != nil {
		return fmt.Errorf("building resource %s: %w", id, err)
	}
	resourcesURL, err := url.JoinPath(c.baseURL, resourcePath)
	if err != nil {
		return fmt.Errorf("building the URL of the resources: %w", err)
	}
	status, err := c.write(ctx, http.MethodPost, resourcesURL, "application/json", body, http.StatusCreated)
	if status == http.StatusConflict {
		return fmt.Errorf("resource %s: %w", id, ErrTaken)
	}
	if err != nil {
		return fmt.Errorf("creating resource %s in Altinn: %w", id, err)
	}
	return nil
}

// SetMaskinportenSchemaPolicy gives the MaskinportenSchema the policy every MaskinportenSchema gets, in place of any
// policy it has. Like GetMaskinportenSchema, it fails with ErrNotFound or ErrTaken, and changes nothing, when the
// organisation has no MaskinportenSchema with this id.
func (c *Client) SetMaskinportenSchemaPolicy(ctx context.Context, id string) error {
	if c.tokenProvider == nil {
		return errors.New("setting a policy needs an AccessTokenProvider")
	}
	// Altinn's policy endpoint doesn't check the type, so this keeps the policy off the organisation's other resources
	if _, err := c.getOwnedMaskinportenSchema(ctx, id); err != nil {
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
	if _, err := c.write(ctx, http.MethodPut, policyURL, contentType, body, http.StatusCreated); err != nil {
		return fmt.Errorf("setting the policy of resource %s in Altinn: %w", id, err)
	}
	return nil
}

func (c *Client) WithdrawMaskinportenSchema(ctx context.Context, id string) error {
	if c.tokenProvider == nil {
		return errors.New("withdrawing a resource needs an AccessTokenProvider")
	}
	r, err := c.getOwnedMaskinportenSchema(ctx, id)
	if err != nil {
		return err
	}
	// Altinn makes a new version even when nothing changes
	if r.Status == statusWithdrawn && !r.Delegable && !r.Visible {
		return nil
	}

	body, err := withdrawn(r.raw)
	if err != nil {
		return fmt.Errorf("building withdrawn resource %s: %w", id, err)
	}
	resourceURL, err := url.JoinPath(c.baseURL, resourcePath, id)
	if err != nil {
		return fmt.Errorf("building the URL of resource %s: %w", id, err)
	}
	if _, err := c.write(ctx, http.MethodPut, resourceURL, "application/json", body, http.StatusOK); err != nil {
		return fmt.Errorf("withdrawing resource %s in Altinn: %w", id, err)
	}
	return nil
}

func withdrawn(raw []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	fields["delegable"] = json.RawMessage("false")
	fields["visible"] = json.RawMessage("false")
	fields["status"] = json.RawMessage(`"` + statusWithdrawn + `"`)
	return json.Marshal(fields)
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

// write returns the status Altinn answered with, or 0 when it did not answer
func (c *Client) write(ctx context.Context, method, target, contentType string, body []byte, success int) (int, error) {
	token, err := c.tokenProvider.Token(ctx)
	if err != nil {
		return 0, fmt.Errorf("getting a token for Altinn: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Authorization", "Bearer "+token.Reveal())

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("calling Altinn: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == success {
		return response.StatusCode, nil
	}
	answer, _ := io.ReadAll(io.LimitReader(response.Body, maxAnswer))
	if rejected(response.StatusCode, answer) {
		return response.StatusCode, fmt.Errorf("%s: %w", responded(response.StatusCode, answer), ErrRejected)
	}
	if response.StatusCode == http.StatusUnauthorized {
		c.tokenProvider.Refused(token)
	}
	return response.StatusCode, errors.New(responded(response.StatusCode, answer))
}

// Altinn answers 400 to a resource it will not take, and 401 with InvalidPrefix to scopes that do not start with one of
// the organisation's prefixes. That 401 is about the scopes, not the token.
func rejected(status int, answer []byte) bool {
	if status == http.StatusBadRequest {
		return true
	}
	var problem struct {
		Errors map[string]json.RawMessage `json:"errors"`
	}
	if status != http.StatusUnauthorized || json.Unmarshal(answer, &problem) != nil {
		return false
	}
	_, invalidPrefix := problem.Errors["InvalidPrefix"]
	return invalidPrefix
}

func checkID(id string) error {
	if !resourceIDPattern.MatchString(id) {
		return fmt.Errorf("expected a resource id of 4 or more of a-z, 0-9, _ and -, but was %q", id)
	}
	if slices.Contains(fixedRoutes, id) {
		return fmt.Errorf("resource id %q is one of Altinn's routes, so Altinn can't return the resource", id)
	}
	return nil
}

func (c *Client) getResource(ctx context.Context, id string) (*resource, error) {
	if err := checkID(id); err != nil {
		return nil, err
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
		answer, _ := io.ReadAll(io.LimitReader(response.Body, maxAnswer))
		return nil, fmt.Errorf("getting resource %s from Altinn: %s", id, responded(response.StatusCode, answer))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading resource %s from Altinn: %w", id, err)
	}
	r := resource{raw: body}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parsing resource %s from Altinn: %w", id, err)
	}
	return &r, nil
}

const maxAnswer = 1 << 16

// The answer is capped, so an error page in HTML does not flood the log
func responded(status int, answer []byte) string {
	if strings.TrimSpace(string(answer)) == "" {
		return fmt.Sprintf("responded %d", status)
	}
	if len(answer) > 1024 {
		answer = answer[:1024]
	}
	return fmt.Sprintf("responded %d: %s", status, answer)
}

type resource struct {
	ResourceType          string              `json:"resourceType"`
	Title                 MultilingualText    `json:"title"`
	Description           MultilingualText    `json:"description"`
	RightDescription      MultilingualText    `json:"rightDescription"`
	Status                string              `json:"status"`
	Delegable             bool                `json:"delegable"`
	Visible               bool                `json:"visible"`
	ResourceReferences    []resourceReference `json:"resourceReferences"`
	HasCompetentAuthority competentAuthority  `json:"hasCompetentAuthority"`
	raw                   []byte
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

type newResource struct {
	Identifier            string                 `json:"identifier"`
	Title                 MultilingualText       `json:"title"`
	Description           MultilingualText       `json:"description"`
	RightDescription      MultilingualText       `json:"rightDescription"`
	Homepage              string                 `json:"homepage,omitempty"`
	ContactPoints         []ContactPoint         `json:"contactPoints,omitempty"`
	Keywords              []Keyword              `json:"keywords,omitempty"`
	ResourceReferences    []newResourceReference `json:"resourceReferences"`
	Delegable             bool                   `json:"delegable"`
	Visible               bool                   `json:"visible"`
	ResourceType          string                 `json:"resourceType"`
	HasCompetentAuthority newCompetentAuthority  `json:"hasCompetentAuthority"`
	AccessListMode        string                 `json:"accessListMode"`
}

type newResourceReference struct {
	ReferenceSource string `json:"referenceSource"`
	Reference       string `json:"reference"`
	ReferenceType   string `json:"referenceType"`
}

type newCompetentAuthority struct {
	Organization string `json:"organization"`
	OrgCode      string `json:"orgcode"`
}

func (c *Client) newResourceFrom(id string, content MaskinportenSchemaContent) newResource {
	references := make([]newResourceReference, 0, len(content.Scopes))
	for _, scope := range content.Scopes {
		references = append(references, newResourceReference{
			ReferenceSource: "Default",
			Reference:       scope,
			ReferenceType:   maskinportenScopeType,
		})
	}
	return newResource{
		Identifier:            id,
		Title:                 content.Title,
		Description:           content.Description,
		RightDescription:      content.DelegationDescription,
		Homepage:              content.Homepage,
		ContactPoints:         content.ContactPoints,
		Keywords:              content.Keywords,
		ResourceReferences:    references,
		Delegable:             true,
		Visible:               true,
		ResourceType:          maskinportenSchemaType,
		HasCompetentAuthority: newCompetentAuthority{Organization: c.orgNumber, OrgCode: c.orgCode},
		AccessListMode:        "Disabled",
	}
}
