package wsfold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	envTrustedDir        = "WSFOLD_TRUSTED_DIR"
	envExternalDir       = "WSFOLD_EXTERNAL_DIR"
	envTrustedGitHubOrgs = "WSFOLD_TRUSTED_GITHUB_ORGS"
)

type Config struct {
	TrustedDir        string
	ExternalDir       string
	TrustedGitHubOrgs []string
}

func LoadConfig() (Config, error) {
	return loadConfig(os.LookupEnv)
}

func loadConfig(lookupEnv func(string) (string, bool)) (Config, error) {
	trustedDir, ok := lookupEnv(envTrustedDir)
	if !ok || strings.TrimSpace(trustedDir) == "" {
		return Config{}, fmt.Errorf("%s must be set", envTrustedDir)
	}

	externalDir, ok := lookupEnv(envExternalDir)
	if !ok || strings.TrimSpace(externalDir) == "" {
		return Config{}, fmt.Errorf("%s must be set", envExternalDir)
	}

	orgsRaw, ok := lookupEnv(envTrustedGitHubOrgs)
	if !ok {
		return Config{}, fmt.Errorf("%s must be set", envTrustedGitHubOrgs)
	}

	orgs, err := parseTrustedGitHubOrgs(orgsRaw)
	if err != nil {
		return Config{}, err
	}

	trustedDir, err = filepath.Abs(trustedDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve %s: %w", envTrustedDir, err)
	}

	externalDir, err = filepath.Abs(externalDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve %s: %w", envExternalDir, err)
	}

	return Config{
		TrustedDir:        trustedDir,
		ExternalDir:       externalDir,
		TrustedGitHubOrgs: orgs,
	}, nil
}

func parseTrustedGitHubOrgs(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	orgs := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			return nil, fmt.Errorf("%s contains an empty org entry", envTrustedGitHubOrgs)
		}

		normalized := strings.ToLower(trimmed)
		if strings.Contains(normalized, "/") || strings.Contains(normalized, " ") {
			return nil, fmt.Errorf("%s contains an invalid org %q", envTrustedGitHubOrgs, trimmed)
		}

		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		orgs = append(orgs, normalized)
	}

	sort.Strings(orgs)
	return orgs, nil
}

func (c Config) IsTrustedGitHubOrg(org string) bool {
	org = strings.ToLower(strings.TrimSpace(org))
	for _, candidate := range c.TrustedGitHubOrgs {
		if candidate == org {
			return true
		}
	}
	return false
}
