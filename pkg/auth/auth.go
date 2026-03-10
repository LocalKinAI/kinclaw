package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	oauthAuthURL  = "https://claude.ai/oauth/authorize"
	oauthTokenURL = "https://platform.claude.com/v1/oauth/token"
	oauthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
)

func randB64(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func Login() error {
	verifier := randB64(32)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	state := randB64(32)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	redirect := fmt.Sprintf("http://localhost:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	codeCh, errCh := make(chan string, 1), make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			fmt.Fprint(w, "<h2>Error: state mismatch.</h2>")
			errCh <- fmt.Errorf("state mismatch")
			return
		}
		if c := r.URL.Query().Get("code"); c != "" {
			fmt.Fprint(w, "<h2>Login successful! You can close this tab.</h2>")
			codeCh <- c
		} else {
			fmt.Fprint(w, "<h2>Error: no authorization code received.</h2>")
			errCh <- fmt.Errorf("no code in callback")
		}
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())
	authURL := oauthAuthURL + "?" + url.Values{
		"response_type": {"code"}, "client_id": {oauthClientID},
		"redirect_uri": {redirect}, "state": {state},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
		"scope": {"user:profile user:inference"},
	}.Encode()
	fmt.Fprintln(os.Stderr, "Opening browser for Claude login...")
	openBrowser(authURL)
	fmt.Fprintln(os.Stderr, "Waiting for authorization (2 min timeout)...")
	select {
	case code := <-codeCh:
		return exchangeAndSave(code, verifier, redirect, state)
	case err := <-errCh:
		return err
	case <-time.After(120 * time.Second):
		return fmt.Errorf("login timed out")
	}
}

func exchangeAndSave(code, verifier, redirect, state string) error {
	payload, _ := json.Marshal(map[string]string{
		"grant_type": "authorization_code", "client_id": oauthClientID,
		"code": code, "code_verifier": verifier, "redirect_uri": redirect, "state": state,
	})
	req, _ := http.NewRequest("POST", oauthTokenURL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://claude.ai/")
	req.Header.Set("Origin", "https://claude.ai")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return err
	}
	if tok.AccessToken == "" {
		return fmt.Errorf("no access token received (check your Claude account)")
	}
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".localkin", "auth.json")
	os.MkdirAll(filepath.Dir(path), 0700)
	data, _ := json.MarshalIndent(map[string]interface{}{
		"access_token": tok.AccessToken, "refresh_token": tok.RefreshToken,
		"expires_at": time.Now().Unix() + int64(tok.ExpiresIn),
	}, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Logged in! Token saved to %s\n", path)
	return nil
}

func openBrowser(u string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", u).Start()
	case "linux":
		exec.Command("xdg-open", u).Start()
	default:
		exec.Command("cmd", "/c", "start", u).Start()
	}
}
