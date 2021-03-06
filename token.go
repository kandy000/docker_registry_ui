package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/docker/libtrust"
)

//Copy from github.com/docker/distribution/registry/auth/token/token.go#ResourceActions
const TokenSeparator = "."

//Refer github.com/docker/distribution/registry/auth/token/token.go#ResourceActions
type ResourceActions struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

//Refer github.com/docker/distribution/registry/auth/token/token.go#ClaimSet
type TokenClaimSet struct {
	// Public claims
	Issuer     string `json:"iss"`
	Subject    string `json:"sub"`
	Audience   string `json:"aud"`
	Expiration int64  `json:"exp"`
	NotBefore  int64  `json:"nbf"`
	IssuedAt   int64  `json:"iat"`
	JWTID      string `json:"jti"`

	// Private claims
	Access []*ResourceActions `json:"access"`
}

//Refer github.com/docker/distribution/registry/auth/token/token.go#Header
type TokenHeader struct {
	Type       string           `json:"typ"`
	SigningAlg string           `json:"alg"`
	KeyID      string           `json:"kid,omitempty"`
	X5c        []string         `json:"x5c,omitempty"`
	RawJWK     *json.RawMessage `json:"jwk,omitempty"`
}

func loadRegistryKeyPair() (libtrust.PublicKey, libtrust.PrivateKey, error) {
	//从数据库配置表中获取Registry认证密钥对
	keyBlock, err := GetStringConfig("registry_auth_token_key")
	if err != nil {
		return nil, nil, fmt.Errorf("获取Key失败: %s", err)
	}

	crtBlock, _ := GetStringConfig("registry_auth_token_cert")
	if err != nil {
		return nil, nil, fmt.Errorf("获取Cert失败: %s", err)
	}

	cert, err := tls.X509KeyPair([]byte(crtBlock), []byte(keyBlock))
	if err != nil {
		return nil, nil, err
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, nil, err
	}

	publicKey, err := libtrust.FromCryptoPublicKey(x509Cert.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	privateKey, err := libtrust.FromCryptoPrivateKey(cert.PrivateKey)
	if err != nil {
		return nil, nil, err
	}

	//刷新到文件系统，供Registry后端使用
	keyPath, err := GetStringConfig("registry_auth_token_key_path")
	ioutil.WriteFile(keyPath, []byte(crtBlock), 0755)

	crtPath, err := GetStringConfig("registry_auth_token_cert_path")
	ioutil.WriteFile(crtPath, []byte(keyBlock), 0755)

	return publicKey, privateKey, nil
}

func CreateToken(account, service string, authzResults []ResourceActions) (string, error) {
	publicKey, privateKey, err := loadRegistryKeyPair()
	if err != nil {
		return "", fmt.Errorf("Failed to load keypaire: %s", err)
	}

	now := time.Now().Unix()

	// Sign something dummy to find out which algorithm is used.
	_, sigAlg, err := privateKey.Sign(strings.NewReader("dummy"), 0)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %s", err)
	}

	//生成header
	header := TokenHeader{
		Type:       "JWT",
		SigningAlg: sigAlg,
		KeyID:      publicKey.KeyID(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %s", err)
	}
	hdrPayload := strings.TrimRight(base64.URLEncoding.EncodeToString(headerJSON), "=")

	tokenIssuer, _ := GetStringConfig("registry_token_issuer")
	tokenExpiration, _ := GetInt64Config("registry_token_expiration")

	//生成claims
	claims := TokenClaimSet{
		Issuer:     tokenIssuer,
		Subject:    account,
		Audience:   service,
		NotBefore:  now - tokenExpiration,
		IssuedAt:   now,
		Expiration: now + tokenExpiration,
		JWTID:      fmt.Sprintf("%d", rand.Int63()),
		Access:     []*ResourceActions{},
	}
	for _, result := range authzResults {
		/*ra := &token.ResourceActions{
			Name:    result.Name,
			Type:    result.Category,
			Actions: result.Actions,
		}
		*/
		if result.Actions == nil {
			result.Actions = []string{}
		}

		sort.Strings(result.Actions)
		claims.Access = append(claims.Access, &result)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %s", err)
	}

	claimsPayload := strings.TrimRight(base64.URLEncoding.EncodeToString(claimsJSON), "=")

	payload := hdrPayload + TokenSeparator + claimsPayload

	//签名
	sig, sigAlg2, err := privateKey.Sign(strings.NewReader(payload), 0)
	if err != nil || sigAlg2 != sigAlg {
		return "", fmt.Errorf("failed to sign token: %s", err)
	}

	sigPayload := strings.TrimRight(base64.URLEncoding.EncodeToString(sig), "=")

	return payload + TokenSeparator + sigPayload, nil
}
