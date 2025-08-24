package ipcountrylocator

// OpenDatabase ouvre (ou crée) la base BoltDB et garantit les buckets si lecture/écriture.
// readOnly = true désactive la création de buckets.
// Retourne un *DBManager prêt à l'emploi.
func OpenDatabase(path string, readOnly bool) (*DBManager, error) {
	return openDatabase(path, readOnly)
}

// Close ferme proprement la base BoltDB.
func (m *DBManager) Close() error {
	return m.closeDatabase()
}

// ImportDirectory importe tous les fichiers *.zone d'un répertoire (ignore zz.zone).
// Retourne (processedLines, updatedEntries, error).
func (m *DBManager) ImportDirectory(dir string) (int, int, error) {
	return m.importZoneDirectory(dir)
}

// ImportFile importe un fichier .zone unique.
// Retourne (processedLines, updatedEntries, error).
func (m *DBManager) ImportFile(file string) (int, int, error) {
	return m.importZoneFile(file)
}

// UpsertRange insère ou remplace une plage IP (format "start-end" ou CIDR) pour un pays.
// start/end doivent être fournis (utiliser ParseRange pour les dériver).
// Retourne (true si succès, error).
func (m *DBManager) UpsertRange(rangeStr string, start, end uint32, country string) (bool, error) {
	return m.upsertIPRangeCountry(rangeStr, start, end, country)
}

// VerifyNumericIndex vérifie l'ordre des clés du bucket numérique.
// Retourne (count, error).
func (m *DBManager) VerifyNumericIndex() (int, error) {
	return m.verifyRangeIndexes()
}

// NewLocator crée un localisateur IP avec cache mémoire (taille en entrées).
func NewLocator(mgr *DBManager, cacheSize int) *IPLocator {
	return newIPLocator(mgr, cacheSize)
}

// Lookup résout le code pays (ISO 2 lettres attendu dans les données) pour une IPv4.
// Recherche: cache -> index numérique -> fallback scan texte.
func (l *IPLocator) Lookup(ip string) (string, error) {
	return l.lookupCountryByIP(ip)
}

// Ranges retourne toutes les plages (forme texte originale) associées à un pays.
func (l *IPLocator) Ranges(country string) ([]string, error) {
	return l.listIPRangesByCountry(country)
}

// ParseRange parse une plage "start-end" OU un CIDR et retourne (startUint32, endUint32, error).
func ParseRange(rangeStr string) (uint32, uint32, error) {
	return parseIPRange(rangeStr)
}
