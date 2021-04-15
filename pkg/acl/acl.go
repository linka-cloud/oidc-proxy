package acl

import (
	"strings"
	"sync"
)

type ACL struct {
	Endpoints     Endpoints `yaml:"endpoints" mapstructure:"endpoints" json:"endpoints"`
	DefaultPolicy Policy    `yaml:"defaultPolicy" mapstructure:"defaultPolicy" json:"defaultPolicy"`
}

func (a *ACL) Normalize() {
	if a.DefaultPolicy != AcceptPolicy && a.DefaultPolicy != RejectPolicy {
		a.DefaultPolicy = RejectPolicy
	}
	for _, v := range a.Endpoints {
		if v.DefaultPolicy != AcceptPolicy && v.DefaultPolicy != RejectPolicy {
			v.DefaultPolicy = a.DefaultPolicy
		}
		for _, vv := range v.Groups {
			if vv.Policy != AcceptPolicy && vv.Policy != RejectPolicy {
				switch v.DefaultPolicy {
				case AcceptPolicy:
					vv.Policy = RejectPolicy
				case RejectPolicy:
					vv.Policy = AcceptPolicy
				}
			}
		}
	}
}

type Endpoint struct {
	Host          string `yaml:"host" mapstructure:"host" json:"host"`
	DefaultPolicy Policy `yaml:"defaultPolicy" mapstructure:"defaultPolicy" json:"defaultPolicy"`
	Groups        Groups `yaml:"groups" mapstructure:"groups" json:"groups"`
}

type Endpoints []*Endpoint

func (e Endpoints) For(host string) (*Endpoint, bool) {
	for _, v := range e {
		if strings.EqualFold(v.Host, host) {
			return v, true
		}
	}
	return nil, false
}

type Group struct {
	Name   string `yaml:"name" mapstructure:"name" json:"name"`
	Policy Policy `yaml:"policy" mapstructure:"policy" json:"policy"`
}

type Groups []*Group

func (g Groups) For(group string) (*Group, bool) {
	for _, v := range g {
		if strings.EqualFold(v.Name, group) {
			return v, true
		}
	}
	return nil, false
}

const (
	AcceptPolicy Policy = "ACCEPT"
	RejectPolicy Policy = "REJECT"
)

type Policy string

func (p Policy) Equals(o Policy) bool {
	return strings.EqualFold(string(p), string(o))
}

type ACLMiddleware struct {
	acl ACL
	mu  sync.RWMutex
}
