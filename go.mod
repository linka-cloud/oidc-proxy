module go.linka.cloud/oidc-proxy

go 1.16

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/fsnotify/fsnotify v1.7.0
	github.com/rs/cors v1.10.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/viper v1.17.0
	github.com/urfave/cli/v2 v2.25.7
	go.linka.cloud/oidc-handlers v0.0.8
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20231110203233-9a3e6036ecaa // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231120223509-83a465c0220f // indirect
)

replace github.com/coreos/go-oidc/v3 => go.linka.cloud/go-oidc/v3 v3.0.1-0.20231110111922-95602609e6b6
