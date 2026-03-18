package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://api.github.com"

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

type User struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type Repo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	CloneURL    string `json:"clone_url"`
	SSHURL      string `json:"ssh_url"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	Fork        bool   `json:"fork"`
	Archived    bool   `json:"archived"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type Org struct {
	Login       string `json:"login"`
	Description string `json:"description"`
}

func GetAuthenticatedUser(token string) (*User, error) {
	body, err := doRequest(token, "/user")
	if err != nil {
		return nil, fmt.Errorf("fetching user profile: %w", err)
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parsing user profile: %w", err)
	}

	return &user, nil
}

func GetUserRepos(token string, username string) ([]Repo, error) {
	var allRepos []Repo
	page := 1

	for {
		url := fmt.Sprintf("/user/repos?per_page=100&page=%d&affiliation=owner", page)
		body, err := doRequest(token, url)
		if err != nil {
			return nil, fmt.Errorf("fetching user repos (page %d): %w", page, err)
		}

		var repos []Repo
		if err := json.Unmarshal(body, &repos); err != nil {
			return nil, fmt.Errorf("parsing repos: %w", err)
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			if strings.EqualFold(r.Owner.Login, username) {
				allRepos = append(allRepos, r)
			}
		}

		if len(repos) < 100 {
			break
		}
		page++
	}

	return allRepos, nil
}

func GetUserOrgs(token string) ([]Org, error) {
	var allOrgs []Org
	page := 1

	for {
		url := fmt.Sprintf("/user/orgs?per_page=100&page=%d", page)
		body, err := doRequest(token, url)
		if err != nil {
			return nil, fmt.Errorf("fetching orgs (page %d): %w", page, err)
		}

		var orgs []Org
		if err := json.Unmarshal(body, &orgs); err != nil {
			return nil, fmt.Errorf("parsing orgs: %w", err)
		}

		if len(orgs) == 0 {
			break
		}

		allOrgs = append(allOrgs, orgs...)

		if len(orgs) < 100 {
			break
		}
		page++
	}

	return allOrgs, nil
}

func GetOrgRepos(token string, org string) ([]Repo, error) {
	var allRepos []Repo
	page := 1

	for {
		url := fmt.Sprintf("/orgs/%s/repos?per_page=100&page=%d", org, page)
		body, err := doRequest(token, url)
		if err != nil {
			return nil, fmt.Errorf("fetching org repos for %s (page %d): %w", org, page, err)
		}

		var repos []Repo
		if err := json.Unmarshal(body, &repos); err != nil {
			return nil, fmt.Errorf("parsing org repos: %w", err)
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)

		if len(repos) < 100 {
			break
		}
		page++
	}

	return allRepos, nil
}

func doRequest(token string, path string) ([]byte, error) {
	url := baseURL + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-Ratelimit-Remaining") == "0" {
		return nil, fmt.Errorf("GitHub API rate limit exceeded. Resets at: %s", resp.Header.Get("X-Ratelimit-Reset"))
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed — check your token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
