// Package assetclient is identity's minimal HTTP client for the asset service's
// three-step upload (init → blob PUT → finalize). Unlike blog/resource (which
// forward an end-user bearer), identity drives the whole upload server-side on
// behalf of a logged-in user: the IdP mints a short-lived user token (see
// oidc.Manager.MintServiceToken) for init/finalize, and PUTs the bytes itself —
// so the browser never talks to the asset service and no cross-origin CORS or
// presigned-link round-trip to the client is involved.
package assetclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
)

type Client struct{ base string }

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
	UploadURL   string
	UploadToken string
}

type View struct {
	ID     string
	CdnURL string
}

func (c *Client) post(ctx context.Context, bearer, path string, body g.Map) (*gjson.Json, error) {
	cli := g.Client()
	cli.SetHeader("Authorization", "Bearer "+bearer)
	cli.ContentJson()
	resp, err := cli.Post(ctx, c.base+path, body)
	if err != nil {
		return nil, fmt.Errorf("asset service unreachable: %w", err)
	}
	defer resp.Close()
	j := gjson.New(resp.ReadAllString())
	if code := j.Get("code").String(); code != "ok" {
		return nil, fmt.Errorf("asset service error: %s", code)
	}
	return j, nil
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
		UploadURL:   j.Get("data.uploadUrl").String(),
		UploadToken: j.Get("data.uploadToken").String(),
	}, nil
}

func (c *Client) finalize(ctx context.Context, bearer, token string) (View, error) {
	j, err := c.post(ctx, bearer, "/api/v1/assets/finalize", g.Map{"uploadToken": token})
	if err != nil {
		return View{}, err
	}
	return View{
		ID:     j.Get("data.asset.id").String(),
		CdnURL: j.Get("data.asset.cdnUrl").String(),
	}, nil
}

// putBlob streams the bytes to the presigned, token-validated blob URL. The blob
// endpoint authenticates by the URL token (not a bearer), so no auth header here.
func (c *Client) putBlob(ctx context.Context, uploadURL, mime string, data []byte) error {
	cli := g.Client()
	cli.SetHeader("Content-Type", mime)
	resp, err := cli.Put(ctx, uploadURL, data)
	if err != nil {
		return fmt.Errorf("asset blob upload failed: %w", err)
	}
	defer resp.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("asset blob upload failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Delete removes an asset by id (owner-scoped via the bearer). Best-effort:
// used to drop the previous avatar/cover when a new one replaces it.
func (c *Client) Delete(ctx context.Context, bearer, assetID string) error {
	cli := g.Client()
	cli.SetHeader("Authorization", "Bearer "+bearer)
	resp, err := cli.Delete(ctx, c.base+"/api/v1/assets/"+assetID)
	if err != nil {
		return fmt.Errorf("asset service unreachable: %w", err)
	}
	defer resp.Close()
	if code := gjson.New(resp.ReadAllString()).Get("code").String(); code != "ok" {
		return fmt.Errorf("asset delete failed: %s", code)
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
	if err := c.putBlob(ctx, out.UploadURL, in.Mime, data); err != nil {
		return View{}, err
	}
	return c.finalize(ctx, bearer, out.UploadToken)
}
