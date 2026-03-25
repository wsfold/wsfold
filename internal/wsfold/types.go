package wsfold

import (
	"fmt"
	"strings"
)

type TrustClass string
type CompletionSource string

const (
	TrustClassTrusted      TrustClass       = "trusted"
	TrustClassExternal     TrustClass       = "external"
	CompletionSourceLocal  CompletionSource = "local"
	CompletionSourceRemote CompletionSource = "remote"
)

type Repo struct {
	LocalName    string
	Name         string
	Slug         string
	Branch       string
	IsWorktree   bool
	CheckoutPath string
	OriginURL    string
	TrustClass   TrustClass
}

func (r Repo) DisplayRef() string {
	if r.Slug != "" && !r.IsWorktree {
		return r.Slug
	}
	if r.Slug != "" && r.IsWorktree && strings.TrimSpace(r.Branch) != "" {
		return r.Slug + "/" + strings.TrimSpace(r.Branch)
	}
	if r.LocalName != "" {
		return r.LocalName
	}
	if r.Name != "" {
		return r.Name
	}
	return r.CheckoutPath
}

type Entry struct {
	RepoRef      string     `yaml:"repo_ref" json:"repo_ref"`
	CheckoutPath string     `yaml:"checkout_path" json:"checkout_path"`
	TrustClass   TrustClass `yaml:"trust_class" json:"trust_class"`
	MountPath    string     `yaml:"mount_path,omitempty" json:"mount_path,omitempty"`
}

func (e Entry) Key() string {
	return fmt.Sprintf("%s|%s|%s", e.TrustClass, e.RepoRef, e.CheckoutPath)
}
