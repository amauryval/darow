export const fr = {
    nav: {
        app: 'darow',
        tables: 'Tables',
        filters: 'Filtres',
        sql: 'Requête SQL',
        sources: 'Sources',
        settings: 'Réglages'
    },
    pages: {
        tables: 'Tables',
        filters: 'Filtres enregistrés',
        sql: 'Créer une table par SQL',
        sources: 'Sources',
        settings: 'Réglages'
    },
    actions: {
        cancel: 'Annuler',
        confirm: 'Valider',
        save: 'Enregistrer'
    },
    tables: {
        search: 'Rechercher une table',
        select: 'Choisissez une table',
        empty: 'Aucune table',
        loadFailed: 'Chargement des tables impossible'
    },
    table: {
        id: 'N°',
        noRow: 'Aucune ligne',
        map: 'Carte',
        grid: 'Tableau',
        csv: 'CSV',
        geojson: 'GeoJSON',
        rename: 'Renommer la table',
        name: 'Nom de la table',
        renamed: 'Table renommée',
        renameFailed: 'Renommage impossible',
        delete: 'Supprimer la table',
        confirmDelete: 'Supprimer la table « {name} », ses lignes et ses filtres ?',
        deleted: 'Table supprimée',
        deleteFailed: 'Suppression impossible',
        loadFailed: 'Chargement de la table impossible'
    },
    links: {
        title: 'Les adresses de la table pour un autre outil',
        copy: 'Copier le lien',
        copied: 'Lien copié',
        copyFailed: 'Copie impossible',
        filtered: 'avec les filtres actifs',
        qgis: {
            label: 'QGIS — service OGC API - Features',
            hint: 'Couche > Ajouter une couche WFS / OGC API - Features, nouvelle connexion ; sans filtre'
        },
        geojson: { label: 'GeoJSON', hint: 'OpenLayers, MapLibre, Leaflet, fetch' },
        tiles: {
            label: 'Tuiles vectorielles (MVT)',
            hint: 'MapLibre, OpenLayers ; couche « features »'
        },
        csv: { label: 'CSV', hint: 'Tableur, pandas, fetch' }
    },
    export: {
        title: 'Exporter la table',
        format: 'Format',
        srid: 'Code EPSG',
        sridHint: 'Vide : longitude et latitude (CRS84) ; 4326 aussi, longitude en premier',
        submit: 'Exporter'
    },
    wkt: {
        title: 'Géométrie en WKT',
        srid: 'Code EPSG',
        sridHint: 'Vide : longitude et latitude (CRS84)',
        project: 'Reprojeter',
        copy: 'Copier le WKT',
        copied: 'WKT copié',
        copyFailed: 'Copie impossible',
        empty: 'Aucune géométrie',
        loadFailed: 'Lecture du WKT impossible'
    },
    row: {
        title: 'Ligne {id}',
        new: 'Nouvelle ligne',
        open: 'Ouvrir la ligne',
        delete: 'Supprimer la ligne',
        confirmDelete: 'Supprimer cette ligne ?',
        saved: 'Ligne enregistrée',
        saveFailed: 'Enregistrement impossible',
        deleted: 'Ligne supprimée',
        deleteFailed: 'Suppression impossible',
        loadFailed: 'Lecture de la ligne impossible'
    },
    mapEdit: {
        toggle: 'Éditer',
        title: 'Éditer les géométries sur la carte : un clic sur un objet le prend en main',
        create: 'Nouvel objet',
        save: 'Enregistrer la géométrie',
        cancel: 'Annuler les modifications',
        confirmDiscard: 'Abandonner les modifications de la géométrie ?',
        editing: 'Objet {id}',
        drawing: 'Nouvel objet : dessinez-le, ou cliquez sur un objet pour l’éditer'
    },
    selection: {
        filter: 'Sélection ({count})',
        only: 'Ne montrer que la sélection (rangées, carte, exports)',
        clear: 'Vider la sélection',
        mapHint:
            'Sélectionner : un clic ajoute ou retire un objet, un cadre tracé ajoute ceux qu’il contient (Maj + glisser hors de ce mode)',
        recenter: 'Recentrer sur l’emprise de la table'
    },
    filters: {
        title: 'Filtres',
        conditions: 'Condition',
        where: 'Condition SQL',
        wherePlaceholder:
            '"population" > 10000 AND ("nom" ILIKE \'%sur-mer%\' OR "code" IN (\'29\', \'56\'))',
        insertColumn: 'Insérer une colonne',
        unapply: 'Retirer la condition',
        apply: 'Appliquer',
        save: 'Enregistrer le filtre',
        saveDraft: 'Enregistrer cette condition comme filtre',
        name: 'Nom du filtre',
        table: 'Table',
        new: 'Nouveau filtre',
        search: 'Rechercher un filtre',
        select: 'Choisissez un filtre ou créez-en un',
        empty: 'Aucun filtre enregistré',
        delete: 'Supprimer le filtre',
        openTable: 'Ouvrir la table avec ce filtre',
        manage: 'Gérer les filtres enregistrés',
        anyTable: 'toute table',
        allTables: 'Toutes les tables (celles qui ont ses colonnes)',
        confirmDelete: 'Supprimer le filtre « {name} » ?',
        saveFailed: 'Enregistrement du filtre impossible',
        saved: 'Filtre enregistré',
        deleted: 'Filtre supprimé',
        deleteFailed: 'Suppression du filtre impossible',
        loadFailed: 'Chargement des filtres impossible'
    },
    sources: {
        upload: 'Ajouter une source',
        file: 'Fichier',
        send: 'Envoyer',
        uploaded: 'Source « {name} » ajoutée',
        uploadFailed: 'Envoi impossible',
        empty: 'Aucune source',
        name: 'Fichier',
        size: 'Taille',
        modifiedAt: 'Modifié le',
        load: 'Charger dans une table',
        loadTitle: 'Charger « {name} »',
        table: 'Nom de la table',
        tableHint: 'Minuscules, chiffres et _ ; ne commence pas par un chiffre',
        layer: 'Couche',
        srid: 'Code EPSG',
        sridHint: 'Vide : lu dans le fichier',
        loaded: 'Table « {table} » créée',
        loadIntoFailed: 'Chargement impossible',
        delete: 'Supprimer la source',
        confirmDelete:
            'Supprimer la source « {name} » ? Les tables chargées depuis elle sont gardées.',
        deleted: 'Source supprimée',
        deleteFailed: 'Suppression impossible',
        layersFailed: 'Lecture des couches impossible',
        loadFailed: 'Chargement des sources impossible'
    },
    sql: {
        title: 'Nouvelle table',
        name: 'Nom de la table créée',
        tables: 'Tables lues',
        chooseTables: 'Choisir les tables dont voir les champs',
        insertHint: 'Un clic insère le nom dans la requête',
        docs: 'Documentation DuckDB',
        docLinks: {
            sql: 'Introduction au SQL',
            select: 'SELECT',
            functions: 'Fonctions',
            spatial: 'Fonctions spatiales'
        },
        query: 'Requête SQL',
        placeholder: 'SELECT * FROM ma_table WHERE …',
        run: 'Créer la table',
        created: 'Table « {name} » créée',
        failed: 'Création impossible'
    },
    geometry: {
        asText: 'GeoJSON',
        geojson: 'Géométrie (GeoJSON, longitude et latitude)',
        invalid: 'GeoJSON illisible'
    },
    columns: {
        title: 'Colonnes',
        list: 'Colonnes de la table',
        add: 'Ajouter une colonne',
        name: 'Nom de la colonne',
        type: 'Type',
        rename: 'Renommer la colonne',
        delete: 'Supprimer la colonne',
        confirmDelete: 'Supprimer la colonne « {name} » et ses valeurs ?',
        transform: 'Transformer une colonne',
        target: 'Colonne transformée',
        expression: 'Nouvelle valeur (expression SQL)',
        expressionPlaceholder: 'upper(nom), population * 2, …',
        onAll: 'Sur toutes les lignes',
        onFiltered: 'Sur les lignes du filtre appliqué',
        preview: 'Aperçu',
        apply: 'Transformer',
        matched: '{count} lignes concernées',
        before: 'Avant',
        after: 'Après',
        added: 'Colonne « {name} » ajoutée',
        addFailed: 'Ajout de la colonne impossible',
        renamed: 'Colonne renommée',
        renameFailed: 'Renommage de la colonne impossible',
        deleted: 'Colonne supprimée',
        deleteFailed: 'Suppression de la colonne impossible',
        transformed: 'Colonne transformée',
        transformFailed: 'Transformation impossible',
        previewFailed: 'Aperçu impossible'
    },
    settings: {
        globalHint: 'Valeurs de toutes les tables, sauf celles qu’une table surcharge.',
        tableTitle: 'Réglages de la table',
        tableHint: 'Vide : la valeur globale, indiquée en grisé.',
        saved: 'Réglages enregistrés',
        saveFailed: 'Enregistrement des réglages impossible',
        loadFailed: 'Lecture des réglages impossible',
        fields: {
            rowsPageSize: 'Lignes par page',
            featuresPageSize: 'Entités par page (OGC)',
            coordinateDecimals: 'Décimales des coordonnées',
            tileCacheSeconds: 'Cache des tuiles (secondes)'
        },
        hints: {
            rowsPageSize: 'De 10 à 1000',
            featuresPageSize: 'De 1 à 10000, par défaut d’une page OGC API - Features',
            coordinateDecimals: 'De 0 à 15, pour le GeoJSON servi par l’OGC API',
            tileCacheSeconds: 'De 0 (aucun) à 86400'
        }
    },
    errors: {
        invalid_table_name: 'nom de table invalide',
        unknown_column: 'colonne « {column} » inconnue',
        invalid_operator: 'opérateur « {operator} » inapplicable',
        invalid_value: 'valeur illisible pour la colonne',
        invalid_query: 'requête refusée : {message}',
        unknown_crs: 'système de coordonnées introuvable dans le fichier, indiquez son code EPSG',
        unreadable_source: 'fichier illisible',
        invalid_format: 'format « {format} » indisponible',
        invalid_source_name: 'type de fichier non accepté',
        source_too_large: 'fichier trop volumineux (au plus {max} octets)',
        name_required: 'le nom est requis',
        where_required: 'la condition est requise',
        invalid_geometry: 'géométrie GeoJSON invalide',
        invalid_column_type: 'type « {type} » non proposé',
        invalid_column_name: 'nom de colonne « {column} » invalide ou déjà pris',
        invalid_expression: 'expression refusée : {message}',
        out_of_range: 'valeur hors de [{min}, {max}]'
    },
    erasures: {
        tables: {
            label: 'Tables',
            description: 'Toutes les tables, leurs lignes, leurs filtres et leurs réglages'
        },
        filters: {
            label: 'Filtres enregistrés',
            description: 'Les filtres enregistrés sur les tables'
        }
    }
}

export type DarowMessages = typeof fr
