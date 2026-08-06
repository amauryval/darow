import type { Settings } from '../client/types'

export type SettingsFieldKey = keyof Omit<Settings, 'updatedAt'>

export interface SettingsFieldSpec {
    key: SettingsFieldKey
    label: string
    // Plain-language explanation of what the field does and what changing it
    // costs — every field is a server internal by nature (there is no
    // business meaning to attach it to, unlike a table or a source), so the
    // label alone is not enough for someone with no notion of pagination
    // caps or LRU caches to act on it safely.
    description: string
    unit?: string
    // Whole numbers only (every field but the two geometry bounds, which are
    // fractional tolerances/precisions).
    step?: number
}

export interface SettingsFieldGroup {
    title: string
    description: string
    fields: SettingsFieldSpec[]
}

// The families the server itself groups these constants into
// (internal/settings), used to lay the form out as sections instead of one
// flat list of eighteen inputs.
export const SETTINGS_FIELD_GROUPS: SettingsFieldGroup[] = [
    {
        title: 'Pagination',
        description: 'Combien de lignes ou d’éléments le serveur renvoie à la fois.',
        fields: [
            {
                key: 'defaultRowLimit',
                label: 'Lignes chargées par page dans une table',
                description:
                    'Quand on ouvre une table, ce nombre de lignes est affiché sans que personne n’ait à le demander.'
            },
            {
                key: 'maxRowLimit',
                label: 'Nombre de lignes maximum par page',
                description:
                    'Plafond absolu : une page de résultats ne peut jamais dépasser ce nombre de lignes, même si on le demande explicitement.'
            },
            {
                key: 'defaultListLimit',
                label: 'Éléments chargés par page dans une liste (tables, sources)',
                description:
                    'Même principe que ci-dessus, mais pour les catalogues (liste des tables, liste des sources) plutôt que pour les lignes d’une table.'
            },
            {
                key: 'maxListLimit',
                label: 'Nombre d’éléments maximum par page de liste',
                description: 'Plafond absolu pour ces mêmes catalogues.'
            }
        ]
    },
    {
        title: 'Requêtes réseau',
        description: 'Limites appliquées à chaque requête envoyée au serveur.',
        fields: [
            {
                key: 'maxBodyBytes',
                label: 'Taille maximale d’un envoi de données',
                unit: 'octets',
                description:
                    'Au-delà de cette taille, un envoi (import de fichier, ajout de lignes) est refusé. 1 000 000 octets ≈ 1 Mo.'
            },
            {
                key: 'requestTimeoutSec',
                label: 'Délai maximum avant abandon d’une requête',
                unit: 's',
                description:
                    'Si le serveur met plus longtemps que ça à répondre, la requête est annulée plutôt que de rester bloquée.'
            }
        ]
    },
    {
        title: 'Export',
        description: 'Comportement d’un export CSV ou GeoJSON.',
        fields: [
            {
                key: 'exportBatchSize',
                label: 'Lignes écrites à la fois pendant un export',
                description:
                    'Un export volumineux est découpé en lots de cette taille pour ne pas surcharger la mémoire du serveur pendant l’opération. Sans effet sur le résultat final, seulement sur la façon dont il est produit.'
            }
        ]
    },
    {
        title: 'Cartographie et tuiles',
        description: 'Ce que renvoient les API cartographiques (OGC, tuiles vectorielles).',
        fields: [
            {
                key: 'defaultFeaturesLimit',
                label: 'Objets géographiques renvoyés par défaut',
                description:
                    'Nombre d’objets (points, lignes, polygones) renvoyés par une requête cartographique qui ne précise pas elle-même de limite.'
            },
            {
                key: 'maxFeaturesLimit',
                label: 'Nombre maximum d’objets géographiques par requête',
                description: 'Plafond absolu, même si la requête en demande davantage.'
            },
            {
                key: 'maxSimplifyTolerance',
                label: 'Simplification maximale autorisée des formes',
                step: 0.01,
                description:
                    'Plus la valeur est élevée, plus une forme géographique peut être allégée (moins de détails, réponse plus rapide) quand un client le demande.'
            },
            {
                key: 'maxPrecision',
                label: 'Nombre de décimales maximum des coordonnées',
                step: 0.01,
                description:
                    'Plafond de précision qu’un client peut demander pour les coordonnées renvoyées.'
            },
            {
                key: 'tileCacheMaxAgeSec',
                label: 'Durée de mise en cache d’une tuile par le navigateur',
                unit: 's',
                description:
                    'Le navigateur réutilise une tuile de carte déjà téléchargée pendant cette durée avant de la redemander au serveur.'
            }
        ]
    },
    {
        title: 'Mémoire du serveur (caches)',
        description:
            'Le serveur garde certaines informations en mémoire pour répondre plus vite. Une valeur plus haute consomme plus de mémoire ; une valeur plus basse fait relire la base plus souvent.',
        fields: [
            {
                key: 'metadataTTLSec',
                label: 'Informations sur les tables — durée avant relecture',
                unit: 's',
                description:
                    'Nombre de lignes, colonnes, date de mise à jour… gardés en mémoire pendant cette durée avant d’être relus en base.'
            },
            {
                key: 'metadataMaxItems',
                label: 'Informations sur les tables — nombre maximum gardé en mémoire',
                description:
                    'Au-delà, les informations des tables les moins consultées récemment sont oubliées et relues en base au prochain accès.'
            },
            {
                key: 'schemaTTLSec',
                label: 'Structure des tables (colonnes) — durée avant relecture',
                unit: 's',
                description: 'Même principe, pour la liste des colonnes et leurs types.'
            },
            {
                key: 'schemaMaxItems',
                label: 'Structure des tables — nombre maximum gardé en mémoire',
                description: 'Même principe que ci-dessus, pour la structure des tables.'
            },
            {
                key: 'tileTTLSec',
                label: 'Tuiles de carte — durée avant régénération',
                unit: 's',
                description:
                    'Une tuile déjà générée est réutilisée pendant cette durée côté serveur.'
            },
            {
                key: 'tileMaxItems',
                label: 'Tuiles de carte — nombre maximum gardé en mémoire',
                description: 'Même principe que ci-dessus, pour les tuiles déjà générées.'
            }
        ]
    }
]

const FIELD_KEYS = new Set<string>(
    SETTINGS_FIELD_GROUPS.flatMap((group) => group.fields.map((field) => field.key))
)

// SettingsService.UpdateSettings joins every violation into one message
// ("defaultRowLimit must be positive; maxRowLimit must be >= defaultRowLimit"),
// each segment starting with the JSON field name it names. Parsed here into
// a per-field map so the form can attach each refusal to its own input
// instead of showing one undifferentiated error.
export function parseSettingsFieldErrors(
    message: string
): Partial<Record<SettingsFieldKey, string>> {
    const errors: Partial<Record<SettingsFieldKey, string>> = {}
    for (const segment of message.split(';')) {
        const trimmed = segment.trim()
        const field = trimmed.split(' ', 1)[0]
        if (field && FIELD_KEYS.has(field)) {
            errors[field as SettingsFieldKey] = trimmed
        }
    }
    return errors
}
