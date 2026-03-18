package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type OrgOverride struct {
	Exclude []string `yaml:"exclude,omitempty"`
}

type GitHubConfig struct {
	Token string `yaml:"token,omitempty"`
}

type GitLabConfig struct {
	Token string `yaml:"token,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

type Config struct {
	DefaultDest     string                 `yaml:"default_dest,omitempty"`
	DefaultPlatform string                 `yaml:"default_platform,omitempty"`
	WithOrgs        bool                   `yaml:"with_orgs"`
	GitHub          GitHubConfig           `yaml:"github,omitempty"`
	GitLab          GitLabConfig           `yaml:"gitlab,omitempty"`
	Exclude         []string               `yaml:"exclude,omitempty"`
	Orgs            map[string]OrgOverride `yaml:"orgs,omitempty"`
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".userclone.yml")
}

func Load() (*Config, error) {
	cfg := &Config{
		DefaultDest:     "~/Desktop",
		DefaultPlatform: "github",
	}

	path := ConfigPath()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.DefaultDest == "" {
		cfg.DefaultDest = "~/Desktop"
	}
	if cfg.DefaultPlatform == "" {
		cfg.DefaultPlatform = "github"
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	path := ConfigPath()
	if path == "" {
		return fmt.Errorf("could not determine home directory")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

func (c *Config) GetToken(isGitLab bool) string {
	if isGitLab {
		if c.GitLab.Token != "" {
			return c.GitLab.Token
		}
		if t := os.Getenv("GITLAB_TOKEN"); t != "" {
			return t
		}
		return ""
	}

	if c.GitHub.Token != "" {
		return c.GitHub.Token
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return ""
}

func (c *Config) IsExcluded(repoName string, orgName string) bool {
	for _, e := range c.Exclude {
		if strings.EqualFold(e, repoName) {
			return true
		}
	}

	if orgName != "" {
		if orgCfg, ok := c.Orgs[orgName]; ok {
			for _, e := range orgCfg.Exclude {
				if strings.EqualFold(e, repoName) {
					return true
				}
			}
		}
	}

	return false
}

func GenerateTemplate() string {
	return `# userclone configuration
# Docs: https://github.com/cj-ways/userclone

default_dest: ~/Desktop
default_platform: github    # github or gitlab
with_orgs: false             # clone org repos by default

github:
  token: ""                  # or set GITHUB_TOKEN env var

gitlab:
  token: ""                  # or set GITLAB_TOKEN env var
  url: https://gitlab.com    # self-hosted GitLab URL

exclude:                     # repos to always skip
  # - dotfiles
  # - old-project

orgs:                        # per-org overrides
  # my-org:
  #   exclude:
  #     - legacy-repo
`
}
