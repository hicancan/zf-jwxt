// Package jwxt speaks the classic ASP.NET ZhengFang jwxt protocol
// (GB18030-encoded WebForms with ViewState and CheckCode captcha login, or
// externally injected session cookies).
package jwxt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Cookie is a portable HTTP cookie record for external session injection.
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// ErrLoginRejected means credentials were refused; callers must not retry.
var ErrLoginRejected = errors.New("jwxt: login rejected")

// ErrCaptchaRejected means the server rejected the captcha code.
var ErrCaptchaRejected = errors.New("jwxt: captcha rejected")

// ErrSessionExpired means the server redirected to the login page.
var ErrSessionExpired = errors.New("jwxt: session expired")

// CaptchaSolver reads a CheckCode.aspx GIF/PNG image and returns the text.
type CaptchaSolver func(ctx context.Context, img []byte) (string, error)

// Client holds an authenticated session against classic jwxt.
type Client struct {
	base     *url.URL
	http     *http.Client
	username string
	password string
}

// New returns a client for base (e.g. "http://jwxt.example.edu.cn").
func New(base, proxy, username, password string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("jwxt: invalid base URL %q", base)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("jwxt: invalid proxy %q", proxy)
		}
		transport.Proxy = http.ProxyURL(p)
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		base: u,
		http: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		username: username,
		password: password,
	}, nil
}

// Username returns the student/staff ID configured on the client.
func (c *Client) Username() string { return c.username }

var (
	viewStatePattern = regexp.MustCompile(`name="__VIEWSTATE"[^>]*value="([^"]*)"`)
	alertPattern     = regexp.MustCompile(`alert\(['"]([^'"]+)['"]\)`)
)

// Login performs direct password + CheckCode.aspx captcha login against /default2.aspx.
func (c *Client) Login(ctx context.Context, solve CaptchaSolver) error {
	if solve == nil {
		return errors.New("jwxt: captcha solver is required for direct login")
	}
	page, resp, err := c.doRaw(ctx, http.MethodGet, c.base.String()+"/default2.aspx", nil, "")
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwxt: GET /default2.aspx returned %d", resp.StatusCode)
	}
	vs := extractViewState(decodeGBK(page))
	if vs == "" {
		return errors.New("jwxt: __VIEWSTATE not found on /default2.aspx")
	}

	imgBytes, imgResp, err := c.doRaw(ctx, http.MethodGet, c.base.String()+"/CheckCode.aspx", nil, c.base.String()+"/default2.aspx")
	if err != nil {
		return err
	}
	if imgResp.StatusCode != http.StatusOK || len(imgBytes) == 0 {
		return fmt.Errorf("jwxt: GET /CheckCode.aspx returned %d", imgResp.StatusCode)
	}
	code, err := solve(ctx, imgBytes)
	if err != nil {
		return fmt.Errorf("jwxt: solve captcha: %w", err)
	}

	// RadioButtonList1 = "学生" in GBK (%D1%A7%C9%FA).
	form := url.Values{
		"__VIEWSTATE":      {vs},
		"txtUserName":      {c.username},
		"TextBox2":         {c.password},
		"txtSecretCode":    {strings.TrimSpace(code)},
		"RadioButtonList1": {"\xd1\xa7\xc9\xfa"},
		"Button1":          {""},
		"lbLanguage":       {""},
		"hidPdrs":          {""},
		"hidsc":            {""},
	}
	bodyBytes, postResp, err := c.doRaw(ctx, http.MethodPost, c.base.String()+"/default2.aspx", strings.NewReader(form.Encode()), c.base.String()+"/default2.aspx")
	if err != nil {
		return err
	}
	if postResp.StatusCode/100 == 3 {
		loc := postResp.Header.Get("Location")
		if strings.Contains(loc, "xs_main.aspx") || strings.Contains(loc, "js_main.aspx") {
			return nil
		}
	}
	body := decodeGBK(bodyBytes)
	if m := alertPattern.FindStringSubmatch(body); m != nil {
		msg := m[1]
		if strings.Contains(msg, "验证码") {
			return fmt.Errorf("%w: %s", ErrCaptchaRejected, msg)
		}
		return fmt.Errorf("%w: %s", ErrLoginRejected, msg)
	}
	return fmt.Errorf("%w: unexpected HTTP %d", ErrLoginRejected, postResp.StatusCode)
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

// GetPage fetches an authenticated GBK-encoded page and returns UTF-8 HTML.
func (c *Client) GetPage(ctx context.Context, path string) (string, error) {
	u := c.base.String() + path
	bodyBytes, resp, err := c.doRaw(ctx, http.MethodGet, u, nil, c.base.String()+"/xs_main.aspx?xh="+url.QueryEscape(c.username))
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 == 3 {
		return "", ErrSessionExpired
	}
	body := decodeGBK(bodyBytes)
	if strings.Contains(body, "default2.aspx") && strings.Contains(body, "txtSecretCode") {
		return "", ErrSessionExpired
	}
	return body, nil
}

// PostForm submits an application/x-www-form-urlencoded POST (raw url.Values)
// and decodes the GBK response to UTF-8.
func (c *Client) PostForm(ctx context.Context, path string, form url.Values, refererPath string) (string, error) {
	u := c.base.String() + path
	ref := c.base.String() + refererPath
	bodyBytes, resp, err := c.doRaw(ctx, http.MethodPost, u, strings.NewReader(form.Encode()), ref)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 == 3 {
		return "", ErrSessionExpired
	}
	return decodeGBK(bodyBytes), nil
}

func (c *Client) doRaw(ctx context.Context, method, fullURL string, body io.Reader, referer string) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp, err
}

func extractViewState(doc string) string {
	if m := viewStatePattern.FindStringSubmatch(doc); m != nil {
		return html.UnescapeString(m[1])
	}
	return ""
}

func decodeGBK(b []byte) string {
	r := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GB18030.NewDecoder())
	out, err := io.ReadAll(r)
	if err != nil {
		return string(b)
	}
	return string(out)
}

func encodeGBK(s string) string {
	r := transform.NewReader(strings.NewReader(s), simplifiedchinese.GB18030.NewEncoder())
	out, err := io.ReadAll(r)
	if err != nil {
		return s
	}
	return string(out)
}
