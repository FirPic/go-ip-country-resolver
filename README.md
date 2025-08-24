# go-ip-country-resolver

Bibliothèque Go pour résoudre rapidement le pays associé à une adresse IPv4 à partir de fichiers de zones (plages start-end ou CIDR) importés dans BoltDB avec double index (texte + représentation numérique).

## Sommaire
- Objectifs
- Fonctionnalités
- Architecture & Buckets
- Import & Flux
- API Publique
- Exemples
- Format des fichiers .zone
- Détails techniques
- Performances & Optimisations futures
- Tests
- Roadmap
- Licence

## Objectifs
- Stockage persistant compact des plages IP.
- Recherche rapide via conversion numérique (uint32) + scan ordonné.
- Cache mémoire léger.
- Import par lots résilient (ignore lignes invalides / privées).

## Fonctionnalités
- Import de répertoires *.zone (exclusion automatique de zz.zone).
- Support formats: "start-end" ou CIDR.
- Filtrage plages privées / loopback / link-local.
- Double index (forme originale + clé numérique 8 octets).
- Vérification de l’ordre des plages.
- Extraction de toutes les plages d’un pays.
- API publique simple (wrappers exportés).

## Architecture & Buckets
Buckets BoltDB:
1. ip_ranges : clé = chaîne "start-end" ou CIDR original, valeur = code pays.
2. ip_ranges_numeric : clé = 8 octets (start(4) big-endian + end(4)), valeur = code pays.
3. ip_prefix_index : réservé (pré-indexation future).

Structures:
- DBManager : ouverture, import, upsert, vérif d’index.
- IPLocator : résolution IP + cache.
- IPCache : map bornée réinitialisée quand pleine.
- IPRange : représentation start/end numériques.

## Import & Flux
Fichier .zone -> lecture ligne à ligne:
1. Ignore commentaires / vide.
2. Filtre plages privées.
3. parseIPRange -> (start,end).
4. Ajout en batch (1000) -> writeBatch:
   - ip_ranges (forme texte)
   - ip_ranges_numeric (clé binaire).
Lookup: cache -> index numérique ordonné -> fallback scan texte.

## API Publique

Principales fonctions exportées (package ipcountrylocator):

```go
OpenDatabase(path string, readOnly bool) (*DBManager, error)
(*DBManager) Close() error
(*DBManager) ImportDirectory(dir string) (processed, updated int, err error)
(*DBManager) ImportFile(file string) (processed, updated int, err error)
(*DBManager) UpsertRange(rangeStr string, start, end uint32, country string) (bool, error)
(*DBManager) VerifyNumericIndex() (count int, err error)

NewLocator(mgr *DBManager, cacheSize int) *IPLocator
(*IPLocator) Lookup(ip string) (country string, err error)
(*IPLocator) Ranges(country string) ([]string, error)

ParseRange(rangeStr string) (startUint32, endUint32, error)
```

Nota:
- ParseRange aide à dériver start/end si vous ajoutez manuellement une plage.
- Les codes pays attendus sont ISO 3166-1 alpha-2 (ex: "FR", "US").

## Exemples

### Import d’un répertoire complet et résolution
```go
mgr, err := ipcountrylocator.OpenDatabase("ipcountry.db", false)
if err != nil { log.Fatal(err) }
defer mgr.Close()

processed, updated, err := mgr.ImportDirectory("./zones")
if err != nil { log.Fatal(err) }
log.Printf("Import: %d lignes valides, %d écritures\n", processed, updated)

locator := ipcountrylocator.NewLocator(mgr, 5000)

country, err := locator.Lookup("8.8.8.8")
if err != nil {
    log.Println("IP inconnue:", err)
} else {
    log.Println("Pays:", country)
}

frRanges, _ := locator.Ranges("FR")
log.Printf("Plages FR: %d\n", len(frRanges))
```

### Ajout manuel d’une plage
```go
start, end, err := ipcountrylocator.ParseRange("203.0.113.0/24")
if err != nil { log.Fatal(err) }

_, err = mgr.UpsertRange("203.0.113.0/24", start, end, "EX")
if err != nil { log.Fatal(err) }
```

### Vérification de l’index numérique
```go
count, err := mgr.VerifyNumericIndex()
if err != nil {
    log.Fatal(err)
}
log.Printf("Index numérique contient %d plages\n", count)
```

## Format des fichiers .zone
Nom: CC.zone (CC = code pays ISO 2 lettres).
Contenu (exemple):
```
# Commentaires ignorés
1.0.0.0-1.0.0.255
8.8.8.0/24
192.168.0.0/16         # Privé -> ignoré
```
Lignes invalides ou privées: ignorées (non bloquant).

## Détails techniques
- parseIPRange détecte CIDR vs start-end.
- CIDR -> calcul end via masque: end = start | ((1<<hostBits)-1).
- Clé numérique 8 octets triée naturellement par start.
- encodeUint32BE / decodeUint32BE: big-endian pour cohérence lexicographique.

## Performances & Optimisations futures
Actuel:
- Parcours séquentiel des ranges numériques (adapté volume modéré).
Optimisations possibles:
- Recherche binaire (dichotomie) sur ip_ranges_numeric.
- Pré-indexation par préfixes (ip_prefix_index).
- Fusion de plages contiguës.
- Support IPv6 (128 bits).
- Cache LRU réel (actuel = reset simple).

## Tests
Exécuter:
```bash
go test ./...
```
Couvre: parsing, conversion, inclusion, import fichiers & répertoires, upsert, vérification d’index, cache, lookup.

## Roadmap
- IPv6
- Index préfixe (/24 ou adaptatif)
- Recherche binaire
- Service HTTP + CLI
- Compression des plages
- Métriques (hits cache, temps lookup)

## Licence
CC BY-NC-SA 4.0 (voir LICENSE).
