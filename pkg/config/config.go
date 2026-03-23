package config

import (
	"context"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.linka.cloud/grpc-toolkit/logger"

	"go.linka.cloud/oidc-proxy/pkg/acl"
)

type Config interface {
	ACL() *acl.ACL
	Watch() <-chan *acl.ACL
}

type config struct {
	acl *acl.ACL
}

func Load(path string) (Config, error) {
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
	log := logger.C(context.Background()).WithField("component", "config")
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.WithField("path", e.Name).Info("reloading config")
		acl := &acl.ACL{}
		viper.SetConfigFile(e.Name)
		if err := viper.ReadInConfig(); err != nil {
			log.WithError(err).Error("reload config")
			return
		}
		if err := viper.Unmarshal(acl); err != nil {
			log.WithError(err).Error("parse config")
			return
		}
		confCh <- acl
	})
	viper.WatchConfig()
	return confCh
}
