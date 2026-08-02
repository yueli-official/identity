// Package assetclient is identity's minimal HTTP client for the asset service's
// three-step upload (init → blob PUT → finalize). Unlike blog/resource (which
// forward an end-user bearer), identity drives the whole upload server-side on
// behalf of a logged-in user: the IdP mints a short-lived user token (see
// oidc.Manager.MintServiceToken) for init/finalize, and PUTs the bytes itself —
// so the browser never talks to the asset service and no cross-origin CORS or
// presigned-link round-trip to the client is involved.
package assetclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	foundationhttpclient "github.com/yueli-official/foundation/go/httpclient"
)

type Client struct{ base string }

var mediaKeyPattern = regexp.MustCompile(`^[0-9A-Za-z]{20,32}$`)

// New builds a client rooted at the asset service base URL (e.g. http://localhost:8082).
func New(baseURL string) *Client { return &Client{base: strings.TrimRight(baseURL, "/")} }

// InitInput / View mirror the asset upload-init request and finalized asset view.
type InitInput struct {
	Filename   string
	Mime       string
	Size       int64
	Category   string
	Visibility string
}

type initOutput struct {
	UploadURL     string
	UploadToken   string
	UploadHeaders map[string]string
}

type View struct {
	ID       string
	MediaKey string
}

func (c *Client) post(ctx context.Context, bearer, path string, body g.Map) (*gjson.Json, error) {
	raw, _ := json.Marshal(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("asset request invalid: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+bearer)
	request.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("asset service unreachable: %w", err)
	}
	defer resp.Body.Close()
	out, err := foundationhttpclient.DecodeJSON[map[string]any](resp, foundationhttpclient.Limits{})
	if err != nil {
		return nil, fmt.Errorf("asset service error: %w", err)
	}
	return gjson.New(out), nil
}

func (c *Client) uploadInit(ctx context.Context, bearer string, in InitInput) (initOutput, error) {
	j, err := c.post(ctx, bearer, "/api/v1/assets/upload-init", g.Map{
		"filename": in.Filename, "mime": in.Mime, "size": in.Size,
		"category": in.Category, "siteKey": "account", "profileKey": in.Category,
		"visibility": in.Visibility,
	})
	if err != nil {
		return initOutput{}, err
	}
	return initOutput{
		UploadURL:     j.Get("uploadUrl").String(),
		UploadToken:   j.Get("uploadToken").String(),
		UploadHeaders: stringMap(j.Get("uploadHeaders").Map()),
	}, nil
}

func stringMap(raw map[string]any) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = g.NewVar(v).String()
	}
	return out
}

func (c *Client) finalize(ctx context.Context, bearer, token string) (View, error) {
	j, err := c.post(ctx, bearer, "/api/v1/assets/finalize", g.Map{"uploadToken": token})
	if err != nil {
		return View{}, err
	}
	view := View{
		ID:       j.Get("asset.id").String(),
		MediaKey: j.Get("asset.mediaKey").String(),
	}
	if view.ID == "" || !mediaKeyPattern.MatchString(view.MediaKey) {
		return View{}, fmt.Errorf("asset finalize returned an invalid media reference")
	}
	return view, nil
}

// putBlob streams the bytes to the presigned, token-validated blob URL. The blob
// endpoint authenticates by the URL token (not a bearer), so no auth header here.
func (c *Client) putBlob(ctx context.Context, uploadURL, mime string, headers map[string]string, data []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("asset blob upload request invalid: %w", err)
	}
	request.Header.Set("Content-Type", mime)
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("asset blob upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("asset blob upload failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Delete removes an asset by id (owner-scoped via the bearer). Best-effort:
// used to drop the previous avatar/cover when a new one replaces it.
func (c *Client) Delete(ctx context.Context, bearer, assetID string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/api/v1/assets/"+assetID, nil)
	if err != nil {
		return fmt.Errorf("asset delete request invalid: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("asset service unreachable: %w", err)
	}
	defer resp.Body.Close()
	if _, err := foundationhttpclient.DecodeJSON[any](resp, foundationhttpclient.Limits{}); err != nil {
		return fmt.Errorf("asset delete failed: %w", err)
	}
	return nil
}

// Upload runs the full init → PUT → finalize cycle and returns the public asset
// view. bearer is the user-scoped token minted by the IdP.
func (c *Client) Upload(ctx context.Context, bearer string, in InitInput, data []byte) (View, error) {
	out, err := c.uploadInit(ctx, bearer, in)
	if err != nil {
		return View{}, err
	}
	if err := c.putBlob(ctx, out.UploadURL, in.Mime, out.UploadHeaders, data); err != nil {
		return View{}, err
	}
	return c.finalize(ctx, bearer, out.UploadToken)
}
