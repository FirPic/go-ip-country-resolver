# Go Ip Country Resolver

Go Ip Country Resolver est une bibliothèque Go qui permet de localiser le pays d'origine d'une adresse IP en utilisant une base de données BoltDB. Cette solution est optimisée pour la performance et la consommation mémoire, idéale pour les applications nécessitant une géolocalisation IP légère mais efficace.

## Caractéristiques

- Stockage efficace des plages d'adresses IP dans une base de données BoltDB
- Conversion d'adresses IP en format numérique pour des recherches optimisées
- Mise en cache des résultats pour améliorer les performances
- Support des formats CIDR et des plages IP (début-fin)
- Importation de données depuis des fichiers de zones
- Filtrage automatique des plages d'adresses IP privées et locales
- API simple pour la recherche et la gestion des données

## Installation

```bash
go get github.com/FirPic/go-ip-country-resolver
```

## Utilisation

### Initialisation

```go
// Ouvrir la base de données
dbManager, err := goipcountryresolver.OpenDB("ip-country.db", false)
if err != nil {
    log.Fatalf("Erreur lors de l'ouverture de la base de données: %v", err)
}
defer dbManager.Close()

// Créer un localisateur avec une taille de cache de 1000 entrées
locator := goipcountryresolver.NewIPLocator(dbManager, 1000)
```

### Importation de données

```go
// Traiter un répertoire contenant des fichiers .zone
processed, updated, err := dbManager.ProcessDirectory("/path/to/zone-files")
if err != nil {
    log.Fatalf("Erreur lors du traitement du répertoire: %v", err)
}
log.Printf("Traités: %d, Mis à jour: %d", processed, updated)

// Ou traiter un seul fichier
processed, updated, err := dbManager.ProcessFile("/path/to/FR.zone")
if err != nil {
    log.Fatalf("Erreur lors du traitement du fichier: %v", err)
}
```

### Recherche du pays d'une adresse IP

```go
// Rechercher le pays pour une adresse IP
country, err := locator.FindCountryForIP("8.8.8.8")
if err != nil {
    log.Printf("Erreur lors de la recherche du pays: %v", err)
} else {
    log.Printf("Pays pour 8.8.8.8: %s", country)
}
```

### Obtention de toutes les plages IP pour un pays

```go
// Obtenir toutes les plages pour un pays
ranges, err := locator.GetAllRangesForCountry("FR")
if err != nil {
    log.Printf("Erreur lors de la récupération des plages: %v", err)
} else {
    log.Printf("Nombre de plages pour FR: %d", len(ranges))
    for _, r := range ranges {
        log.Printf("Plage: %s", r)
    }
}
```

## Architecture

La bibliothèque utilise trois buckets BoltDB principaux:

1. `ip_ranges` - Stocke les plages d'adresses IP au format texte (ex: "192.168.1.0-192.168.1.255" => "FR")
2. `ip_ranges_numeric` - Stocke les plages au format numérique pour des recherches optimisées
3. `ip_prefix_index` - Stocke des index de préfixes pour accélérer les recherches

Le système de cache intégré permet d'éviter des recherches répétées dans la base de données pour les adresses IP fréquemment consultées.

## Format des fichiers de zone

Chaque fichier de zone doit être nommé avec le code pays à deux lettres (ex: `FR.zone`) et contenir une liste de plages IP:

```
# Commentaire (ignoré)
1.0.0.0-1.0.0.255
2.0.0.0/24
```

Les plages privées sont automatiquement filtrées.

## Performance

- La conversion des adresses IP en format numérique permet des comparaisons efficaces
- L'indexation des plages d'adresses IP optimise les temps de recherche
- Le système de cache réduit la charge sur la base de données
- Traitement par lots pour l'importation rapide de grands volumes de données

## Licence
 <p xmlns:cc="http://creativecommons.org/ns#" xmlns:dct="http://purl.org/dc/terms/"><a property="dct:title" rel="cc:attributionURL" href="https://github.com/FirPic/go-ip-country-resolver">GoIpCountryLocator</a> by <a rel="cc:attributionURL dct:creator" property="cc:attributionName" href="https://github.com/FirPic">FirPic</a> is licensed under <a href="https://creativecommons.org/licenses/by-nc-sa/4.0/?ref=chooser-v1" target="_blank" rel="license noopener noreferrer" style="display:inline-block;">CC BY-NC-SA 4.0<img style="height:22px!important;margin-left:3px;vertical-align:text-bottom;" src="https://mirrors.creativecommons.org/presskit/icons/cc.svg?ref=chooser-v1" alt=""><img style="height:22px!important;margin-left:3px;vertical-align:text-bottom;" src="https://mirrors.creativecommons.org/presskit/icons/by.svg?ref=chooser-v1" alt=""><img style="height:22px!important;margin-left:3px;vertical-align:text-bottom;" src="https://mirrors.creativecommons.org/presskit/icons/nc.svg?ref=chooser-v1" alt=""><img style="height:22px!important;margin-left:3px;vertical-align:text-bottom;" src="https://mirrors.creativecommons.org/presskit/icons/sa.svg?ref=chooser-v1" alt=""></a></p> 

[CC BY-NC-SA 4.0 License](LICENSE)
