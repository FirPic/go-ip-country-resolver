# go-ip-country-resolver

Résolution rapide du pays pour une IPv4 à partir de fichiers `.zone` (plages `start-end` ou CIDR) importés dans BoltDB avec un double index (texte + représentation numérique).  
API publique simple, prête à intégrer dans un service, une CLI ou un middleware réseau.

---

## 1. Installation

```bash
go get github.com/FirPic/go-ip-country-resolver
```

Import:
```go
import "github.com/FirPic/go-ip-country-resolver/ipcountrylocator"
```

---

## 2. Vue d’ensemble

Pipeline:
1. Fichiers `CC.zone` (ISO 3166-1 alpha-2) placés dans un répertoire.
2. Import par lot → stockage:
   - Bucket `ip_ranges` (clé texte originale).
   - Bucket `ip_ranges_numeric` (clé binaire 8 octets start|end big-endian).
3. Lookup:
   - Cache mémoire (clé IP string).
   - Scan séquentiel du bucket numérique (tri implicite).
   - Fallback sur bucket texte (sécurité).
4. API publique = wrappers stables; logique interne masquée.

---

## 3. Format des fichiers `.zone`

Nom: `FR.zone`, `US.zone`, …  
Contenu:
```
# Commentaires ignorés
1.0.0.0-1.0.0.255
8.8.8.0/24
192.168.0.0/16     # Privé -> ignoré
```

Règles:
- Lignes vides / débutant par `#` ou `//` ignorées.
- Ranges privées / loopback / link-local ignorées (aucune erreur).
- Formats acceptés: `A.B.C.D-E.F.G.H` ou `CIDR`.

---

## 4. Types principaux (publics)

- DBManager: encapsule la base BoltDB + opérations d’import / maintenance.
- IPLocator: moteur de résolution + cache.
- IPRange (interne) non nécessaire à l’API publique.
- Cache: implémentation simple (reset quand plein).

---

## 5. API Publique (Référence détaillée)

### 5.1 Base / Import

#### OpenDatabase(path string, readOnly bool) (*DBManager, error)
Ouvre (et crée si besoin) la base:
- path: chemin du fichier `.db`
- readOnly = true: interdit création de buckets / écritures
Erreurs: permissions, lock concurrent, chemin invalide.

#### (m *DBManager) Close() error
Ferme proprement BoltDB. Toujours appeler avec `defer`.

#### (m *DBManager) ImportDirectory(dir string) (processed, updated int, err error)
Parcourt `dir`, importe chaque `*.zone` sauf `zz.zone`.
- processed: lignes publiques valides lues.
- updated: écritures effectives (nouvelles ou modifiées).
Continue en cas d’erreurs partielles (log possible côté appelant).

#### (m *DBManager) ImportFile(file string) (processed, updated int, err error)
Import ciblé d’un seul fichier `.zone`.

#### (m *DBManager) UpsertRange(rangeStr string, start, end uint32, country string) (bool, error)
Insertion / remplacement manuel d’une plage.
- Utiliser ParseRange(rangeStr) pour dériver start/end.
- Retourne true si succès (actuellement toujours true si pas d’erreur).

#### (m *DBManager) VerifyNumericIndex() (count int, err error)
Parcourt `ip_ranges_numeric`, vérifie l’ordre non décroissant de `start`.
- count: nombre d’entrées vues.
- Log interne d’avertissements si désordres.

### 5.2 Résolution

#### NewLocator(mgr *DBManager, cacheSize int) *IPLocator
Construit un localisateur lié à une base ouverte.
- cacheSize: taille maxi avant reset intégral du cache.

#### (l *IPLocator) Lookup(ip string) (country string, err error)
Résout une IPv4 (ex: `"8.8.8.8"`).  
Chemin: cache → bucket numérique → fallback texte.  
Erreurs: IP invalide, non trouvée.

#### (l *IPLocator) Ranges(country string) ([]string, error)
Retourne toutes les chaînes originales (`start-end` ou CIDR) associées au code.

### 5.3 Utilitaires

#### ParseRange(rangeStr string) (start uint32, end uint32, err error)
Normalise une plage en deux bornes inclusives (uint32).
- Accepte CIDR ou `start-end`.
- Erreurs: format invalide, adresses invalides.

---

## 6. Exemples

