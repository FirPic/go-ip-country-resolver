package ipcountrylocator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

// DBManager gère l'accès à la base BoltDB (fichier, chemin et opérations de mise à jour).
type DBManager struct {
	DB     *bbolt.DB
	DBPath string
}

// openDatabase ouvre (ou crée) la base BoltDB située à dbPath.
// Paramètres:
//   - dbPath: chemin du fichier .db
//   - readOnly: si true ouverture en lecture seule (aucune création de buckets)
//
// Retourne un gestionnaire DBManager initialisé ou une erreur.
func openDatabase(dbPath string, readOnly bool) (*DBManager, error) {
	options := &bbolt.Options{
		Timeout:  1 * time.Second,
		NoSync:   false,
		ReadOnly: readOnly,
		PageSize: 4096,
	}

	db, err := bbolt.Open(dbPath, 0600, options)
	if err != nil {
		return nil, fmt.Errorf("error opening the database: %v", err)
	}

	manager := &DBManager{
		DB:     db,
		DBPath: dbPath,
	}

	if !readOnly {
		// Create necessary buckets
		err = manager.ensureBuckets()
		if err != nil {
			db.Close()
			return nil, err
		}
	}

	return manager, nil
}

// closeDatabase ferme proprement la connexion BoltDB.
// Retourne une erreur si la fermeture échoue.
func (m *DBManager) closeDatabase() error {
	return m.DB.Close()
}

// ensureBuckets garantit l'existence des buckets nécessaires (compatibilité texte, index numérique, index préfixes).
// Idempotent: recrée uniquement les buckets manquants.
// Retourne une erreur si une création échoue.
func (m *DBManager) ensureBuckets() error {
	return m.DB.Update(func(tx *bbolt.Tx) error {
		// Original bucket for compatibility
		if _, err := tx.CreateBucketIfNotExists([]byte("ip_ranges")); err != nil {
			return fmt.Errorf("error creating bucket ip_ranges: %v", err)
		}
		// Bucket for numeric ranges
		if _, err := tx.CreateBucketIfNotExists([]byte("ip_ranges_numeric")); err != nil {
			return fmt.Errorf("error creating bucket ip_ranges_numeric: %v", err)
		}
		// Bucket for prefix index
		if _, err := tx.CreateBucketIfNotExists([]byte("ip_prefix_index")); err != nil {
			return fmt.Errorf("error creating bucket ip_prefix_index: %v", err)
		}
		return nil
	})
}

// importZoneDirectory parcourt un dossier et importe chaque fichier *.zone (hors 'zz.zone').
// Agrège le nombre total de lignes valides lues (processed) et mises à jour (updated).
// Ignore les fichiers invalides en continuant le traitement.
// Retourne (processed, updated, error).
func (m *DBManager) importZoneDirectory(directory string) (int, int, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*.zone"))
	if err != nil {
		return 0, 0, fmt.Errorf("error searching for files: %v", err)
	}

	var totalProcessed, totalUpdated int

	for _, file := range files {
		// Skip specific files if necessary
		if !strings.Contains(file, "zz.zone") {
			processed, updated, err := m.importZoneFile(file)
			if err != nil {
				fmt.Printf("Error processing file %s: %v\n", file, err)
				continue
			}
			totalProcessed += processed
			totalUpdated += updated
		}
	}

	return totalProcessed, totalUpdated, nil
}

