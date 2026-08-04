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

// Command server runs the customer-portal backend-v2 HTTP server.
package main

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/registry"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/updates"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// entity-service, the updates service, and SCIM all authenticate as the
	// same OAuth2 client-credentials app; only each service's base URL and
	// scopes differ.
	oauth2ClientID := os.Getenv("OAUTH2_CLIENT_ID")
	oauth2ClientSecret := os.Getenv("OAUTH2_CLIENT_SECRET")
	oauth2TokenURL := os.Getenv("OAUTH2_TOKEN_URL")

	entityCfg := entity.Config{
		BaseURL:      mustEnv("ENTITY_SERVICE_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("ENTITY_SERVICE_SCOPES")),
	}
	entityClient := entity.NewClient(entityCfg)

	updatesCfg := updates.Config{
		BaseURL:      mustEnv("UPDATES_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("UPDATES_SCOPES")),
	}
	updatesClient := updates.NewClient(updatesCfg)

	scimCfg := scim.Config{
		BaseURL:      mustEnv("SCIM_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("SCIM_SCOPES")),
	}
	scimClient := scim.NewClient(scimCfg)

	// The AI chat agent is a separate Python service (not entity-service),
	// but authenticates as the same shared OAuth2 client-credentials app as
	// entity/updates/scim above (see the Ballerina backend's Config.toml,
	// where every module — including ai_chat_agent's WebSocket variant —
	// reuses the same clientId/tokenUrl); only its base URLs differ.
	aiChatAgentCfg := aichatagent.Config{
		BaseURL:      mustEnv("AI_CHAT_AGENT_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("AI_CHAT_AGENT_SCOPES")),
	}
	aiChatAgentClient := aichatagent.NewClient(aiChatAgentCfg)

	aiChatAgentWsCfg := aichatagent.WSConfig{
		BaseURL:      mustEnv("AI_CHAT_AGENT_WS_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("AI_CHAT_AGENT_WS_SCOPES")),
	}
	aiChatAgentWsClient := aichatagent.NewWSClient(aiChatAgentWsCfg)

	// The product-consumption service is a separate service (not
	// entity-service) that provisions deployment licenses; it also
	// authenticates as the same shared OAuth2 app (see the Ballerina
	// backend's Config.toml).
	productConsumptionCfg := productconsumption.Config{
		BaseURL:      mustEnv("PRODUCT_CONSUMPTION_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("PRODUCT_CONSUMPTION_SCOPES")),
	}
	productConsumptionClient := productconsumption.NewClient(productConsumptionCfg)

	// The registry service (robot-account/registry-token issuance) is a
	// separate microservice (not entity-service), authenticating as the same
	// shared OAuth2 app.
	registryCfg := registry.Config{
		BaseURL:      mustEnv("REGISTRY_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("REGISTRY_SCOPES")),
	}
	registryClient, err := registry.NewClient(registryCfg)
	if err != nil {
		slog.Error("failed to construct registry client", "err", err)
		os.Exit(1)
	}

	// The project-contact onboarding service (contact/membership management)
	// is a separate microservice (not entity-service, not SCIM),
	// authenticating as the same shared OAuth2 app.
	userManagementCfg := usermanagement.Config{
		BaseURL:      mustEnv("USER_MANAGEMENT_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("USER_MANAGEMENT_SCOPES")),
	}
	userManagementClient, err := usermanagement.NewClient(userManagementCfg)
	if err != nil {
		slog.Error("failed to construct user-management client", "err", err)
		os.Exit(1)
	}

	// adminRole is the role string (from entity.GetUserMeResponse.Roles) that
	// grants admin privileges for registry-token and contact management —
	// mirrors the Ballerina reference's configurable authorizedRoles.adminRole.
	adminRole := mustEnv("AUTH_ADMIN_ROLE")

	userHandler := handler.NewUserHandler(entityClient, scimClient)
	projectHandler := handler.NewProjectHandler(entityClient)
	projectStatsHandler := handler.NewProjectStatsHandler(entityClient)
	caseHandler := handler.NewCaseHandler(entityClient)
	deploymentHandler := handler.NewDeploymentHandler(entityClient)
	deployedProductHandler := handler.NewDeployedProductHandler(entityClient)
	attachmentHandler := handler.NewAttachmentHandler(entityClient)
	productHandler := handler.NewProductHandler(entityClient)
	changeRequestHandler := handler.NewChangeRequestHandler(entityClient)
	callRequestHandler := handler.NewCallRequestHandler(entityClient)
	accountHandler := handler.NewAccountHandler(entityClient)
	updatesHandler := handler.NewUpdatesHandler(updatesClient)
	commentHandler := handler.NewCommentHandler(entityClient)
	productVulnerabilityHandler := handler.NewProductVulnerabilityHandler(entityClient)
	catalogHandler := handler.NewCatalogHandler(entityClient)
	timeCardHandler := handler.NewTimeCardHandler(entityClient)
	aiChatHandler := handler.NewAIChatHandler(aiChatAgentClient, entityClient)
	webSocketHandler := handler.NewWebSocketHandler(aiChatAgentWsClient, entityClient, splitComma(os.Getenv("WS_ALLOWED_ORIGINS")))
	productConsumptionHandler := handler.NewProductConsumptionHandler(productConsumptionClient, entityClient)
	globalHandler := handler.NewGlobalHandler(entityClient)
	instanceHandler := handler.NewInstanceHandler(entityClient)
	registryHandler := handler.NewRegistryHandler(entityClient, registryClient, adminRole)
	contactHandler := handler.NewContactHandler(entityClient, userManagementClient)

	authCfg := middleware.Config{
		JWKSEndpoint:          mustEnv("AUTH_JWKS_ENDPOINT"),
		Issuer:                mustEnv("AUTH_ISSUER"),
		Audiences:             splitComma(mustEnv("AUTH_AUDIENCE")),
		ClockSkew:             5 * time.Second,
		TokenValidatorEnabled: os.Getenv("AUTH_TOKEN_VALIDATOR_ENABLED") != "false",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// GET/PATCH /users/me are only served by entity-service when it runs
	// with DATA_SOURCE=servicenow — see internal/entity/users.go.
	mux.HandleFunc("GET /users/me", userHandler.GetMe)
	mux.HandleFunc("PATCH /users/me", userHandler.PatchMe)

	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("GET /projects/{id}/filters", projectStatsHandler.GetProjectFilters)
	mux.HandleFunc("GET /projects/{id}/features", projectStatsHandler.GetProjectFeatures)
	mux.HandleFunc("GET /projects/{id}/stats", projectStatsHandler.GetProjectDashboardStats)
	mux.HandleFunc("GET /projects/{id}/stats/cases", projectStatsHandler.GetProjectCaseStats)
	mux.HandleFunc("GET /projects/{id}/stats/conversations", projectStatsHandler.GetProjectConversationStats)
	mux.HandleFunc("GET /projects/{id}/stats/support", projectStatsHandler.GetProjectSupportStats)
	mux.HandleFunc("GET /projects/{id}/stats/time-cards", projectStatsHandler.GetProjectTimeCardStats)
	mux.HandleFunc("GET /projects/{id}/stats/change-requests", projectStatsHandler.GetProjectChangeRequestStats)
	mux.HandleFunc("POST /projects/{id}/cases/time-cards/search", projectStatsHandler.SearchProjectCaseTimeCards)
	mux.HandleFunc("POST /projects/{id}/instances/search", instanceHandler.SearchProjectInstances)
	mux.HandleFunc("POST /projects/{id}/instances/metrics/search", instanceHandler.SearchProjectInstanceMetrics)
	mux.HandleFunc("POST /projects/{id}/instances/usages/search", instanceHandler.SearchProjectInstanceUsage)
	mux.HandleFunc("POST /projects/{id}/instances/stats/metrics/search", instanceHandler.SearchProjectInstanceMetricsStats)
	mux.HandleFunc("POST /projects/{id}/instances/stats/usages/search", instanceHandler.SearchProjectInstanceUsageStats)
	mux.HandleFunc("POST /projects/{id}/registry-tokens", registryHandler.CreateRegistryToken)
	mux.HandleFunc("POST /projects/{id}/registry-tokens/search", registryHandler.SearchRegistryTokens)
	mux.HandleFunc("GET /projects/{id}/integration-users", registryHandler.GetProjectIntegrationUsers)
	mux.HandleFunc("GET /projects/{id}/contacts", contactHandler.GetProjectContacts)
	mux.HandleFunc("POST /projects/{id}/contacts", contactHandler.CreateProjectContact)
	mux.HandleFunc("DELETE /projects/{id}/contacts/{email}", contactHandler.RemoveProjectContact)
	mux.HandleFunc("PATCH /projects/{id}/contacts/{email}", contactHandler.UpdateProjectContactRole)
	mux.HandleFunc("POST /projects/{id}/contacts/validate", contactHandler.ValidateProjectContact)
	mux.HandleFunc("DELETE /registry-tokens/{id}", registryHandler.DeleteRegistryToken)
	mux.HandleFunc("POST /registry-tokens/{id}/regenerate", registryHandler.RegenerateRegistryToken)

	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("GET /cases/{id}/feedback", caseHandler.GetCaseFeedback)
	mux.HandleFunc("POST /cases/{id}/feedback", caseHandler.SubmitCaseFeedback)
	mux.HandleFunc("PATCH /cases/{caseId}/attachments/{attachmentId}", caseHandler.PatchCaseAttachment)
	mux.HandleFunc("POST /cases/{caseId}/escalations", caseHandler.CreateCaseEscalation)
	mux.HandleFunc("POST /cases/{caseId}/escalations/search", caseHandler.SearchCaseEscalations)

	mux.HandleFunc("POST /deployments/search", deploymentHandler.SearchDeployments)
	// POST /deployments only succeeds against entity-service's ServiceNow
	// data source — see internal/entity/deployments.go.
	mux.HandleFunc("POST /deployments", deploymentHandler.CreateDeployment)
	mux.HandleFunc("PATCH /deployments/{id}", deploymentHandler.PatchDeployment)
	mux.HandleFunc("PATCH /deployments/{deploymentId}/attachments/{attachmentId}", deploymentHandler.PatchDeploymentAttachment)
	mux.HandleFunc("POST /deployments/{id}/instances/search", instanceHandler.SearchDeploymentInstances)
	mux.HandleFunc("POST /deployments/{id}/instances/metrics/search", instanceHandler.SearchDeploymentInstanceMetrics)
	mux.HandleFunc("POST /deployments/{id}/instances/usages/search", instanceHandler.SearchDeploymentInstanceUsage)
	mux.HandleFunc("POST /deployments/{id}/instances/stats/metrics/search", instanceHandler.SearchDeploymentInstanceMetricsStats)
	mux.HandleFunc("POST /deployments/{id}/instances/stats/usages/search", instanceHandler.SearchDeploymentInstanceUsageStats)

	mux.HandleFunc("POST /deployed-products/search", deployedProductHandler.SearchDeployedProducts)
	// POST/PATCH /deployed-products only succeed against entity-service's
	// ServiceNow data source — see internal/entity/deployed_products.go.
	mux.HandleFunc("POST /deployed-products", deployedProductHandler.CreateDeployedProduct)
	mux.HandleFunc("PATCH /deployed-products/{id}", deployedProductHandler.PatchDeployedProduct)
	mux.HandleFunc("POST /deployments/{deploymentId}/products/{productId}/metrics/search", deployedProductHandler.SearchDeployedProductMetrics)
	mux.HandleFunc("POST /deployments/{deploymentId}/products/{productId}/metrics/usage-counts/search", deployedProductHandler.SearchDeployedProductUsageCounts)
	mux.HandleFunc("POST /deployments/products/{id}/instances/search", instanceHandler.SearchDeployedProductInstances)
	mux.HandleFunc("POST /deployments/products/{id}/instances/metrics/search", instanceHandler.SearchDeployedProductInstanceMetrics)
	mux.HandleFunc("POST /deployments/products/{id}/instances/usages/search", instanceHandler.SearchDeployedProductInstanceUsage)
	mux.HandleFunc("POST /deployments/products/{id}/instances/stats/metrics/search", instanceHandler.SearchDeployedProductInstanceMetricsStats)
	mux.HandleFunc("POST /deployments/products/{id}/instances/stats/usages/search", instanceHandler.SearchDeployedProductInstanceUsageStats)

	mux.HandleFunc("POST /attachments", attachmentHandler.CreateAttachment)
	mux.HandleFunc("POST /attachments/search", attachmentHandler.SearchAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", attachmentHandler.GetAttachmentContent)
	mux.HandleFunc("GET /attachments/{id}", attachmentHandler.GetAttachment)
	mux.HandleFunc("DELETE /attachments/{id}", attachmentHandler.DeleteAttachment)

	mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	mux.HandleFunc("POST /products/{id}/versions/search", productHandler.SearchProductVersions)

	// entity-service only supports change requests and call requests on its
	// ServiceNow data source — see internal/entity/change_requests.go and
	// internal/entity/call_requests.go.
	mux.HandleFunc("POST /change-requests", changeRequestHandler.CreateChangeRequest)
	mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
	mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
	mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
	mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)

	mux.HandleFunc("POST /call-requests", callRequestHandler.CreateCallRequest)
	mux.HandleFunc("POST /call-requests/search", callRequestHandler.SearchCallRequests)
	mux.HandleFunc("PATCH /call-requests/{id}", callRequestHandler.PatchCallRequest)

	mux.HandleFunc("POST /accounts/search", accountHandler.SearchAccounts)
	mux.HandleFunc("GET /accounts/{id}", accountHandler.GetAccount)

	mux.HandleFunc("POST /comments", commentHandler.CreateComment)
	mux.HandleFunc("POST /comments/search", commentHandler.SearchComments)

	mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
	mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
	mux.HandleFunc("GET /products/vulnerabilities/meta", globalHandler.GetVulnerabilityMeta)

	mux.HandleFunc("GET /metadata", globalHandler.GetMetadata)
	mux.HandleFunc("POST /search", globalHandler.GlobalSearch)

	mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
	mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)

	// entity-service only supports time cards on its ServiceNow data source —
	// see internal/entity/time_cards.go.
	mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)

	// AI chat feature: case classification and KB recommendations call the
	// upstream AI chat agent directly; conversation search/messages/summary
	// mix the AI agent with entity-service's conversation/comment routes.
	// See handler.AIChatHandler and handler.WebSocketHandler's doc comments
	// for the comment createdBy override gap this works around.
	mux.HandleFunc("POST /cases/classify", aiChatHandler.ClassifyCase)
	mux.HandleFunc("POST /conversations/recommendations/search", aiChatHandler.SearchRecommendations)
	mux.HandleFunc("POST /projects/{id}/conversations/search", aiChatHandler.SearchConversations)
	mux.HandleFunc("POST /projects/{id}/conversations", aiChatHandler.CreateConversation)
	mux.HandleFunc("GET /conversations/{id}", aiChatHandler.GetConversation)
	mux.HandleFunc("PATCH /conversations/{id}", aiChatHandler.UpdateConversation)
	mux.HandleFunc("GET /conversations/{id}/messages", aiChatHandler.GetConversationMessages)
	mux.HandleFunc("POST /projects/{projectId}/conversations/{conversationId}/messages", aiChatHandler.SendConversationMessage)
	mux.HandleFunc("GET /projects/{id}/conversations/{conversationId}/summary", aiChatHandler.GetConversationSummary)
	mux.HandleFunc("GET /ws", webSocketHandler.HandleWebSocket)

	// The product-consumption service is a separate service (not
	// entity-service) — see internal/productconsumption's package doc comment.
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", productConsumptionHandler.GetDeploymentLicense)
	mux.HandleFunc("POST /deployment-usages", productConsumptionHandler.ImportDeploymentUsage)

	mux.HandleFunc("GET /updates/product-update-levels", updatesHandler.GetProductUpdateLevels)
	mux.HandleFunc("POST /updates/levels/search", updatesHandler.SearchUpdatesBetweenUpdateLevels)

	addr := ":" + mustPort("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("Customer Portal Backend (v2) started", "addr", addr)

	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Auth(authCfg)(
					middleware.Logger(mux),
				),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("Customer Portal Backend (v2) stopped")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// mustPort returns the value of the given environment variable (or def if
// unset) as a bare port number, e.g. "8080" — not an address like ":8080" or
// "localhost:8080". Exits the process if the value isn't a valid TCP port.
func mustPort(key, def string) string {
	v := envOrDefault(key, def)
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("environment variable must be a plain port number (e.g. \"8080\"), not an address", "key", key, "value", v)
		os.Exit(1)
	}
	return v
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Silently ignored if the file does not exist; logs a warning for any other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// Strip surrounding quotes from value.
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, exists := os.LookupEnv(k); !exists {
			if err := os.Setenv(k, v); err != nil {
				slog.Warn("loadDotEnv: failed to set environment variable", "key", k, "err", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
