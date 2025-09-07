package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pusher/internal/apps"
	"pusher/internal/constants"
)

// Helper function to create a test app
func createTestAppForArrayManager(id, key, secret string) apps.App {
	app := apps.App{
		ID:      constants.AppID(id),
		Key:     key,
		Secret:  secret,
		Enabled: true,
	}
	app.SetMissingDefaults()
	return app
}

// Helper function to create multiple test apps
func createTestAppsForArrayManager() []apps.App {
	return []apps.App{
		createTestAppForArrayManager("app1", "key1", "secret1"),
		createTestAppForArrayManager("app2", "key2", "secret2"),
		createTestAppForArrayManager("app3", "key3", "secret3"),
	}
}

func TestArrayAppManager_Init(t *testing.T) {
	t.Run("SuccessfulInit", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()

		err := manager.Init(testApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 3)
		assert.Equal(t, testApps[0], manager.Apps["app1"])
		assert.Equal(t, testApps[1], manager.Apps["app2"])
		assert.Equal(t, testApps[2], manager.Apps["app3"])
	})

	t.Run("EmptyAppsList", func(t *testing.T) {
		manager := &ArrayAppManager{}
		emptyApps := []apps.App{}

		err := manager.Init(emptyApps)

		assert.Error(t, err)
		assert.Equal(t, "no apps found", err.Error())
		assert.Len(t, manager.Apps, 0)
	})

	t.Run("NilAppsList", func(t *testing.T) {
		manager := &ArrayAppManager{}

		err := manager.Init(nil)

		assert.Error(t, err)
		assert.Equal(t, "no apps found", err.Error())
		assert.Len(t, manager.Apps, 0)
	})

	t.Run("SingleApp", func(t *testing.T) {
		manager := &ArrayAppManager{}
		singleApp := []apps.App{createTestAppForArrayManager("single", "single-key", "single-secret")}

		err := manager.Init(singleApp)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1)
		assert.Equal(t, singleApp[0], manager.Apps["single"])
	})

	t.Run("DuplicateAppIDs", func(t *testing.T) {
		manager := &ArrayAppManager{}
		duplicateApps := []apps.App{
			createTestAppForArrayManager("duplicate", "key1", "secret1"),
			createTestAppForArrayManager("duplicate", "key2", "secret2"), // Same ID, different key/secret
		}

		err := manager.Init(duplicateApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1) // Only one app should be stored (last one wins)
		assert.Equal(t, duplicateApps[1], manager.Apps["duplicate"])
	})

	t.Run("AppsWithMissingDefaults", func(t *testing.T) {
		manager := &ArrayAppManager{}
		appWithoutDefaults := apps.App{
			ID:     "test-app",
			Key:    "test-key",
			Secret: "test-secret",
			// Missing other fields that should be set by SetMissingDefaults
		}
		appWithoutDefaults.SetMissingDefaults()
		testApps := []apps.App{appWithoutDefaults}

		err := manager.Init(testApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1)
		assert.Equal(t, appWithoutDefaults, manager.Apps["test-app"])
	})
}