// importZoneFile importe un fichier .zone (plages ou CIDR) et écrit les données en base.
// Retourne (processed: lignes publiques lues, updated: entrées réellement écrites/modifiées, error).
func (m *DBManager) importZoneFile(file string) (int, int, error) {
	country_code := filepath.Base(file)
	country_code = country_code[:strings.Index(country_code, ".")]

	if country_code == "" {
		return 0, 0, fmt.Errorf("empty country code for file %s", file)
	}

	country_file, err := os.Open(file)
	if err != nil {
		return 0, 0, fmt.Errorf("error opening file %s: %v", file, err)
	}
	defer country_file.Close()

	processed := 0
	updated := 0
	skipped := 0

	// Batch processing
	const batchSize = 1000
	batch := make(map[string]string, batchSize)
	numericBatch := make([]IPRange, 0, batchSize)

	scanner := bufio.NewScanner(country_file)
	for scanner.Scan() {
		ipRange := strings.TrimSpace(scanner.Text())

		// Skip empty or commented lines
		if ipRange == "" || strings.HasPrefix(ipRange, "#") || strings.HasPrefix(ipRange, "//") {
			continue
		}

		// Check if it's a private or local range
		if isPrivateOrLocalCIDR(ipRange) {
			skipped++
			continue
		}

		processed++

		// Convert to numeric format
		start, end, err := parseIPRange(ipRange)
		if err != nil {
			skipped++
			continue
		}

		// Add to batch
		batch[ipRange] = country_code
		numericBatch = append(numericBatch, IPRange{
			Start:   start,
			End:     end,
			Country: country_code,
		})

		// When the batch is full, commit it to the database
		if len(batch) >= batchSize {
			u, err := m.writeBatch(batch, numericBatch)
			if err != nil {
				fmt.Printf("Error updating batch: %v\n", err)
			}
			updated += u
			batch = make(map[string]string, batchSize)
			numericBatch = make([]IPRange, 0, batchSize)
		}
	}

	// Commit the last batch if there are remaining data
	if len(batch) > 0 {
		u, err := m.writeBatch(batch, numericBatch)
		if err != nil {
			fmt.Printf("Error updating last batch: %v\n", err)
		}
		updated += u
	}

	if err := scanner.Err(); err != nil {
		return processed, updated, fmt.Errorf("error reading file: %v", err)
	}

	return processed, updated, nil
}

// writeBatch applique un lot d'insertions/mises à jour dans les deux représentations:
//   - bucket texte "ip_ranges"
//   - bucket binaire "ip_ranges_numeric" (clé: start|end sur 8 octets big-endian)
//
// Retourne le nombre d'entrées mises à jour (texte) et une erreur éventuelle.
func (m *DBManager) writeBatch(batch map[string]string, numericBatch []IPRange) (int, error) {
	updated := 0
	err := m.DB.Batch(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ip_ranges"))
		numericBucket := tx.Bucket([]byte("ip_ranges_numeric"))

		if bucket == nil || numericBucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Store in the original bucket
		for ipRange, countryCode := range batch {
			existingCountry := string(bucket.Get([]byte(ipRange)))
			if existingCountry != countryCode {
				if err := bucket.Put([]byte(ipRange), []byte(countryCode)); err != nil {
					return err
				}
				updated++
			}
		}

		// Store numeric ranges
		for _, ipRange := range numericBatch {
			key := make([]byte, 8)
			encodeUint32BE(key[0:4], ipRange.Start)
			encodeUint32BE(key[4:8], ipRange.End)

			existingValue := numericBucket.Get(key)
			if existingValue == nil || string(existingValue) != ipRange.Country {
				if err := numericBucket.Put(key, []byte(ipRange.Country)); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return updated, err
}

// upsertIPRangeCountry associe (ou ré-associe) une plage à un pays.
// Retourne true si succès, sinon false + erreur.
func (m *DBManager) upsertIPRangeCountry(ipRange string, start, end uint32, countryCode string) (bool, error) {
	success := false

	err := m.DB.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ip_ranges"))
		numericBucket := tx.Bucket([]byte("ip_ranges_numeric"))

		if bucket == nil || numericBucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Update the original bucket
		if err := bucket.Put([]byte(ipRange), []byte(countryCode)); err != nil {
			return err
		}

		// Update the numeric bucket
		key := make([]byte, 8)
		encodeUint32BE(key[0:4], start)
		encodeUint32BE(key[4:8], end)

		if err := numericBucket.Put(key, []byte(countryCode)); err != nil {
			return err
		}

		success = true
		return nil
	})

	return success, err
}

// verifyRangeIndexes vérifie l'ordre des plages numériques.
// Retourne le nombre total de plages numérisées et une erreur de lecture éventuelle.
// Affiche un avertissement si des inversions d'ordre sont détectées.
func (m *DBManager) verifyRangeIndexes() (int, error) {
	count := 0
	var lastStart uint32 = 0
	var warnings int = 0

	err := m.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ip_ranges_numeric"))
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if len(k) >= 4 {
				start := decodeUint32BE(k[0:4])

				// Check that ranges are sorted by start address
				if count > 0 && start < lastStart {
					warnings++
				}

				lastStart = start
				count++
			}
		}

		return nil
	})

	if warnings > 0 {
		fmt.Printf("Warning: %d IP ranges are not correctly sorted\n", warnings)
	}

	return count, err
}
