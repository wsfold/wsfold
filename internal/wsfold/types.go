package wsfold

import "fmt"

type TrustClass string

const (
	TrustClassTrusted  TrustClass = "trusted"
	TrustClassExternal TrustClass = "external"
)

type Repo struct {
	LocalName    string
	Name         string
	Slug         string
	CheckoutPath string
	OriginURL    string
	TrustClass   TrustClass
}

func (r Repo) DisplayRef() string {
	if r.Slug != "" {
		return r.Slug
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