### 6.1 Import + lookup basique
```go
mgr, err := ipcountrylocator.OpenDatabase("ipcountry.db", false)
if err != nil { panic(err) }
defer mgr.Close()

processed, updated, err := mgr.ImportDirectory("./zones")
if err != nil { panic(err) }

locator := ipcountrylocator.NewLocator(mgr, 10_000)

country, err := locator.Lookup("8.8.8.8")
if err != nil {
    fmt.Println("Non résolu:", err)
} else {
    fmt.Println("Pays:", country)
}
```

### 6.2 Ajout manuel d’une plage
```go
start, end, err := ipcountrylocator.ParseRange("203.0.113.0/24")
if err != nil { log.Fatal(err) }

_, err = mgr.UpsertRange("203.0.113.0/24", start, end, "EX")
if err != nil { log.Fatal(err) }
```

### 6.3 Vérification de cohérence
```go
count, err := mgr.VerifyNumericIndex()
if err != nil { log.Fatal(err) }
log.Printf("Index numérique: %d plages", count)
```

### 6.4 Récupération de toutes les plages d’un pays
```go
fr, err := locator.Ranges("FR")
if err != nil { log.Fatal(err) }
for _, r := range fr {
    fmt.Println(r)
}
```

---

## 7. Gestion des erreurs

Catégories:
- Ouverture DB: permission, verrou concurrent.
- Parsing: formats invalides de ligne (ignorés pendant import).
- Lookup: IP invalide / inconnue → erreur explicite.
- Upsert: erreurs I/O BoltDB (rare).

Stratégie import: lignes invalides ignorées silencieusement (compteur `processed` exclut lignes commentées mais inclut lignes privées avant filtrage ? → Non: privées = ignorées + non ajoutées; elles n’incrémentent pas `updated`).

---

## 8. Concurrence

- BoltDB: multiple lecteurs simultanés OK, un seul writer.  
- Lookup → DB.View (lecture) + cache thread-safe (RWMutex).  
- Import (écriture) à séparer des flux fortement concurrents de lookup dans un service critique: envisager fenêtre de maintenance ou ré-ouvrir DB dans un processus distinct.

---

## 9. Performance (actuelle)

- Scan linéaire du bucket numérique (O(N)) jusqu’à la plage adéquate.
- Cache IP direct (map limitée).
- Batches d’écriture (1000) réduisent la pression sur BoltDB.

Optimisations futures possibles:
- Recherche binaire (cursor seek + dichotomie sur clé 8 octets).
- Index préfixe (/16, /24 adaptatif) → bucket `ip_prefix_index`.
- Fusion automatique de plages contiguës (réduction cardinalité).
- Compression (delta + varint).
- IPv6 (128 bits → 16 octets clé).

---

## 10. Bonnes pratiques

- Toujours `defer mgr.Close()`.
- Exécuter `VerifyNumericIndex()` après gros imports externes.
- Limiter `cacheSize` selon mémoire (clé+valeur petites).
- Pré-valider vos fichiers (pas de ranges chevauchées si vous visez cohérence stricte).
- Versionner votre fichier .db si utilisé en production (backup régulier).

---

## 11. FAQ

Q: Pourquoi les fonctions internes sont en camelCase non exportées ?  
R: Séparation nette noyau interne / API publique stable (wrappers).

Q: Que faire si une IP n’est pas trouvée ?  
R: Retour erreur; gérez côté appelant (ex: "XX"/"UNK" par défaut).

Q: Puis-je injecter IPv6 ?  
R: Non (pour l’instant). Ajout nécessitera un second schéma (16+16 bytes).

Q: Le cache devient-il incohérent après UpsertRange ?  
R: Oui potentiellement pour les IP déjà résolues; invalidez manuellement en recréant le locator si cohérence stricte nécessaire.

---

## 12. Tests

Lancer:
```bash
go test ./...
```
Couvre: parsing, encodage, inclusion, import, upsert, cache, lookup, index.

---

## 13. Roadmap

- IPv6
- Index préfixe
- Recherche binaire
- CLI + service HTTP
- Fusion / normalisation des plages
- Métriques (hits cache / durée lookup)

---

## 14. Licence

Licence: CC BY-NC-SA 4.0 (adapter selon vos besoins si usage commercial).  
Ajouter un fichier LICENSE différent si redistribution commerciale planifiée.

---

## 15. Résumé ultra-rapide

```go
mgr,_ := ipcountrylocator.OpenDatabase("db/ip.db", false)
defer mgr.Close()
mgr.ImportDirectory("./zones")
loc := ipcountrylocator.NewLocator(mgr, 5000)
country,_ := loc.Lookup("1.0.0.8")
fmt.Println(country)
```

Prêt à l’emploi.
