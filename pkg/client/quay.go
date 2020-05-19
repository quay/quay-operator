package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
)

type QuayClient struct {
	BaseURL    *url.URL
	httpClient *http.Client
	Username   string
	Password   string
}

type QuayValidationType string

const (
	DatabaseValidation    QuayValidationType = "database"
	RedisValidation       QuayValidationType = "redis"
	RegistryValidation    QuayValidationType = "registry-storage"
	TimeMachineValidation QuayValidationType = "time-machine"
	AccessValidation      QuayValidationType = "access"
	SslValidation         QuayValidationType = "ssl"
)

func (c *QuayClient) InitializationConfiguration() (*http.Response, StringValue, error) {
	req, err := c.newRequest("POST", "/api/v1/configapp/initialization", StringValue{})
	if err != nil {
		return nil, StringValue{}, err
	}
	var initializationResponse StringValue
	resp, err := c.do(req, &initializationResponse)

	return resp, initializationResponse, err
}

func (c *QuayClient) GetQuayConfiguration() (*http.Response, QuayConfig, error) {
	req, err := c.newRequest("GET", "/api/v1/superuser/config", nil)
	if err != nil {
		return nil, QuayConfig{}, err
	}
	var quayConfig QuayConfig
	resp, err := c.do(req, &quayConfig)

	return resp, quayConfig, err
}

func (c *QuayClient) UpdateQuayConfiguration(config QuayConfig) (*http.Response, QuayConfig, error) {
	req, err := c.newRequest("PUT", "/api/v1/superuser/config", config)
	if err != nil {
		return nil, QuayConfig{}, err
	}
	var quayConfig QuayConfig
	resp, err := c.do(req, &quayConfig)

	return resp, quayConfig, err
}

func (c *QuayClient) GetRegistryStatus() (*http.Response, RegistryStatus, error) {
	req, err := c.newRequest("GET", "/api/v1/superuser/registrystatus", nil)
	if err != nil {
		return nil, RegistryStatus{}, err
	}
	var registryStatus RegistryStatus
	resp, err := c.do(req, &registryStatus)

	return resp, registryStatus, err
}

func (c *QuayClient) GetKeys() (*http.Response, KeysResponse, error) {
	req, err := c.newRequest("GET", "/api/v1/superuser/keys", nil)
	if err != nil {
		return nil, KeysResponse{}, err
	}
	var keysResponse KeysResponse
	resp, err := c.do(req, &keysResponse)

	return resp, keysResponse, err
}

func (c *QuayClient) GetKey(kid string) (*http.Response, Key, error) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/v1/superuser/keys/%s", kid), nil)
	if err != nil {
		return nil, Key{}, err
	}
	var key Key
	resp, err := c.do(req, &key)

	return resp, key, err
}

func (c *QuayClient) CreateKey(key KeyCreationRequest) (*http.Response, KeyCreationResponse, error) {
	req, err := c.newRequest("POST", "/api/v1/superuser/keys", key)
	if err != nil {
		return nil, KeyCreationResponse{}, err
	}
	var keyCreationResponse KeyCreationResponse
	resp, err := c.do(req, &keyCreationResponse)

	return resp, keyCreationResponse, err
}

func (c *QuayClient) ValidateDatabase(config QuayConfig) (*http.Response, QuayStatusResponse, error) {
	req, err := c.newRequest("POST", "/api/v1/superuser/config/validate/database", config)
	if err != nil {
		return nil, QuayStatusResponse{}, err
	}
	var quayStatusResponse QuayStatusResponse
	resp, err := c.do(req, &quayStatusResponse)

	return resp, quayStatusResponse, err
}

func (c *QuayClient) ValidateComponent(config QuayConfig, validationType QuayValidationType) (*http.Response, QuayStatusResponse, error) {
	req, err := c.newRequest("POST", fmt.Sprintf("/api/v1/superuser/config/validate/%s", validationType), config)
	if err != nil {
		return nil, QuayStatusResponse{}, err
	}
	var quayStatusResponse QuayStatusResponse
	resp, err := c.do(req, &quayStatusResponse)

	return resp, quayStatusResponse, err
}

