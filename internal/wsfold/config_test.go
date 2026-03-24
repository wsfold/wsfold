package wsfold

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMissingEnv(t *testing.T) {
	t.Parallel()

	_, err := loadConfig(func(string) (string, bool) {
		return "", false
	})
	if err == nil || !strings.Contains(err.Error(), envTrustedDir) {
		t.Fatalf("expected missing trusted dir error, got %v", err)
	}
}

func TestLoadConfigAllowsMissingTrustedGitHubOrgs(t *testing.T) {
	t.Parallel()

	cfg, err := loadConfig(func(key string) (string, bool) {
		switch key {
		case envTrustedDir:
			return "/tmp/trusted", true
		case envExternalDir:
			return "/tmp/external", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if len(cfg.TrustedGitHubOrgs) != 0 {
		t.Fatalf("expected empty trusted orgs by default, got %#v", cfg.TrustedGitHubOrgs)
	}
}

func TestParseTrustedGitHubOrgsRejectsEmptyEntries(t *testing.T) {
	t.Parallel()

	_, err := parseTrustedGitHubOrgs("acme,,platform-team")
	if err == nil || !strings.Contains(err.Error(), "empty org entry") {
		t.Fatalf("expected malformed csv error, got %v", err)
	}
}

func TestParseTrustedGitHubOrgsNormalizes(t *testing.T) {
	t.Parallel()

	orgs, err := parseTrustedGitHubOrgs(" Platform-Team,acme,acme ")
	if err != nil {
		t.Fatalf("parseTrustedGitHubOrgs returned error: %v", err)
	}

	expected := []string{"acme", "platform-team"}
	if strings.Join(orgs, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected orgs: %#v", orgs)
	}
}

func TestClassifyCloneTarget(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TrustedDir:        filepath.Clean("/tmp/trusted"),
		ExternalDir:       filepath.Clean("/tmp/external"),
		TrustedGitHubOrgs: []string{"acme"},
	}

	trustClass, owner, repo, err := classifyCloneTarget(cfg, "acme/service")
	if err != nil {
		t.Fatalf("classifyCloneTarget returned error: %v", err)
	}
	if trustClass != TrustClassTrusted || owner != "acme" || repo != "service" {
		t.Fatalf("unexpected trusted classification: %v %s %s", trustClass, owner, repo)
	}

	trustClass, _, _, err = classifyCloneTarget(cfg, "git@gitlab.com:acme/service.git")
	if err != nil {
		t.Fatalf("classifyCloneTarget returned error: %v", err)
	}
	if trustClass != TrustClassExternal {
		t.Fatalf("expected non-github ref to default external, got %v", trustClass)
	}
}

func TestLoadProjectsDirNameDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := loadConfig(func(key string) (string, bool) {
		switch key {
		case envTrustedDir:
			return "/tmp/trusted", true
		case envExternalDir:
			return "/tmp/external", true
		case envTrustedGitHubOrgs:
			return "acme", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if cfg.ProjectsDirName != "." {
		t.Fatalf("expected default projects dir, got %q", cfg.ProjectsDirName)
	}

	cfg, err = loadConfig(func(key string) (string, bool) {
		switch key {
		case envTrustedDir:
			return "/tmp/trusted", true
		case envExternalDir:
			return "/tmp/external", true
		case envTrustedGitHubOrgs:
			return "acme", true
		case envProjectsDir:
			return "_ctx", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if cfg.ProjectsDirName != "_ctx" {
		t.Fatalf("expected overridden projects dir, got %q", cfg.ProjectsDirName)
	}

	cfg, err = loadConfig(func(key string) (string, bool) {
		switch key {
		case envTrustedDir:
			return "/tmp/trusted", true
		case envExternalDir:
			return "/tmp/external", true
		case envProjectsDir:
			return ".", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if cfg.ProjectsDirName != "." {
		t.Fatalf("expected explicit root-mount projects dir, got %q", cfg.ProjectsDirName)
	}
}
