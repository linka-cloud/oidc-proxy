package config

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.bertha.cloud/partitio/lab/oidc-proxy/pkg/acl"
)

type Config interface {
	ACL() *acl.ACL
	Watch() <-chan *acl.ACL
}

type config struct {
	acl *acl.ACL
}

func Load(path string) (*config, error) {
	viper.AddConfigPath(".")
	viper.AddConfigPath(filepath.Dir(path))
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	acl := &acl.ACL{}
	if err := viper.Unmarshal(acl); err != nil {
		return nil, err
	}
	return &config{acl: acl}, nil
}

func (c *config) ACL() *acl.ACL {
	return c.acl
}

func (c *config) Watch() <-chan *acl.ACL {
	confCh := make(chan *acl.ACL, 10)
	viper.OnConfigChange(func(_ fsnotify.Event) {
		logrus.Info("reloading config")
		acl := &acl.ACL{}
		if err := viper.Unmarshal(acl); err != nil {
			logrus.Errorf("reload config: %v", err)
			return
		}
		confCh <- acl
	})
	return confCh
}
