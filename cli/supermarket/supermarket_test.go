package supermarket

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tas50/cinc-cli/cli/config"
)

func TestShareDryRunPackagesCookbookWithoutNetwork(t *testing.T) {
	cookbookRoot := writeSupermarketCookbook(t, "nginx")
	client := supermarketTestClient(t, "https://supermarket.example.test")

	result, err := client.Share(context.Background(), ShareOptions{
		Cookbook: "nginx", Category: "Other", CookbookPath: cookbookRoot, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Share: %v", err)
	}
	if result.Uploaded {
		t.Fatal("dry-run reported upload")
	}
	if result.Tarball != "nginx.tgz" {
		t.Fatalf("tarball = %q, want nginx.tgz", result.Tarball)
	}
	if len(result.Files) != 2 || result.Files[0] != "nginx/metadata.json" {
		t.Fatalf("files = %v, want metadata and recipe", result.Files)
	}
}

func TestShareDryRunGeneratesMetadataJSONFromMetadataRB(t *testing.T) {
	cookbookRoot := writeSupermarketCookbookFromMetadataRB(t, "nginx")
	client := supermarketTestClient(t, "https://supermarket.example.test")

	result, err := client.Share(context.Background(), ShareOptions{
		Cookbook: "nginx", Category: "Other", CookbookPath: cookbookRoot, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Share: %v", err)
	}
	if len(result.Files) != 3 || result.Files[0] != "nginx/metadata.json" || result.Files[1] != "nginx/metadata.rb" {
		t.Fatalf("files = %v, want generated metadata.json before metadata.rb", result.Files)
	}
}

func TestShareInfersCategoryAndPostsMultipartUpload(t *testing.T) {
	cookbookRoot := writeSupermarketCookbook(t, "nginx")
	var sawUpload bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cookbooks/nginx":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"category":"Web Servers"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cookbooks":
			sawUpload = true
			if r.Header.Get("X-Ops-Authorization-1") == "" {
				t.Error("upload request missing X-Ops-Authorization-1")
			}
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader: %v", err)
			}
			fields := map[string]string{}
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				data, _ := io.ReadAll(part)
				if part.FormName() == "tarball" {
					if part.FileName() != "nginx.tgz" {
						t.Errorf("tarball filename = %q, want nginx.tgz", part.FileName())
					}
					if len(data) == 0 {
						t.Error("tarball field is empty")
					}
					continue
				}
				fields[part.FormName()] = string(data)
			}
			if fields["cookbook"] != `{"category":"Web Servers"}` {
				t.Errorf("cookbook field = %q", fields["cookbook"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := supermarketTestClient(t, srv.URL)
	result, err := client.Share(context.Background(), ShareOptions{
		Cookbook: "nginx", CookbookPath: cookbookRoot,
	})
	if err != nil {
		t.Fatalf("Share: %v", err)
	}
	if !sawUpload {
		t.Fatal("expected upload POST")
	}
	if !result.Uploaded || result.Status != http.StatusCreated || result.Category != "Web Servers" {
		t.Fatalf("result = %+v", result)
	}
}

func TestShareSurfacesSupermarketValidationError(t *testing.T) {
	cookbookRoot := writeSupermarketCookbook(t, "nginx")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error_code":"VALIDATION_ERROR","error_messages":["Version already exists"]}`)
	}))
	t.Cleanup(srv.Close)

	client := supermarketTestClient(t, srv.URL)
	_, err := client.Share(context.Background(), ShareOptions{
		Cookbook: "nginx", Category: "Other", CookbookPath: cookbookRoot,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "Version already exists") {
		t.Fatalf("error = %q, want validation message", err)
	}
}

func TestNewUsesSupermarketSiteFromProfileWithoutServerURL(t *testing.T) {
	keyPath := writeSupermarketTestKey(t)
	client, err := New(config.Profile{
		SupermarketSite: "https://supermarket.example.test",
		ClientName:      "tim",
		KeyPath:         keyPath,
	}, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := client.base.String(); got != "https://supermarket.example.test" {
		t.Fatalf("base = %q, want profile SupermarketSite", got)
	}
}

func TestSignHeadersUsesChefAuthPrivateEncryptCanonicalRequest(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	request := signRequest{
		Method:    http.MethodPost,
		Path:      "/api/v1/cookbooks",
		Body:      []byte("tarball bytes"),
		UserID:    "damacus",
		Timestamp: "2026-05-20T10:11:12Z",
	}
	var h http.Header = map[string][]string{}
	if err := signHeaders(h, request, key); err != nil {
		t.Fatalf("signHeaders: %v", err)
	}

	signature, err := base64.StdEncoding.DecodeString(strings.Join(authorizationChunks(h), ""))
	if err != nil {
		t.Fatalf("decode authorization headers: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, 0, []byte(canonicalRequest(request, contentHash(request.Body))), signature); err != nil {
		t.Fatalf("signature did not verify against raw canonical request: %v", err)
	}
}

func writeSupermarketCookbook(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"`+name+`","version":"1.2.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes", "default.rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeSupermarketCookbookFromMetadataRB(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := "name '" + name + "'\nversion '1.2.0'\ndescription 'Generated metadata test'\n"
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes", "default.rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func supermarketTestClient(t *testing.T, site string) *Client {
	t.Helper()
	keyPath := writeSupermarketTestKey(t)
	client, err := New(config.Profile{
		SupermarketSite: site,
		ClientName:      "tim",
		KeyPath:         keyPath,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func writeSupermarketTestKey(t *testing.T) string {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "key.pem")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return keyPath
}

func authorizationChunks(h http.Header) []string {
	var chunks []string
	for i := 1; ; i++ {
		chunk := h.Get("X-Ops-Authorization-" + strconv.Itoa(i))
		if chunk == "" {
			return chunks
		}
		chunks = append(chunks, chunk)
	}
}
