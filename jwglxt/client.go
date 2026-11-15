// Package jwglxt speaks the modern ZhengFang (正方) jwglxt protocol
// (JSON / UTF-8 / RSA login / jqGrid pagination). It is school-agnostic and
// accepts either direct RSA password login or externally injected session cookies.
package jwglxt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrLoginRejected means the server refused the credentials. Callers must not
// retry: ZhengFang locks an account after a few consecutive failures.
var ErrLoginRejected = errors.New("jwglxt: login rejected")

// ErrSessionExpired means a query was answered with the login page.
var ErrSessionExpired = errors.New("jwglxt: session expired")

// Cookie is a portable HTTP cookie record for external session injection.
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// PagedResponse preserves the raw ZhengFang jqGrid pagination envelope and
// unedited JSON rows.
type PagedResponse struct {
	CurrentPage int               `json:"currentPage"`
	TotalPage   int               `json:"totalPage"`
	TotalCount  int               `json:"totalCount"`
	TotalResult int               `json:"totalResult"`
	ShowCount   int               `json:"showCount"`
	Items       []json.RawMessage `json:"items"`
}

// Client holds an HTTP session against a ZhengFang jwglxt deployment.
type Client struct {
	base     *url.URL
	http     *http.Client
	username string
	password string
}

// New returns a client for base (e.g. "http://jwglxt.example.edu.cn").
// proxy may be empty, or an http:// or socks5:// URL.
func New(base, proxy, username, password string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("jwglxt: invalid base URL %q", base)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("jwglxt: invalid proxy %q", proxy)
		}
		transport.Proxy = http.ProxyURL(p)
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		base: u,
		http: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   60 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		username: username,
		password: password,
	}, nil
}

// InjectCookies installs externally acquired session cookies into the client's jar.
func (c *Client) InjectCookies(cookies []Cookie) {
	var list []*http.Cookie
	for _, ck := range cookies {
		path := ck.Path
		if path == "" {
			path = "/"
		}
		list = append(list, &http.Cookie{
			Name:  ck.Name,
			Value: ck.Value,
			Path:  path,
		})
	}
	c.http.Jar.SetCookies(c.base, list)
}

// InjectCookieHeader parses a raw "k1=v1; k2=v2" Cookie header string into the jar.
func (c *Client) InjectCookieHeader(header string) {
	var list []*http.Cookie
	for _, part := range strings.Split(header, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || k == "" {
			continue
		}
		list = append(list, &http.Cookie{Name: strings.TrimSpace(k), Value: strings.TrimSpace(v), Path: "/"})
	}
	if len(list) > 0 {
		c.http.Jar.SetCookies(c.base, list)
	}
}

var (
	csrfPattern = regexp.MustCompile(`name="csrftoken"\s+value="([^"]*)"|id="csrftoken"[^>]*value="([^"]*)"`)
	tipsPattern = regexp.MustCompile(`(?s)id="tips"[^>]*>(.*?)<`)
)

// Login performs exactly one RSA password login.
func (c *Client) Login(ctx context.Context) error {
	page, _, err := c.do(ctx, http.MethodGet, "/xtgl/login_slogin.html", nil, false)
	if err != nil {
		return err
	}
	csrf := ""
	if m := csrfPattern.FindStringSubmatch(page); m != nil {
		csrf = m[1] + m[2]
	}
	keyBody, _, err := c.do(ctx, http.MethodGet, "/xtgl/login_getPublicKey.html?time="+now(), nil, false)
	if err != nil {
		return err
	}
	var key struct{ Modulus, Exponent string }
	if err := json.Unmarshal([]byte(keyBody), &key); err != nil {
		return fmt.Errorf("jwglxt: public key: %w", err)
	}
	cipher, err := encrypt(key.Modulus, key.Exponent, c.password)
	if err != nil {
		return err
	}
	form := url.Values{"csrftoken": {csrf}, "language": {"zh_CN"}, "yhm": {c.username}, "mm": {cipher}}
	body, resp, err := c.do(ctx, http.MethodPost, "/xtgl/login_slogin.html?time="+now(), form, false)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 == 3 {
		return nil
	}
	if m := tipsPattern.FindStringSubmatch(body); m != nil {
		return fmt.Errorf("%w: %s", ErrLoginRejected, strings.TrimSpace(m[1]))
	}
	return fmt.Errorf("jwglxt: login returned HTTP %d without redirect", resp.StatusCode)
}

