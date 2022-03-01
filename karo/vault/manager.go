package vault

import (
	vaultAPI "github.com/hashicorp/vault/api"
)

type Manager struct {
	api       vaultAPI.Client
	config    vaultAPI.Config
	tlsConfig vaultAPI.TLSConfig
}

func (v *Manager) SetSecret(keyName string, key []byte) error {
	return nil
}
