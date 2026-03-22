package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
)

// GitHubClientID is the OAuth App client ID for the device flow.
// Register at https://github.com/settings/applications/new with "Device flow" enabled.
// The client_id is not secret for native/CLI apps.
const GitHubClientID = "" // TODO: Register OAuth App and set client_id

// GetGitHubToken attempts to obtain a GitHub token via multiple methods.
// Order: gh CLI -> OAuth Device Flow -> manual paste.
func GetGitHubToken() (token, source string, err error) {
	// Try gh CLI
	if t, err := ghCLIToken(); err == nil && t != "" {
		fmt.Println("  Found token from GitHub CLI (gh)")
		return t, "gh-cli", nil
	}

	// Try OAuth Device Flow
	if GitHubClientID != "" {
		fmt.Println("Starting GitHub authentication...")
		if t, err := githubDeviceFlow(); err == nil && t != "" {
			return t, "device-flow", nil
		} else if err != nil {
			fmt.Printf("  Device flow unavailable: %v\n\n", err)
		}
	}

	// Manual paste
	t, err := promptToken("GitHub")
	if err != nil {
		return "", "", err
	}
	return t, "manual", nil
}

// GetGitLabToken attempts to obtain a GitLab token.
// Order: glab CLI -> manual paste.
func GetGitLabToken() (token, source string, err error) {
	// Try glab CLI
	if t, err := glabCLIToken(); err == nil && t != "" {
		fmt.Println("  Found token from GitLab CLI (glab)")
		return t, "glab-cli", nil
	}

	// Manual paste
	t, err := promptToken("GitLab")
	if err != nil {
		return "", "", err
	}
	return t, "manual", nil
}

// OfferSaveToken asks the user if they want to persist the token.
func OfferSaveToken() bool {
	var save bool
	prompt := &survey.Confirm{
		Message: "Save token to ~/.userclone.yml for future use?",
		Default: true,
	}
	if err := survey.AskOne(prompt, &save); err != nil {
		return false
	}
	return save
}

func ghCLIToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token")
	}
	return token, nil
}

func glabCLIToken() (string, error) {
	out, err := exec.Command("glab", "auth", "status", "-t").CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Token:") {
			t := strings.TrimSpace(strings.TrimPrefix(line, "Token:"))
			if t != "" {
				return t, nil
			}
		}
	}
	return "", fmt.Errorf("could not parse glab token")
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error"`
	ErrorDesc       string `json:"error_description"`
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

func githubDeviceFlow() (string, error) {
	reqBody := url.Values{
		"client_id": {GitHubClientID},
		"scope":     {"repo read:org"},
	}

	req, err := http.NewRequest("POST", "https://github.com/login/device/code",
		strings.NewReader(reqBody.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var dc deviceCodeResponse
	if err := json.Unmarshal(body, &dc); err != nil {
		return "", fmt.Errorf("parsing device code response: %w", err)
	}
	if dc.Error != "" {
		return "", fmt.Errorf("%s: %s", dc.Error, dc.ErrorDesc)
	}
	if dc.DeviceCode == "" {
		return "", fmt.Errorf("no device code in response")
	}

	fmt.Printf("\n  Open this URL:  %s\n", dc.VerificationURI)
	fmt.Printf("  Enter code:     %s\n\n", dc.UserCode)
	fmt.Println("  Waiting for authorization...")

	interval := dc.Interval
	if interval < 5 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		tokenReqBody := url.Values{
			"client_id":   {GitHubClientID},
			"device_code": {dc.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		tokenReq, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token",
			strings.NewReader(tokenReqBody.Encode()))
		if err != nil {
			return "", err
		}
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tokenReq.Header.Set("Accept", "application/json")

		tokenResp, err := http.DefaultClient.Do(tokenReq)
		if err != nil {
			continue
		}

		tokenBody, err := io.ReadAll(tokenResp.Body)
		tokenResp.Body.Close()
		if err != nil {
			continue
		}

		var at accessTokenResponse
		if err := json.Unmarshal(tokenBody, &at); err != nil {
			continue
		}

		switch at.Error {
		case "":
			if at.AccessToken != "" {
				fmt.Println("  Authorization complete!")
				return at.AccessToken, nil
			}
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			return "", fmt.Errorf("authorization timed out — please try again")
		case "access_denied":
			return "", fmt.Errorf("authorization was denied")
		default:
			return "", fmt.Errorf("%s: %s", at.Error, at.ErrorDesc)
		}
	}

	return "", fmt.Errorf("authorization timed out")
}

func promptToken(platform string) (string, error) {
	if platform == "GitHub" {
		fmt.Println("\n  Create a token at: github.com/settings/tokens")
		fmt.Println("  Required scope:    repo")
	} else {
		fmt.Println("\n  Create a token in your GitLab settings > Access Tokens")
		fmt.Println("  Required scope:    read_api")
	}
	fmt.Println()

	var token string
	prompt := &survey.Password{
		Message: "Paste your token:",
	}
	if err := survey.AskOne(prompt, &token); err != nil {
		return "", err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}
	return token, nil
}
