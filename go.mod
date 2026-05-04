module github.com/wesdod/mira-vpn/mira-vpn-backend

go 1.25.0

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/golang-migrate/migrate/v4 v4.17.1
	github.com/lib/pq v1.10.9
	github.com/wesdod/mira-vpn/mira-vpn-wgmgr v0.0.0
	golang.org/x/crypto v0.50.0
)

replace github.com/wesdod/mira-vpn/mira-vpn-wgmgr => ../mira-vpn-wgmgr

require (
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	go.uber.org/atomic v1.7.0 // indirect
)
