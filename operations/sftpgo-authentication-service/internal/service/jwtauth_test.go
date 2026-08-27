// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
)

const (
	jwtTestIssuer   = "https://idp.example.com/oauth2/token"
	jwtTestAudience = "test-audience"
	jwtTestEmail    = "jane.doe@example.com"
	jwtTestUserID   = "00000000-0000-0000-0000-000000000000"
)

// newJWKSTestServer starts an httptest server serving key's public key as a
// JWKS document (kty=RSA), the same shape the real IdP publishes.
func newJWKSTestServer(t *testing.T, key *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()

	n := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
	eBytes := big.NewInt(int64(key.PublicKey.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func newJWTAuthTestService(t *testing.T, jwksURL string) *JWTAuthService {
	t.Helper()

	cfg := &config.Config{
		AuthJWKSEndpoint:          jwksURL,
		AuthIssuer:                jwtTestIssuer,
		AuthAudiences:             []string{jwtTestAudience},
		AuthTokenValidatorEnabled: true,
	}
	svc, err := NewJWTAuthService(cfg, log.NewAppLogger("ERROR"))
	if err != nil {
		t.Fatalf("failed to construct JWTAuthService: %v", err)
	}
	return svc
}

func baseTestClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"email":  jwtTestEmail,
		"userid": jwtTestUserID,
		"groups": []string{"cs-engineers"},
		"iss":    jwtTestIssuer,
		"aud":    []string{jwtTestAudience},
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	}
}

// TestValidateAndExtract_RS256_Accepted proves a correctly-signed RS256 token
// is still accepted after pinning WithValidMethods.
func TestValidateAndExtract_RS256_Accepted(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	srv := newJWKSTestServer(t, key, "key-1")
	defer srv.Close()

	svc := newJWTAuthTestService(t, srv.URL)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseTestClaims())
	token.Header["kid"] = "key-1"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	info, err := svc.ValidateAndExtract(signed)
	if err != nil {
		t.Fatalf("expected RS256 token to be accepted, got error: %v", err)
	}
	if info.Email != jwtTestEmail {
		t.Errorf("expected email %q, got %q", jwtTestEmail, info.Email)
	}
}

// TestValidateAndExtract_HS256_Rejected proves fix #1: an HS256-signed token
// is rejected even if an attacker used the RSA public key's modulus bytes as
// the HMAC secret (the algorithm-confusion attack this pin closes). The
// rejection must happen because HS256 is not an allowed method, not merely
// because the guessed secret happens to be wrong.
func TestValidateAndExtract_HS256_Rejected(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	srv := newJWKSTestServer(t, key, "key-1")
	defer srv.Close()

	svc := newJWTAuthTestService(t, srv.URL)

	// Simulate an algorithm-confusion attempt: sign with HS256 using the RSA
	// public key's modulus bytes as the HMAC secret (the "known" material an
	// attacker could derive from the published JWKS).
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, baseTestClaims())
	token.Header["kid"] = "key-1"
	signed, err := token.SignedString(key.PublicKey.N.Bytes())
	if err != nil {
		t.Fatalf("failed to sign HS256 token: %v", err)
	}

	if _, err := svc.ValidateAndExtract(signed); err == nil {
		t.Fatal("expected HS256-signed token to be rejected, got no error")
	}
}

// TestValidateAndExtract_NoneAlgorithm_Rejected proves an unsigned ("alg":
// "none") token is rejected.
func TestValidateAndExtract_NoneAlgorithm_Rejected(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	srv := newJWKSTestServer(t, key, "key-1")
	defer srv.Close()

	svc := newJWTAuthTestService(t, srv.URL)

	token := jwt.NewWithClaims(jwt.SigningMethodNone, baseTestClaims())
	token.Header["kid"] = "key-1"
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to build none-alg token: %v", err)
	}

	if _, err := svc.ValidateAndExtract(signed); err == nil {
		t.Fatal("expected none-algorithm token to be rejected, got no error")
	}
}