// PostRaw posts form to path and returns raw JSON without dropping any fields.
func (c *Client) PostRaw(ctx context.Context, path string, form url.Values) (json.RawMessage, error) {
	body, resp, err := c.do(ctx, http.MethodPost, path, form, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 == 3 || strings.Contains(body, "login_slogin") {
		return nil, ErrSessionExpired
	}
	trimmed := strings.TrimSpace(body)
	if !json.Valid([]byte(trimmed)) {
		return nil, fmt.Errorf("jwglxt: %s answered HTTP %d non-JSON (%d bytes)", path, resp.StatusCode, len(body))
	}
	return json.RawMessage(trimmed), nil
}

// PagedQuery fetches one page from a jqGrid endpoint, preserving raw item JSON.
func (c *Client) PagedQuery(ctx context.Context, path string, params url.Values, showCount, page int) (PagedResponse, error) {
	form := cloneValues(params)
	if showCount <= 0 {
		showCount = 100
	}
	if page <= 0 {
		page = 1
	}
	form.Set("_search", "false")
	form.Set("queryModel.showCount", strconv.Itoa(showCount))
	form.Set("queryModel.currentPage", strconv.Itoa(page))
	if !form.Has("queryModel.sortName") {
		form.Set("queryModel.sortName", "")
	}
	if !form.Has("queryModel.sortOrder") {
		form.Set("queryModel.sortOrder", "asc")
	}

	raw, err := c.PostRaw(ctx, path, form)
	if err != nil {
		return PagedResponse{}, err
	}
	var out PagedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return PagedResponse{}, fmt.Errorf("jwglxt: decode paged response from %s: %w", path, err)
	}
	return out, nil
}

// PagedQueryAll iterates across all pages of a jqGrid endpoint up to maxPages
// (0 means unlimited).
func (c *Client) PagedQueryAll(ctx context.Context, path string, params url.Values, showCount, maxPages int) ([]json.RawMessage, error) {
	if showCount <= 0 {
		showCount = 500
	}
	var all []json.RawMessage
	for page := 1; maxPages <= 0 || page <= maxPages; page++ {
		res, err := c.PagedQuery(ctx, path, params, showCount, page)
		if err != nil {
			return all, err
		}
		all = append(all, res.Items...)
		if res.TotalPage <= 0 || page >= res.TotalPage || len(res.Items) == 0 {
			break
		}
	}
	return all, nil
}

// PostJSON posts form to path and decodes a JSON answer into out.
func (c *Client) PostJSON(ctx context.Context, path string, form url.Values, out any) error {
	body, resp, err := c.do(ctx, http.MethodPost, path, form, true)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 == 3 || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		if resp.StatusCode/100 == 3 || strings.Contains(body, "login_slogin") {
			return ErrSessionExpired
		}
		return fmt.Errorf("jwglxt: %s answered %d %s", path, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return json.Unmarshal([]byte(body), out)
}

// GetPage fetches an HTML page of the authenticated session.
func (c *Client) GetPage(ctx context.Context, path string) (string, error) {
	body, resp, err := c.do(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 == 3 || strings.Contains(body, "login_getPublicKey") {
		return "", ErrSessionExpired
	}
	return body, nil
}

func (c *Client) do(ctx context.Context, method, path string, form url.Values, xhr bool) (string, *http.Response, error) {
	var reader io.Reader
	if form != nil {
		reader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+path, reader)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	if xhr {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	return string(data), resp, nil
}

func encrypt(modulus, exponent, password string) (string, error) {
	n, err := base64.StdEncoding.DecodeString(modulus)
	if err != nil {
		return "", fmt.Errorf("jwglxt: modulus: %w", err)
	}
	e, err := base64.StdEncoding.DecodeString(exponent)
	if err != nil {
		return "", fmt.Errorf("jwglxt: exponent: %w", err)
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	out, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(password))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for k, vs := range in {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func now() string { return strconv.FormatInt(time.Now().UnixMilli(), 10) }
