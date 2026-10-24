// Package zf speaks the ZhengFang (正方) jwglxt protocol: password login and
// form-posted JSON queries against an authenticated session.
package zf

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
var ErrLoginRejected = errors.New("zf: login rejected")

// ErrSessionExpired means a query was answered with the login page.
var ErrSessionExpired = errors.New("zf: session expired")

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
		return nil, fmt.Errorf("zf: invalid base URL %q", base)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("zf: invalid proxy %q", proxy)
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

var csrfPattern = regexp.MustCompile(`name="csrftoken"\s+value="([^"]*)"|id="csrftoken"[^>]*value="([^"]*)"`)
var tipsPattern = regexp.MustCompile(`(?s)id="tips"[^>]*>(.*?)<`)

// Login performs exactly one password login.
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
		return fmt.Errorf("zf: public key: %w", err)
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
	reason := "unknown response"
	if m := tipsPattern.FindStringSubmatch(body); m != nil {
		reason = strings.TrimSpace(m[1])
	}
	return fmt.Errorf("%w: %s", ErrLoginRejected, reason)
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
		return fmt.Errorf("zf: %s answered %d %s", path, resp.StatusCode, resp.Header.Get("Content-Type"))
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
		return "", fmt.Errorf("zf: modulus: %w", err)
	}
	e, err := base64.StdEncoding.DecodeString(exponent)
	if err != nil {
		return "", fmt.Errorf("zf: exponent: %w", err)
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	out, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(password))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func now() string { return strconv.FormatInt(time.Now().UnixMilli(), 10) }
