package resources

import (
	"fmt"
	"net/url"
	"time"

	"github.com/redhat-cop/quay-operator/pkg/client"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
)

func GenerateDefaultClairConfigFile() client.ClairFile {

	verifierProxy, _ := url.Parse(fmt.Sprintf("http://localhost:%d", constants.ClairAPIPort))

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
						Proxy: fmt.Sprintf("http://localhost:%d", constants.ClairProxyPort),
					},
				},
			},
			Updater: &client.ClairUpdater{
				Interval: constants.ClairDefaultUpdateInterval,
			},
			API: &client.ClairAPI{
				Port:          constants.ClairAPIPort,
				HealthPort:    constants.ClairHealthPort,
				Timeout:       time.Second * 900,
				PaginationKey: constants.ClairDefaultPaginationKey,
			},
		},
		JwtProxy: &client.ClairJwtProxy{
			SignerProxy: client.SignerProxyConfig{
				Enabled:    true,
				ListenAddr: fmt.Sprintf(":%d", constants.ClairProxyPort),
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
					ListenAddr: fmt.Sprintf(":%d", constants.ClairPort),
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