func TestArrayAppManager_GetAllApps(t *testing.T) {
	t.Run("EmptyManager", func(t *testing.T) {
		manager := &ArrayAppManager{}

		allApps := manager.GetAllApps()

		assert.Empty(t, allApps)
		assert.Len(t, allApps, 0)
	})

	t.Run("SingleApp", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := []apps.App{createTestAppForArrayManager("single", "single-key", "single-secret")}
		manager.Init(testApps)

		allApps := manager.GetAllApps()

		assert.Len(t, allApps, 1)
		assert.Equal(t, testApps[0], allApps[0])
	})

	t.Run("MultipleApps", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		allApps := manager.GetAllApps()

		assert.Len(t, allApps, 3)
		// Check that all apps are present (order may vary)
		appMap := make(map[constants.AppID]apps.App)
		for _, app := range allApps {
			appMap[app.ID] = app
		}
		assert.Equal(t, testApps[0], appMap["app1"])
		assert.Equal(t, testApps[1], appMap["app2"])
		assert.Equal(t, testApps[2], appMap["app3"])
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		// Test concurrent access to GetAllApps
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				allApps := manager.GetAllApps()
				assert.Len(t, allApps, 3)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestArrayAppManager_GetAppSecret(t *testing.T) {
	t.Run("ValidAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		secret, err := manager.GetAppSecret("app1")

		assert.NoError(t, err)
		assert.Equal(t, "secret1", secret)
	})

	t.Run("InvalidAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		secret, err := manager.GetAppSecret("nonexistent")

		assert.Error(t, err)
		assert.Equal(t, "app not found", err.Error())
		assert.Empty(t, secret)
	})

	t.Run("EmptyStringAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		secret, err := manager.GetAppSecret("")

		assert.Error(t, err)
		assert.Equal(t, "app not found", err.Error())
		assert.Empty(t, secret)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		// Test concurrent access to GetAppSecret
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				secret, err := manager.GetAppSecret("app1")
				assert.NoError(t, err)
				assert.Equal(t, "secret1", secret)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestArrayAppManager_FindByID(t *testing.T) {
	t.Run("ValidAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByID("app1")

		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, "app1", string(app.ID))
		assert.Equal(t, "key1", app.Key)
		assert.Equal(t, "secret1", app.Secret)
	})

	t.Run("InvalidAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByID("nonexistent")

		assert.Error(t, err)
		assert.Equal(t, "app not found", err.Error())
		assert.Nil(t, app)
	})

	t.Run("EmptyStringAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByID("")

		assert.Error(t, err)
		assert.Equal(t, "app not found", err.Error())
		assert.Nil(t, app)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		// Test concurrent access to FindByID
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				app, err := manager.FindByID("app1")
				assert.NoError(t, err)
				assert.NotNil(t, app)
				assert.Equal(t, "app1", string(app.ID))
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("ReturnedAppIsCopy", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByID("app1")
		require.NoError(t, err)
		require.NotNil(t, app)

		// Modify the returned app
		app.Key = "modified-key"

		// Get the app again to verify it wasn't modified in the manager
		app2, err := manager.FindByID("app1")
		require.NoError(t, err)
		require.NotNil(t, app2)

		assert.Equal(t, "key1", app2.Key)        // Original key should be unchanged
		assert.Equal(t, "modified-key", app.Key) // Our copy should be modified
	})
}

func TestArrayAppManager_FindByKey(t *testing.T) {
	t.Run("ValidKey", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByKey("key1")

		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, "app1", string(app.ID))
		assert.Equal(t, "key1", app.Key)
		assert.Equal(t, "secret1", app.Secret)
	})

	t.Run("InvalidKey", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByKey("nonexistent-key")

		assert.Error(t, err)
		assert.Equal(t, "app not found", err.Error())
		assert.Nil(t, app)
	})

	t.Run("EmptyStringKey", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByKey("")

		assert.Error(t, err)
		assert.Equal(t, "app not found", err.Error())
		assert.Nil(t, app)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		// Test concurrent access to FindByKey
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				app, err := manager.FindByKey("key1")
				assert.NoError(t, err)
				assert.NotNil(t, app)
				assert.Equal(t, "key1", app.Key)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("ReturnedAppIsCopy", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		app, err := manager.FindByKey("key1")
		require.NoError(t, err)
		require.NotNil(t, app)

		// Modify the returned app
		app.Secret = "modified-secret"

		// Get the app again to verify it wasn't modified in the manager
		app2, err := manager.FindByKey("key1")
		require.NoError(t, err)
		require.NotNil(t, app2)

		assert.Equal(t, "secret1", app2.Secret)        // Original secret should be unchanged
		assert.Equal(t, "modified-secret", app.Secret) // Our copy should be modified
	})
}

func TestArrayAppManager_EdgeCases(t *testing.T) {
	t.Run("AppsWithEmptyValues", func(t *testing.T) {
		manager := &ArrayAppManager{}
		emptyApp := apps.App{
			ID:     "",
			Key:    "",
			Secret: "",
		}
		emptyApp.SetMissingDefaults()
		testApps := []apps.App{emptyApp}

		err := manager.Init(testApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1)
		assert.Contains(t, manager.Apps, constants.AppID(""))
	})

	t.Run("AppsWithSpecialCharacters", func(t *testing.T) {
		manager := &ArrayAppManager{}
		specialApp := createTestAppForArrayManager("app-with-special-chars", "key@#$%", "secret!@#$%")
		testApps := []apps.App{specialApp}

		err := manager.Init(testApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1)
		assert.Equal(t, specialApp, manager.Apps["app-with-special-chars"])
	})

	t.Run("AppsWithUnicodeCharacters", func(t *testing.T) {
		manager := &ArrayAppManager{}
		unicodeApp := createTestAppForArrayManager("app-测试", "key测试", "secret测试")
		testApps := []apps.App{unicodeApp}

		err := manager.Init(testApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1)
		assert.Equal(t, unicodeApp, manager.Apps["app-测试"])
	})

	t.Run("VeryLongAppID", func(t *testing.T) {
		manager := &ArrayAppManager{}
		longID := "app-" + string(make([]byte, 1000)) // Very long ID
		longApp := createTestAppForArrayManager(longID, "key", "secret")
		testApps := []apps.App{longApp}

		err := manager.Init(testApps)

		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1)
		assert.Equal(t, longApp, manager.Apps[constants.AppID(longID)])
	})
}