func (c *QuayClient) ValidateRedis(config QuayConfig) (*http.Response, QuayStatusResponse, error) {
	req, err := c.newRequest("POST", "/api/v1/superuser/config/validate/redis", config)
	if err != nil {
		return nil, QuayStatusResponse{}, err
	}
	var quayStatusResponse QuayStatusResponse
	resp, err := c.do(req, &quayStatusResponse)

	return resp, quayStatusResponse, err
}

func (c *QuayClient) SetupDatabase() (*http.Response, SetupDatabaseResponse, error) {
	req, err := c.newRequest("GET", "/api/v1/superuser/setupdb", nil)
	if err != nil {
		return nil, SetupDatabaseResponse{}, err
	}
	var setupDatabaseResponse SetupDatabaseResponse
	resp, err := c.do(req, &setupDatabaseResponse)

	return resp, setupDatabaseResponse, err
}

func (c *QuayClient) CreateSuperuser(config QuayCreateSuperuserRequest) (*http.Response, QuayStatusResponse, error) {
	req, err := c.newRequest("POST", "/api/v1/superuser/config/createsuperuser", config)
	if err != nil {
		return nil, QuayStatusResponse{}, err
	}
	var quayStatusResponse QuayStatusResponse
	resp, err := c.do(req, &quayStatusResponse)

	return resp, quayStatusResponse, err
}

func (c *QuayClient) CompleteSetup() (*http.Response, StringValue, error) {
	req, err := c.newRequest("POST", "/api/v1/kubernetes/config", StringValue{})
	if err != nil {
		return nil, StringValue{}, err
	}
	var setupResponse StringValue
	resp, err := c.do(req, &setupResponse)

	return resp, setupResponse, err
}

func (c *QuayClient) GetConfigFileStatus(fileName string) (*http.Response, ConfigFileStatus, error) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/v1/superuser/config/file/%s", fileName), nil)
	if err != nil {
		return nil, ConfigFileStatus{}, err
	}
	var configFileStatus ConfigFileStatus
	resp, err := c.do(req, &configFileStatus)

	return resp, configFileStatus, err
}

func (c *QuayClient) UploadFileResource(fileName string, content []byte) (*http.Response, QuayStatusResponse, error) {

	req, err := c.newFileUploadRequest("POST", fmt.Sprintf("/api/v1/superuser/config/file/%s", fileName), fileName, content)
	if err != nil {
		return nil, QuayStatusResponse{}, err
	}
	var quayStatusResponse QuayStatusResponse
	resp, err := c.do(req, &quayStatusResponse)

	return resp, quayStatusResponse, err
}

func (c *QuayClient) UploadCustomCertificate(certName string, certFile []byte) (*http.Response, error) {
	req, err := c.newFileUploadRequest("POST", fmt.Sprintf("/api/v1/superuser/customcerts/%s", certName), certName, certFile)
	if err != nil {
		return nil, err
	}
	var result interface{}
	resp, err := c.do(req, &result)

	// Quay returns `HTTP 204` with an empty body on success
	if err == io.EOF && resp.StatusCode == 204 {
		return resp, nil
	}

	return resp, err
}

func (c *QuayClient) newFileUploadRequest(method, path string, fileName string, content []byte) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	part.Write(content)

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, u.String(), body)
	req.SetBasicAuth(c.Username, c.Password)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Accept", "application/json")
	return req, nil

}

func (c *QuayClient) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
	req.SetBasicAuth(c.Username, c.Password)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}
func (c *QuayClient) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if _, ok := v.(*StringValue); ok {
		responseData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		responseObject := v.(*StringValue)
		responseObject.Value = string(responseData)

	} else {
		err = json.NewDecoder(resp.Body).Decode(v)
	}
	return resp, err
}

func NewClient(httpClient *http.Client, baseUrl string, username string, password string) *QuayClient {
	quayClient := QuayClient{
		httpClient: httpClient,
		Username:   username,
		Password:   password,
	}

	quayClient.BaseURL, _ = url.Parse(baseUrl)
	return &quayClient
}
