package ipcountrylocator

import (
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

// setupTestDB crée une base temporaire isolée pour un test et retourne un gestionnaire + dossier + fonction de nettoyage.
func setupTestDB(t *testing.T) (*DBManager, string, func()) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "ip-country-test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")

	// Open a test database
	manager, err := openDatabase(dbPath, false)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		manager.closeDatabase()
		os.RemoveAll(tempDir)
	}

	return manager, tempDir, cleanup
}

// createTestZoneFile génère un fichier .zone de test contenant les ranges fournis pour un pays donné.
func createTestZoneFile(dir, countryCode string, ranges []string) (string, error) {
	filePath := filepath.Join(dir, countryCode+".zone")
	f, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	for _, r := range ranges {
		f.WriteString(r + "\n")
	}

	return filePath, nil
}

func TestOpenDB(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	if manager.DB == nil {
		t.Error("The database was not opened correctly")
	}
}

func TestCreateBuckets(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Check that buckets exist
	err := manager.DB.View(func(tx *bbolt.Tx) error {
		buckets := []string{"ip_ranges", "ip_ranges_numeric", "ip_prefix_index"}
		for _, name := range buckets {
			bucket := tx.Bucket([]byte(name))
			if bucket == nil {
				t.Errorf("The bucket %s was not created", name)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Error when checking buckets: %v", err)
	}
}

func TestProcessFile(t *testing.T) {
	manager, tempDir, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a test file
	ranges := []string{
		"1.0.0.0-1.0.0.255",
		"2.0.0.0-2.0.0.255",
		"# Comment to ignore",
		"192.168.0.0-192.168.0.255", // Private IP that will be ignored
	}

	filePath, err := createTestZoneFile(tempDir, "FR", ranges)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	processed, updated, err := manager.importZoneFile(filePath)
	if err != nil {
		t.Fatalf("Error processing file: %v", err)
	}

	// Check that public IP ranges were processed
	if processed != 3 { // 2 public ranges + 1 private
		t.Errorf("Incorrect number of processed ranges. Expected: 3, Got: %d", processed)
	}

	// Check that IP ranges were updated
	if updated < 2 {
		t.Errorf("Incorrect number of updates. Expected at least: 2, Got: %d", updated)
	}

	// Check that data was correctly stored
	err = manager.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ip_ranges"))
		if bucket == nil {
			return nil
		}

		country := string(bucket.Get([]byte("1.0.0.0-1.0.0.255")))
		if country != "FR" {
			t.Errorf("Incorrect country for range. Expected: FR, Got: %s", country)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Error when checking data: %v", err)
	}
}

func TestProcessDirectory(t *testing.T) {
	manager, tempDir, cleanup := setupTestDB(t)
	defer cleanup()

	// Create multiple test files
	countries := map[string][]string{
		"FR": {"1.0.0.0-1.0.0.255", "2.0.0.0-2.0.0.255"},
		"DE": {"3.0.0.0-3.0.0.255", "4.0.0.0-4.0.0.255"},
		"zz": {"5.0.0.0-5.0.0.255"}, // Should be ignored
	}

	for country, ranges := range countries {
		_, err := createTestZoneFile(tempDir, country, ranges)
		if err != nil {
			t.Fatalf("Failed to create test file for %s: %v", country, err)
		}
	}

	processed, _, err := manager.importZoneDirectory(tempDir)
	if err != nil {
		t.Fatalf("Error processing directory: %v", err)
	}

	// Check that files were correctly processed (without zz.zone)
	if processed != 4 { // 2 ranges for FR + 2 ranges for DE
		t.Errorf("Incorrect number of processed ranges. Expected: 4, Got: %d", processed)
	}
}

func TestUpdateIPRangeCountry(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	ipRange := "1.0.0.0-1.0.0.255"
	start, end, _ := parseIPRange(ipRange)

	// Add an IP range
	success, err := manager.upsertIPRangeCountry(ipRange, start, end, "FR")
	if err != nil {
		t.Fatalf("Error updating IP range: %v", err)
	}

	if !success {
		t.Error("IP range update failed")
	}

	// Check that the range was correctly added
	err = manager.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ip_ranges"))
		if bucket == nil {
			return nil
		}

		country := string(bucket.Get([]byte(ipRange)))
		if country != "FR" {
			t.Errorf("Incorrect country for range. Expected: FR, Got: %s", country)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Error when checking data: %v", err)
	}
}

func TestVerifyIndexes(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Add some ranges to test index verification
	ipRanges := []struct {
		ipRange string
		country string
	}{
		{"1.0.0.0-1.0.0.255", "FR"},
		{"2.0.0.0-2.0.0.255", "DE"},
		{"3.0.0.0-3.0.0.255", "US"},
	}

	for _, r := range ipRanges {
		start, end, _ := parseIPRange(r.ipRange)
		_, err := manager.upsertIPRangeCountry(r.ipRange, start, end, r.country)
		if err != nil {
			t.Fatalf("Error adding IP range: %v", err)
		}
	}

	count, err := manager.verifyRangeIndexes()
	if err != nil {
		t.Fatalf("Error verifying indexes: %v", err)
	}

	if count != 3 {
		t.Errorf("Incorrect index count. Expected: 3, Got: %d", count)
	}
}