// func TestArrayAppManager_InterfaceCompliance(t *testing.T) {
// 	t.Run("ImplementsAppManagerInterface", func(t *testing.T) {
// 		// This test ensures that ArrayAppManager implements the apps.AppManagerInterface
// 		var _ apps.AppManagerInterface = (*ArrayAppManager)(nil)
// 	})
//
// 	t.Run("AllMethodsReturnExpectedTypes", func(t *testing.T) {
// 		manager := &ArrayAppManager{}
// 		testApps := createTestAppsForArrayManager()
// 		manager.Init(testApps)
//
// 		// Test GetAllApps returns []apps.App
// 		allApps := manager.GetAllApps()
// 		var expectedType []apps.App
// 		assert.IsType(t, expectedType, allApps)
//
// 		// Test GetAppSecret returns string and error for non-existent app
// 		secret, err := manager.GetAppSecret("app4")
// 		assert.IsType(t, "", secret)
// 		assert.IsType(t, errors.New(""), err)
//
// 		// Test GetAppSecret returns string and error
// 		secret, err = manager.GetAppSecret("app1")
// 		assert.IsType(t, "", secret)
// 		assert.IsType(t, nil, err)
//
// 		// Test FindByID returns *apps.App and error
// 		app, err := manager.FindByID("app1")
// 		assert.IsType(t, &apps.App{}, app)
// 		assert.IsType(t, errors.New(""), err)
//
// 		// Test FindByID returns *apps.App and error
// 		app, err := manager.FindByID("app1")
// 		assert.IsType(t, &apps.App{}, app)
// 		assert.IsType(t, errors.New(""), err)
//
//
//
// 		// Test FindByKey returns *apps.App and error
// 		app, err = manager.FindByKey("key1")
// 		assert.IsType(t, &apps.App{}, app)
// 		assert.IsType(t, errors.New(""), err)
// 	})
// }

func TestArrayAppManager_Performance(t *testing.T) {
	t.Run("LargeNumberOfApps", func(t *testing.T) {
		manager := &ArrayAppManager{}

		// Create a large number of apps
		largeApps := make([]apps.App, 1000)
		for i := 0; i < 1000; i++ {
			largeApps[i] = createTestAppForArrayManager(
				fmt.Sprintf("app-%d", i),
				fmt.Sprintf("key-%d", i),
				fmt.Sprintf("secret-%d", i),
			)
		}

		err := manager.Init(largeApps)
		assert.NoError(t, err)
		assert.Len(t, manager.Apps, 1000)

		// Test that we can still find apps efficiently
		app, err := manager.FindByID("app-500")
		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, "app-500", string(app.ID))

		app, err = manager.FindByKey("key-500")
		assert.NoError(t, err)
		assert.NotNil(t, app)
		assert.Equal(t, "key-500", app.Key)
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		manager := &ArrayAppManager{}
		testApps := createTestAppsForArrayManager()
		manager.Init(testApps)

		// Test concurrent operations
		done := make(chan bool, 30)

		// 10 goroutines calling GetAllApps
		for i := 0; i < 10; i++ {
			go func() {
				allApps := manager.GetAllApps()
				assert.Len(t, allApps, 3)
				done <- true
			}()
		}

		// 10 goroutines calling GetAppSecret
		for i := 0; i < 10; i++ {
			go func() {
				secret, err := manager.GetAppSecret("app1")
				assert.NoError(t, err)
				assert.Equal(t, "secret1", secret)
				done <- true
			}()
		}

		// 10 goroutines calling FindByKey
		for i := 0; i < 10; i++ {
			go func() {
				app, err := manager.FindByKey("key1")
				assert.NoError(t, err)
				assert.NotNil(t, app)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 30; i++ {
			<-done
		}
	})
}
