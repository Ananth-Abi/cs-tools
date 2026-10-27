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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"reflect"
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		"ENTITY_SERVICE_BASE_URL": "http://entity.internal:8081/",
		"OAUTH2_CLIENT_ID":        "id",
		"OAUTH2_CLIENT_SECRET":    "secret",
		"OAUTH2_TOKEN_URL":        "https://idp.example.com/oauth2/token",
	}
}

func TestLoadConfigValid(t *testing.T) {
	m := validEnv()
	m["ENTITY_SERVICE_SCOPES"] = "read, kb"
	m["PORT"] = "9000"
	cfg, err := loadConfig(env(m))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EntityBaseURL != "http://entity.internal:8081" {
		t.Errorf("trailing slash not trimmed: %q", cfg.EntityBaseURL)
	}
	if cfg.Port != "9000" {
		t.Errorf("port = %q", cfg.Port)
	}
	if want := []string{"read", "kb"}; !reflect.DeepEqual(cfg.EntityScopes, want) {
		t.Errorf("scopes = %v", cfg.EntityScopes)
	}
	if cfg.PineconeHost != "" {
		t.Errorf("pinecone should be disabled: %q", cfg.PineconeHost)
	}
}

func TestLoadConfigDefaultPort(t *testing.T) {
	cfg, err := loadConfig(env(validEnv()))
	if err != nil || cfg.Port != "8080" {
		t.Fatalf("port = %q err = %v", cfg.Port, err)
	}
}

func TestLoadConfigFailsFast(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]string)
		wantErr string
	}{
		{"missing entity url", func(m map[string]string) { delete(m, "ENTITY_SERVICE_BASE_URL") }, "ENTITY_SERVICE_BASE_URL is required"},
		{"blank entity url", func(m map[string]string) { m["ENTITY_SERVICE_BASE_URL"] = "   " }, "ENTITY_SERVICE_BASE_URL is required"},
		{"entity url not a url", func(m map[string]string) { m["ENTITY_SERVICE_BASE_URL"] = "entity-service:8081" }, "absolute http(s) URL"},
		{"entity url bad scheme", func(m map[string]string) { m["ENTITY_SERVICE_BASE_URL"] = "ftp://x" }, "absolute http(s) URL"},
		{"missing client id", func(m map[string]string) { delete(m, "OAUTH2_CLIENT_ID") }, "OAUTH2_CLIENT_ID is required"},
		{"missing client secret", func(m map[string]string) { delete(m, "OAUTH2_CLIENT_SECRET") }, "OAUTH2_CLIENT_SECRET is required"},
		{"missing token url", func(m map[string]string) { delete(m, "OAUTH2_TOKEN_URL") }, "OAUTH2_TOKEN_URL is required"},
		{"port not numeric", func(m map[string]string) { m["PORT"] = ":8080" }, "PORT"},
		{"port out of range", func(m map[string]string) { m["PORT"] = "70000" }, "PORT"},
		{"bad timeout", func(m map[string]string) { m["REST_READ_TIMEOUT"] = "soon" }, "REST_READ_TIMEOUT"},
		{"pinecone host without key", func(m map[string]string) { m["PINECONE_HOST"] = "https://p.example.com" }, "set together"},
		{"pinecone key without host", func(m map[string]string) { m["PINECONE_KEY"] = "k" }, "set together"},
		{"pinecone host not a url", func(m map[string]string) { m["PINECONE_HOST"] = "nope"; m["PINECONE_KEY"] = "k" }, "PINECONE_HOST must be"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validEnv()
			tt.mutate(m)
			_, err := loadConfig(env(m))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
