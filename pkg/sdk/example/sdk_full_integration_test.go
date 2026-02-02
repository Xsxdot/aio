package example

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xsxdot/aio/pkg/sdk"
)

// TestSDK_FullIntegration is a comprehensive integration test that exercises all SDK functionality
// against a real registry/user/config/shorturl deployment.
//
// Required environment variables:
//   - SDK_INTEGRATION=1 (explicit opt-in)
//   - REGISTRY_ADDR (e.g., "localhost:50051" or "host1:50051,host2:50051")
//   - CLIENT_KEY
//   - CLIENT_SECRET
//   - SDK_TEST_SERVICE_ID (serviceId for RegisterSelf)
//   - SDK_TEST_SHORTURL_DOMAIN_ID (domainId for shorturl operations)
//
// Optional environment variables:
//   - SDK_TEST_PROJECT (default: "aio")
//   - SDK_TEST_ENV (default: "dev")
//   - SDK_TEST_SERVICE_NAME (if set, use this service for Discovery.Pick; otherwise use first available)
//   - SDK_TEST_SHORTURL_HOST (if set, use this host for Resolve; otherwise parse from short_url)
func TestSDK_FullIntegration(t *testing.T) {
	registryAddr := "127.0.0.1:50051"
	if registryAddr == "" {
		t.Skip("Skipping integration test: REGISTRY_ADDR not set")
	}

	clientKey := "P7ChCGWUIbXXzeezdH3r77ZTaptSDBQG"
	if clientKey == "" {
		t.Skip("Skipping integration test: CLIENT_KEY not set")
	}

	clientSecret := "71N4AsKOpUAzmdC0El7spfYRFK--Hkraom5MC24AlwI="
	if clientSecret == "" {
		t.Skip("Skipping integration test: CLIENT_SECRET not set")
	}

	shortURLDomainIDStr := "1"
	if shortURLDomainIDStr == "" {
		t.Skip("Skipping integration test: SDK_TEST_SHORTURL_DOMAIN_ID not set")
	}
	shortURLDomainID, err := strconv.ParseInt(shortURLDomainIDStr, 10, 64)
	if err != nil {
		t.Fatalf("Invalid SDK_TEST_SHORTURL_DOMAIN_ID: %v", err)
	}

	// Optional parameters
	project := "aio"
	if project == "" {
		project = "aio"
	}

	env := os.Getenv("SDK_TEST_ENV")
	if env == "" {
		env = "dev"
	}

	// Create SDK client
	t.Logf("Creating SDK client (registry: %s, project: %s, env: %s)", registryAddr, project, env)
	client, err := sdk.New(sdk.Config{
		RegistryAddr: registryAddr,
		ClientKey:    clientKey,
		ClientSecret: clientSecret,
	})
	if err != nil {
		t.Fatalf("Failed to create SDK client: %v", err)
	}
	defer client.Close()

	t.Log("SDK client created successfully")

	// Step 1: Authentication
	t.Run("Auth", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		token, err := client.Auth.Token(ctx)
		if err != nil {
			if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
				t.Fatalf("Authentication failed (environment/auth issue): %v", err)
			}
			t.Fatalf("Failed to get token: %v", err)
		}

		if token == "" {
			t.Fatal("Token is empty")
		}

		t.Logf("Token obtained: %s...", token[:min(20, len(token))])
	})

	// Step 2: Registry - List Services
	var services []sdk.ServiceDescriptor
	t.Run("Registry_ListServices", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		services, err = client.Registry.ListServices(ctx, project, env)
		if err != nil {
			if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
				t.Fatalf("ListServices failed (environment/auth issue): %v", err)
			}
			t.Fatalf("Failed to list services: %v", err)
		}

		t.Logf("Found %d services", len(services))
		for _, svc := range services {
			t.Logf("  - %s/%s: %d instances", svc.Project, svc.Name, len(svc.Instances))
		}
	})

	// Step 3: Discovery - Pick
	t.Run("Discovery_Pick", func(t *testing.T) {
		if len(services) == 0 {
			t.Skip("No services available for Pick test")
		}

		// Select target service
		var targetService *sdk.ServiceDescriptor
		testServiceName := os.Getenv("SDK_TEST_SERVICE_NAME")
		if testServiceName != "" {
			for i := range services {
				if services[i].Name == testServiceName {
					targetService = &services[i]
					break
				}
			}
			if targetService == nil {
				t.Fatalf("Service %s not found in service list", testServiceName)
			}
		} else {
			// Use first service with instances
			for i := range services {
				if len(services[i].Instances) > 0 {
					targetService = &services[i]
					break
				}
			}
			if targetService == nil {
				t.Skip("No service with instances available for Pick test")
			}
		}

		t.Logf("Using service: %s/%s", targetService.Project, targetService.Name)

		// Pick 3 times to verify round-robin
		for i := 0; i < 3; i++ {
			instance, reportErr, err := client.Discovery.Pick(
				targetService.Project,
				targetService.Name,
				env,
			)
			if err != nil {
				t.Fatalf("Pick round %d failed: %v", i+1, err)
			}

			if instance == nil {
				t.Fatalf("Pick round %d returned nil instance", i+1)
			}

			if instance.Endpoint == "" {
				t.Fatalf("Pick round %d returned empty endpoint", i+1)
			}

			t.Logf("  Round %d: picked %s", i+1, instance.Endpoint)

			// Report success (simulate successful call)
			reportErr(nil)
		}
	})

	// Step 4: Registry - RegisterSelfWithEnsureService (complete registration loop)
	t.Run("Registry_RegisterSelfWithEnsureService_Heartbeat_Stop", func(t *testing.T) {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "sdk-integration-test"
		}

		// Use timestamp to avoid conflicts if test runs multiple times
		serviceName := fmt.Sprintf("sdk-integration-test-%d", time.Now().Unix())
		instanceKey := fmt.Sprintf("sdk-test-%d", time.Now().Unix())

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Prepare service ensure request (no need for pre-existing serviceID)
		svcReq := &sdk.EnsureServiceRequest{
			Project:     project,
			Name:        serviceName,
			Owner:       "sdk-test",
			Description: "SDK integration test service",
			SpecJSON:    `{"type":"test"}`,
		}

		// Prepare instance registration request
		instReq := &sdk.RegisterInstanceRequest{
			// ServiceID will be filled by RegisterSelfWithEnsureService
			InstanceKey: instanceKey,
			Env:         env,
			Host:        hostname,
			Endpoint:    fmt.Sprintf("http://%s:8080", hostname),
			MetaJSON:    `{"sdk":"go","test":"integration"}`,
			TTLSeconds:  15, // Short TTL for faster test
		}

		// Call the new complete registration loop
		handle, err := client.Registry.RegisterSelfWithEnsureService(ctx, svcReq, instReq)
		if err != nil {
			if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
				t.Fatalf("RegisterSelfWithEnsureService failed (environment/auth issue): %v", err)
			}
			t.Fatalf("Failed to register self: %v", err)
		}

		t.Logf("Successfully registered as: %s", handle.InstanceKey)

		// Ensure cleanup
		defer func() {
			t.Log("Stopping registration and deregistering...")
			if err := handle.Stop(); err != nil {
				t.Errorf("Failed to stop registration: %v", err)
			} else {
				t.Log("Successfully deregistered")
			}
		}()

		// Wait for at least one heartbeat cycle
		// With TTL=15, heartbeat interval = 15/3 = 5s, but minimum is 10s
		// So we wait 11 seconds to ensure at least one heartbeat attempt
		t.Log("Waiting for heartbeat cycle (11 seconds)...")
		time.Sleep(11 * time.Second)
		t.Log("Heartbeat cycle completed")
	})

	// Step 5: Config - GetConfigsByPrefix (create multiple, get by prefix, delete)
	t.Run("Config_GetByPrefix", func(t *testing.T) {
		// Generate unique prefix
		prefix := fmt.Sprintf("sdk.prefix.test.%d", time.Now().Unix())
		t.Logf("Using config prefix: %s", prefix)

		// Keys to create
		configKeys := []string{
			prefix + ".db." + env,
			prefix + ".jwt." + env,
			prefix + ".redis." + env,
			prefix + ".app." + env,
		}

		// Ensure cleanup
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, key := range configKeys {
				_, err := client.ConfigClient.DeleteConfig(ctx, key)
				if err != nil {
					t.Logf("Cleanup: failed to delete config %s (may not exist): %v", key, err)
				} else {
					t.Logf("Cleanup: config %s deleted", key)
				}
			}
		}()

		// Create multiple configs with the same prefix
		t.Run("CreateMultiple", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create db config
			_, err := client.ConfigClient.CreateConfig(ctx, &sdk.CreateConfigRequest{
				Key: configKeys[0], // db
				Value: map[string]*sdk.ConfigValue{
					"host": {
						Value: "localhost",
						Type:  sdk.ValueTypeString,
					},
					"port": {
						Value: "3306",
						Type:  sdk.ValueTypeInt,
					},
					"user": {
						Value: "test_user",
						Type:  sdk.ValueTypeString,
					},
				},
				Metadata: map[string]string{
					"purpose": "sdk_prefix_test",
					"section": "db",
				},
				Description: "SDK prefix test - DB config",
				ChangeNote:  "Initial creation",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("CreateConfig (db) failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to create db config: %v", err)
			}
			t.Log("Created db config")

			// Create jwt config
			_, err = client.ConfigClient.CreateConfig(ctx, &sdk.CreateConfigRequest{
				Key: configKeys[1], // jwt
				Value: map[string]*sdk.ConfigValue{
					"secret": {
						Value: "test_jwt_secret",
						Type:  sdk.ValueTypeString,
					},
					"expire-time": {
						Value: "24",
						Type:  sdk.ValueTypeInt,
					},
				},
				Metadata: map[string]string{
					"purpose": "sdk_prefix_test",
					"section": "jwt",
				},
				Description: "SDK prefix test - JWT config",
				ChangeNote:  "Initial creation",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("CreateConfig (jwt) failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to create jwt config: %v", err)
			}
			t.Log("Created jwt config")

			// Create redis config
			_, err = client.ConfigClient.CreateConfig(ctx, &sdk.CreateConfigRequest{
				Key: configKeys[2], // redis
				Value: map[string]*sdk.ConfigValue{
					"host": {
						Value: "localhost:6379",
						Type:  sdk.ValueTypeString,
					},
					"db": {
						Value: "0",
						Type:  sdk.ValueTypeInt,
					},
				},
				Metadata: map[string]string{
					"purpose": "sdk_prefix_test",
					"section": "redis",
				},
				Description: "SDK prefix test - Redis config",
				ChangeNote:  "Initial creation",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("CreateConfig (redis) failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to create redis config: %v", err)
			}
			t.Log("Created redis config")

			// Create app config
			_, err = client.ConfigClient.CreateConfig(ctx, &sdk.CreateConfigRequest{
				Key: configKeys[3], // app
				Value: map[string]*sdk.ConfigValue{
					"app-name": {
						Value: "test-app",
						Type:  sdk.ValueTypeString,
					},
					"port": {
						Value: "9000",
						Type:  sdk.ValueTypeInt,
					},
					"domain": {
						Value: "test.example.com",
						Type:  sdk.ValueTypeString,
					},
				},
				Metadata: map[string]string{
					"purpose": "sdk_prefix_test",
					"section": "app",
				},
				Description: "SDK prefix test - App config",
				ChangeNote:  "Initial creation",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("CreateConfig (app) failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to create app config: %v", err)
			}
			t.Log("Created app config")

			t.Logf("Successfully created %d configs with prefix %s", len(configKeys), prefix)
		})

		// Get by prefix
		t.Run("GetByPrefix", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			configs, err := client.ConfigClient.GetConfigsByPrefix(ctx, prefix, env)
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("GetConfigsByPrefix failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to get configs by prefix: %v", err)
			}

			if len(configs) == 0 {
				t.Fatal("Got empty configs map")
			}

			expectedCount := len(configKeys)
			if len(configs) != expectedCount {
				t.Errorf("Expected %d configs, got %d", expectedCount, len(configs))
			}

			t.Logf("Retrieved %d configs by prefix %s:", len(configs), prefix)
			for key, jsonStr := range configs {
				t.Logf("  - %s: %s", key, jsonStr)
			}

			// Verify all expected keys are present
			for _, expectedKey := range configKeys {
				if _, ok := configs[expectedKey]; !ok {
					t.Errorf("Expected config key %s not found in results", expectedKey)
				}
			}

			// Verify JSON content for one config
			dbConfigJSON, ok := configs[configKeys[0]]
			if ok {
				if !strings.Contains(dbConfigJSON, "localhost") {
					t.Errorf("DB config JSON does not contain expected 'localhost': %s", dbConfigJSON)
				}
				if !strings.Contains(dbConfigJSON, "test_user") {
					t.Errorf("DB config JSON does not contain expected 'test_user': %s", dbConfigJSON)
				}
			}
		})

		// Delete all
		t.Run("DeleteAll", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			deletedCount := 0
			for _, key := range configKeys {
				resp, err := client.ConfigClient.DeleteConfig(ctx, key)
				if err != nil {
					if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
						t.Fatalf("DeleteConfig failed (environment/auth issue): %v", err)
					}
					t.Errorf("Failed to delete config %s: %v", key, err)
					continue
				}

				if !resp.Success {
					t.Errorf("Delete config %s reported failure: %s", key, resp.Message)
					continue
				}

				deletedCount++
				t.Logf("Deleted config: %s", key)
			}

			if deletedCount != len(configKeys) {
				t.Errorf("Expected to delete %d configs, actually deleted %d", len(configKeys), deletedCount)
			} else {
				t.Logf("Successfully deleted all %d configs", deletedCount)
			}
		})

		// Verify deletion by trying to get by prefix again
		t.Run("VerifyDeletion", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			configs, err := client.ConfigClient.GetConfigsByPrefix(ctx, prefix, env)
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("GetConfigsByPrefix (verify) failed (environment/auth issue): %v", err)
				}
				// It's OK if it fails, configs might be deleted
				t.Logf("GetConfigsByPrefix after deletion returned error (expected): %v", err)
				return
			}

			if len(configs) > 0 {
				t.Errorf("Expected 0 configs after deletion, got %d", len(configs))
				for key := range configs {
					t.Logf("  - Unexpected remaining config: %s", key)
				}
			} else {
				t.Log("Verified: all configs deleted successfully")
			}
		})
	})

	// Step 6: Config - Full CRUD cycle
	t.Run("Config_CRUD", func(t *testing.T) {
		// Generate unique key
		baseKey := fmt.Sprintf("sdk.integration.%d", time.Now().Unix())
		fullKey := baseKey + "." + env

		t.Logf("Using config key: %s", fullKey)

		// Ensure cleanup
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := client.ConfigClient.DeleteConfig(ctx, fullKey)
			if err != nil {
				t.Logf("Cleanup: failed to delete config (may not exist): %v", err)
			} else {
				t.Log("Cleanup: config deleted")
			}
		}()

		// Create
		t.Run("Create", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			configInfo, err := client.ConfigClient.CreateConfig(ctx, &sdk.CreateConfigRequest{
				Key: fullKey,
				Value: map[string]*sdk.ConfigValue{
					"test_field": {
						Value: "initial_value",
						Type:  sdk.ValueTypeString,
					},
					"version": {
						Value: "1",
						Type:  sdk.ValueTypeInt,
					},
				},
				Metadata: map[string]string{
					"purpose": "sdk_integration_test",
				},
				Description: "SDK integration test config",
				ChangeNote:  "Initial creation",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("CreateConfig failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to create config: %v", err)
			}

			if configInfo.Key != fullKey {
				t.Errorf("Expected key %s, got %s", fullKey, configInfo.Key)
			}

			t.Logf("Config created successfully (ID: %d, Version: %d)", configInfo.ID, configInfo.Version)
		})

		// Get (via query endpoint)
		t.Run("Get", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			jsonStr, err := client.ConfigClient.GetConfigJSON(ctx, baseKey, env)
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("GetConfig failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to get config: %v", err)
			}

			if jsonStr == "" {
				t.Fatal("Got empty config JSON")
			}

			t.Logf("Config retrieved: %s", jsonStr)
		})

		// Update
		t.Run("Update", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			configInfo, err := client.ConfigClient.UpdateConfig(ctx, &sdk.UpdateConfigRequest{
				Key: fullKey,
				Value: map[string]*sdk.ConfigValue{
					"test_field": {
						Value: "updated_value",
						Type:  sdk.ValueTypeString,
					},
					"version": {
						Value: "2",
						Type:  sdk.ValueTypeInt,
					},
				},
				Metadata: map[string]string{
					"purpose": "sdk_integration_test",
				},
				Description: "SDK integration test config (updated)",
				ChangeNote:  "Update test",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("UpdateConfig failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to update config: %v", err)
			}

			t.Logf("Config updated successfully (Version: %d)", configInfo.Version)
		})

		// Get again (verify update)
		t.Run("Get_After_Update", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			jsonStr, err := client.ConfigClient.GetConfigJSON(ctx, baseKey, env)
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("GetConfig failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to get config after update: %v", err)
			}

			if jsonStr == "" {
				t.Fatal("Got empty config JSON after update")
			}

			t.Logf("Config after update: %s", jsonStr)
		})

		// Delete
		t.Run("Delete", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := client.ConfigClient.DeleteConfig(ctx, fullKey)
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("DeleteConfig failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to delete config: %v", err)
			}

			if !resp.Success {
				t.Errorf("Delete reported failure: %s", resp.Message)
			}

			t.Log("Config deleted successfully")
		})
	})

	// Step 7: ShortURL - Full flow
	t.Run("ShortURL_CreateResolveReport", func(t *testing.T) {
		var shortCode string
		var shortURL string

		// Create
		t.Run("CreateShortLink", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := client.ShortURL.CreateShortLink(ctx, &sdk.CreateShortLinkRequest{
				DomainID:   shortURLDomainID,
				TargetType: "URL", // Fixed: use uppercase "URL" to match valid TargetType constants
				TargetConfig: map[string]interface{}{
					"url": "https://example.com/test",
				},
				Comment: "SDK integration test",
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("CreateShortLink failed (environment/auth issue): %v", err)
				}
				t.Fatalf("Failed to create short link: %v", err)
			}

			if resp.Code == "" {
				t.Fatal("Got empty short code")
			}

			shortCode = resp.Code
			shortURL = resp.ShortURL

			t.Logf("Short link created: %s (code: %s)", shortURL, shortCode)
		})

		// Resolve
		t.Run("Resolve", func(t *testing.T) {
			if shortCode == "" {
				t.Skip("No short code available from create step")
			}

			// Determine host for resolve
			host := os.Getenv("SDK_TEST_SHORTURL_HOST")
			if host == "" && shortURL != "" {
				// Parse from short_url
				u, err := url.Parse(shortURL)
				if err == nil && u.Host != "" {
					host = u.Host
				}
			}
			if host == "" {
				t.Skip("Cannot determine host for Resolve (set SDK_TEST_SHORTURL_HOST or ensure CreateShortLink returns valid short_url)")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resolveResp, err := client.ShortURL.Resolve(ctx, host, shortCode, "")
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("Resolve failed (environment/auth issue): %v", err)
				}
				// NotFound is acceptable if the service has async processing
				if !sdk.IsNotFound(err) {
					t.Fatalf("Failed to resolve short link: %v", err)
				}
				t.Logf("Resolve returned NotFound (may be async, continuing)")
				return
			}

			if resolveResp.TargetType == "" {
				t.Error("Got empty target type from Resolve")
			}

			t.Logf("Resolved: type=%s, action=%s", resolveResp.TargetType, resolveResp.Action)
		})

		// ReportSuccess
		t.Run("ReportSuccess", func(t *testing.T) {
			if shortCode == "" {
				t.Skip("No short code available from create step")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			eventID := fmt.Sprintf("test-event-%d", time.Now().Unix())

			err := client.ShortURL.ReportSuccess(ctx, shortCode, eventID, map[string]interface{}{
				"test":      true,
				"timestamp": time.Now().Unix(),
			})
			if err != nil {
				if sdk.IsUnauthenticated(err) || sdk.IsUnavailable(err) {
					t.Fatalf("ReportSuccess failed (environment/auth issue): %v", err)
				}
				// NotFound is acceptable
				if !sdk.IsNotFound(err) {
					t.Fatalf("Failed to report success: %v", err)
				}
				t.Logf("ReportSuccess returned NotFound (continuing)")
				return
			}

			t.Logf("Success reported (eventID: %s)", eventID)
		})
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
