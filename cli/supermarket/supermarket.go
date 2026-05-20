// Package supermarket implements Chef Supermarket upload workflows.
package supermarket

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/config"
	localcookbook "github.com/tas50/cinc-cli/cli/cookbook"
)

const DefaultSite = "https://supermarket.chef.io"

// Client uploads cookbooks to one Chef Supermarket endpoint.
type Client struct {
	base       *url.URL
	userID     string
	key        *rsa.PrivateKey
	httpClient *http.Client
	clock      func() time.Time
}

// ShareOptions controls a cookbook share operation.
type ShareOptions struct {
	Cookbook     string
	Category     string
	CookbookPath string
	DryRun       bool
}

// ShareResult describes the work done by Share.
type ShareResult struct {
	Cookbook    string   `json:"cookbook"`
	Category    string   `json:"category"`
	Uploaded    bool     `json:"uploaded"`
	Status      int      `json:"status"`
	Tarball     string   `json:"tarball"`
	TarballSize int      `json:"tarball_size"`
	Files       []string `json:"files,omitempty"`
}

// New builds a Supermarket client using the configured profile identity.
func New(profile config.Profile, site string) (*Client, error) {
	if err := profile.ValidateIdentity(); err != nil {
		return nil, err
	}
	if site == "" {
		site = profile.SupermarketSite
	}
	if site == "" {
		site = DefaultSite
	}
	base, err := url.Parse(strings.TrimRight(site, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("supermarket: invalid site URL %q", site)
	}
	key, err := cinc.LoadKeyFile(profile.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("supermarket: load key: %w", err)
	}
	return &Client{
		base:       base,
		userID:     profile.ClientName,
		key:        key,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		clock:      time.Now,
	}, nil
}

// Share packages and optionally uploads a cookbook to Supermarket.
func (c *Client) Share(ctx context.Context, opts ShareOptions) (ShareResult, error) {
	category := opts.Category
	if category == "" && !opts.DryRun {
		resolved, err := c.lookupCategory(ctx, opts.Cookbook)
		if err != nil {
			return ShareResult{}, err
		}
		category = resolved
	}
	result, archive, err := packageCookbook(opts, category, opts.DryRun)
	if err != nil {
		return ShareResult{}, err
	}
	if opts.DryRun {
		return result, nil
	}
	status, err := c.upload(ctx, opts.Cookbook, category, archive)
	if err != nil {
		return ShareResult{}, err
	}
	result.Uploaded = true
	result.Status = status
	return result, nil
}

// DryRun packages a cookbook for Supermarket without loading credentials or
// contacting the configured Supermarket.
func DryRun(opts ShareOptions) (ShareResult, error) {
	result, _, err := packageCookbook(opts, opts.Category, true)
	return result, err
}

func packageCookbook(opts ShareOptions, category string, includeFiles bool) (ShareResult, localcookbook.Archive, error) {
	dir, err := localcookbook.Locate(opts.Cookbook, opts.CookbookPath)
	if err != nil {
		return ShareResult{}, localcookbook.Archive{}, err
	}
	metadata, err := localcookbook.LoadMetadata(dir)
	if err != nil {
		return ShareResult{}, localcookbook.Archive{}, err
	}
	md := metadata.Metadata
	if md.Name != opts.Cookbook {
		return ShareResult{}, localcookbook.Archive{}, fmt.Errorf("metadata.json name %q does not match requested cookbook %q", md.Name, opts.Cookbook)
	}
	if category == "" {
		category = "Other"
	}

	var archive localcookbook.Archive
	if includeFiles {
		archive, err = localcookbook.BuildArchiveWithMetadata(dir, opts.Cookbook, metadata.JSON)
	} else {
		archive, err = localcookbook.BuildUploadArchiveWithMetadata(dir, opts.Cookbook, metadata.JSON)
	}
	if err != nil {
		return ShareResult{}, localcookbook.Archive{}, err
	}
	return ShareResult{
		Cookbook: opts.Cookbook, Category: category,
		Uploaded: false, Status: 0, Tarball: archive.Name,
		TarballSize: len(archive.Bytes), Files: archive.Files,
	}, archive, nil
}

func (c *Client) lookupCategory(ctx context.Context, cookbook string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/api/v1/cookbooks/"+cookbook), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("supermarket: lookup cookbook category: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", err
		}
		return "", responseError(http.MethodGet, req.URL.Path, resp.StatusCode, body)
	}
	var out struct {
		Category string `json:"category"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("supermarket: decode cookbook category: %w", err)
	}
	return out.Category, nil
}

func (c *Client) upload(ctx context.Context, cookbook, category string, archive localcookbook.Archive) (int, error) {
	var body bytes.Buffer
	body.Grow(len(archive.Bytes) + 1024)
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("tarball", archive.Name)
	if err != nil {
		return 0, err
	}
	if _, err := part.Write(archive.Bytes); err != nil {
		return 0, err
	}
	cookbookBody, err := json.Marshal(map[string]string{"category": category})
	if err != nil {
		return 0, err
	}
	if err := mw.WriteField("cookbook", string(cookbookBody)); err != nil {
		return 0, err
	}
	if err := mw.Close(); err != nil {
		return 0, err
	}

	bodyBytes := body.Bytes()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/v1/cookbooks"), bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", "cinc-cli")
	if err := signHeaders(req.Header, signRequest{
		Method: http.MethodPost, Path: "/api/v1/cookbooks", Body: archive.Bytes,
		UserID: c.userID, Timestamp: c.timestamp(),
	}, c.key); err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("supermarket: upload %s: %w", cookbook, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	return 0, responseError(http.MethodPost, req.URL.Path, resp.StatusCode, respBody)
}

func (c *Client) endpoint(p string) string {
	u := *c.base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(p, "/")
	return u.String()
}

func (c *Client) timestamp() string {
	return c.clock().UTC().Format("2006-01-02T15:04:05Z")
}

type signRequest struct {
	Method    string
	Path      string
	Body      []byte
	UserID    string
	Timestamp string
}

func signHeaders(h http.Header, r signRequest, key *rsa.PrivateKey) error {
	bodyHash := contentHash(r.Body)
	digest := sha1.Sum([]byte(canonicalRequest(r, bodyHash)))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, digest[:])
	if err != nil {
		return fmt.Errorf("supermarket: sign request: %w", err)
	}
	h.Set("X-Ops-Sign", "algorithm=sha1;version=1.0;")
	h.Set("X-Ops-Userid", r.UserID)
	h.Set("X-Ops-Timestamp", r.Timestamp)
	h.Set("X-Ops-Content-Hash", bodyHash)
	for i, chunk := range chunk60(base64.StdEncoding.EncodeToString(sig)) {
		h.Set("X-Ops-Authorization-"+strconv.Itoa(i+1), chunk)
	}
	return nil
}

func canonicalRequest(r signRequest, contentHash string) string {
	p := canonicalPath(r.Path)
	pathHash := contentHashString(p)
	var b strings.Builder
	b.Grow(len(r.Method) + len(pathHash) + len(contentHash) + len(r.Timestamp) + len(r.UserID) + 98)
	b.WriteString("Method:")
	b.WriteString(r.Method)
	b.WriteString("\nHashed Path:")
	b.WriteString(pathHash)
	b.WriteString("\nX-Ops-Content-Hash:")
	b.WriteString(contentHash)
	b.WriteString("\nX-Ops-Timestamp:")
	b.WriteString(r.Timestamp)
	b.WriteString("\nX-Ops-UserId:")
	b.WriteString(r.UserID)
	return b.String()
}

func contentHash(body []byte) string {
	sum := sha1.Sum(body)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func contentHashString(s string) string {
	sum := sha1.Sum([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func canonicalPath(p string) string {
	if p == "" {
		return "/"
	}
	p = path.Clean("/" + strings.TrimLeft(p, "/"))
	if p == "." {
		return "/"
	}
	return p
}

func chunk60(s string) []string {
	var out []string
	for len(s) > 60 {
		out = append(out, s[:60])
		s = s[60:]
	}
	if len(s) > 0 {
		out = append(out, s)
	}
	return out
}

func responseError(method, p string, status int, body []byte) error {
	var decoded struct {
		ErrorMessages []string `json:"error_messages"`
		ErrorCode     string   `json:"error_code"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && len(decoded.ErrorMessages) > 0 {
		if decoded.ErrorCode != "" {
			return fmt.Errorf("supermarket: %s %s: %d %s: %s", method, p, status, decoded.ErrorCode, strings.Join(decoded.ErrorMessages, "; "))
		}
		return fmt.Errorf("supermarket: %s %s: %d: %s", method, p, status, strings.Join(decoded.ErrorMessages, "; "))
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("supermarket: %s %s: %d: %s", method, p, status, msg)
}
