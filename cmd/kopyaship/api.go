package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/tomruk/kopyaship/utils"
)

type httpClient struct {
	*http.Client
	u          *url.URL
	socketAddr string

	basicAuth func() string
}

func newHTTPClient() (*httpClient, error) {
	listen := config.Daemon.API.Listen
	basicAuthConfig := config.Daemon.API.BasicAuth

	setBasicAuth := func(hc *httpClient) {
		if basicAuthConfig.Enabled {
			username := basicAuthConfig.Username
			password := basicAuthConfig.Password
			hc.basicAuth = func() string {
				auth := username + ":" + password
				return base64.StdEncoding.EncodeToString([]byte(auth))
			}

			redirectPolicyFunc := func(req *http.Request, via []*http.Request) error {
				req.Header.Set("Authorization", "Basic "+hc.basicAuth())
				return nil
			}
			hc.Client.CheckRedirect = redirectPolicyFunc
		}
	}

	// Unix socket is supported on Windows 10 Insider Build 17063 and later.
	// For older versions, fall back to HTTP.
	if listen == "ipc" && utils.RunningOnWindows {
		tempSocketFile := "C:\\kopyaship_tmp_" + utils.RandString(5) + ".socket"
		defer os.Remove(tempSocketFile)
		l, err := net.Listen("unix", tempSocketFile)
		if err != nil {
			opErr, ok := err.(*net.OpError)
			if ok {
				_, ok := opErr.Unwrap().(*os.SyscallError)
				if ok {
					listen = "http://" + utils.APIFallbackAddr
				}
			}
		}
		l.Close()
	}

	if listen == "ipc" {
		socketAddr := filepath.Join(cacheDir, "api.socket")
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, "unix", socketAddr)
				},
			},
		}
		hc := &httpClient{Client: client, socketAddr: socketAddr}
		setBasicAuth(hc)
		return hc, nil
	} else {
		u, err := url.Parse(listen)
		if err != nil {
			return nil, err
		}
		hc := &httpClient{Client: &http.Client{}, u: u}
		setBasicAuth(hc)
		return hc, nil
	}
}

func (hc *httpClient) String() string {
	if hc.u == nil {
		return "unix: " + hc.socketAddr
	} else {
		return hc.u.String()
	}
}

func (hc *httpClient) url(path string) string {
	if hc.u == nil {
		return "http://unix" + path
	} else {
		u := *hc.u
		u.Path = path
		return u.String()
	}
}

func (hc *httpClient) CloseIdleConnections() {
	hc.Client.CloseIdleConnections()
}

func (hc *httpClient) NewRequest(method, path string, body io.ReadCloser) (*http.Request, error) {
	req, err := http.NewRequest(method, hc.url(path), body)
	if err != nil {
		return nil, err
	}
	if hc.basicAuth != nil {
		req.Header.Set("Authorization", "Basic "+hc.basicAuth())
	}
	return req, nil
}

func (hc *httpClient) Do(req *http.Request) (*http.Response, error) {
	if hc.basicAuth != nil {
		req.Header.Set("Authorization", "Basic "+hc.basicAuth())
	}
	resp, err := hc.Client.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return resp, fmt.Errorf("non-200 status code received: %s", resp.Status)
	}
	return resp, nil
}

func (hc *httpClient) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", hc.url(path), nil)
	if err != nil {
		return nil, err
	}
	if hc.basicAuth != nil {
		req.Header.Set("Authorization", "Basic "+hc.basicAuth())
	}
	resp, err := hc.Client.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return resp, fmt.Errorf("non-200 status code received: %s", resp.Status)
	}
	return resp, nil
}

func (hc *httpClient) Head(path string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", hc.url(path), nil)
	if err != nil {
		return nil, err
	}
	if hc.basicAuth != nil {
		req.Header.Set("Authorization", "Basic "+hc.basicAuth())
	}
	resp, err := hc.Client.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return resp, fmt.Errorf("non-200 status code received: %s", resp.Status)
	}
	return resp, nil
}

func (hc *httpClient) Post(path string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", hc.url(path), body)
	if err != nil {
		return nil, err
	}
	if hc.basicAuth != nil {
		req.Header.Set("Authorization", "Basic "+hc.basicAuth())
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := hc.Client.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return resp, fmt.Errorf("non-200 status code received: %s", resp.Status)
	}
	return resp, nil
}

func (hc *httpClient) PostForm(path string, data url.Values) (*http.Response, error) {
	return hc.Post(path, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}
