package vault

import (
	"encoding/hex"
	vaultAPI "github.com/hashicorp/vault/api"
	"io/ioutil"
	"net/http"
	"time"
)

type Manager struct {
	api       *vaultAPI.Client
	tlsConfig vaultAPI.TLSConfig
	paths     map[string]string
}

type SecretManager interface {
	SetSecret(keyName string, key []byte) error
	GetSecret(keyName string) ([]byte, error)
}

func (v *Manager) SetSecret(keyName string, key []byte) error {
	input := map[string]interface{}{
		"data": map[string]interface{}{
			keyName: hex.EncodeToString(key),
		},
	}
	_, err := v.api.Logical().Write("/secret/data/"+keyName, input)
	return err
}

func (v *Manager) GetSecret(keyName string) ([]byte, error) {
	data, err := v.api.Logical().Read("/secret/data/" + keyName)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(data.Data[keyName].(string))
}

func (v *Manager) GetRootToken() string {
	file, _ := ioutil.ReadFile(".secret")
	return string(file)
}

func (v *Manager) initAPI() {
	v.api.SetToken(v.GetRootToken())
}

func NewSecretManager() SecretManager {
	var httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	m := &Manager{}
	m.api, _ = vaultAPI.NewClient(
		&vaultAPI.Config{
			Address: "url", HttpClient: httpClient,
		})
	m.initAPI()
	return m
}
