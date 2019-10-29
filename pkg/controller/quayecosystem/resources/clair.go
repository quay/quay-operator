package resources

import (
	"net/url"
	"time"

	"github.com/redhat-cop/quay-operator/pkg/client"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
)

func GenerateDefaultClairConfigFile() client.ClairFile {

	verifierProxy, _ := url.Parse("http://localhost:6062")

	return client.ClairFile{
		Clair: &client.ClairConfig{
			Database: &client.ClairDatabase{
				Type: "pgsql",
				Options: map[string]interface{}{
					"cachesize": 16384,
				},
			},
			Notifier: &client.ClairNotifier{
				Attempts:         1,
				RenotifyInterval: time.Hour * 1,
				Params: map[string]interface{}{
					"http": &client.ClairHttpNotifier{
						Proxy: "http://localhost:6063",
					},
				},
			},
			Updater: &client.ClairUpdater{
				Interval: constants.ClairDefaultUpdateInterval,
			},
			API: &client.ClairAPI{
				Port:          6062,
				HealthPort:    6061,
				Timeout:       time.Second * 900,
				PaginationKey: constants.ClairDefaultPaginationKey,
			},
		},
		JwtProxy: &client.ClairJwtProxy{
			SignerProxy: client.SignerProxyConfig{
				Enabled:    true,
				ListenAddr: ":6063",
				CAKeyFile:  constants.ClairMITMPrivateKey,
				CACrtFile:  constants.ClairMITMCertificate,
				Signer: client.SignerConfig{
					SignerParams: client.SignerParams{
						Issuer:         constants.SecurityScannerService,
						ExpirationTime: time.Minute * 5,
						MaxSkew:        time.Minute * 1,
						NonceLength:    32,
					},
					PrivateKey: client.RegistrableComponentConfig{
						Type:    "preshared",
						Options: map[string]interface{}{},
					},
				},
			},
			VerifierProxies: []client.VerifierProxyConfig{
				client.VerifierProxyConfig{
					CrtFile:    constants.ClairSSLCertPath,
					KeyFile:    constants.ClairSSLKeyPath,
					Enabled:    true,
					ListenAddr: ":6060",
					Verifier: client.VerifierConfig{
						Upstream: client.URL{
							URL: verifierProxy,
						},
						KeyServer: client.RegistrableComponentConfig{
							Type:    "keyregistry",
							Options: map[string]interface{}{},
						},
					},
				},
			},
		},
	}

}
