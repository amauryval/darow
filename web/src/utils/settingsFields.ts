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
    // Whole numbers only, except the two geometry bounds (fractional
    // tolerances/precisions).
    step?: number
    // What the value 0 means for this field, when 0 is itself a valid,
    // meaningful input rather than just the bottom of the range — stated
    // here so SettingsView never has to special-case a field by key.
    zeroMeaning?: string
}

export interface SettingsFieldGroup {
    title: string
    description: string
    fields: SettingsFieldSpec[]
    // Set on a group that tunes a process internal rather than a product
    // decision (IMP-F11/TODO-F40) — SettingsView folds it into a collapsed
    // *Avancé* section instead of showing it at the same weight as
    // Pagination or Cartographie et tuiles.
    advanced?: boolean
}

// The families the server itself groups these constants into
// (internal/settings), used to lay the form out as sections instead of one
// flat list of inputs.
export const SETTINGS_FIELD_GROUPS: SettingsFieldGroup[] = [
    {
        title: 'Pagination',
        description: 'Combien de lignes ou d’éléments le serveur renvoie à la fois.',
        fields: [
            {
                key: 'defaultRowLimit',
                label: 'Lignes chargées par page dans une table',
                description:
                    'Ce que le serveur renvoie quand une requête ne précise pas elle-même de nombre de lignes — c’est ce que l’écran d’une table affiche à l’ouverture.'
            },
            {
                key: 'maxRowLimit',
                label: 'Nombre de lignes maximum par page',
                description:
                    'Plafond que le serveur applique lui-même : une page de résultats ne peut jamais dépasser ce nombre de lignes, même si une requête en demande davantage.'
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
                description: 'Plafond que le serveur applique lui-même, pour ces mêmes catalogues.'
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
                    'Ce que le serveur applique lui-même quand une requête cartographique ne précise pas elle-même de limite (nombre de points, lignes, polygones).'
            },
            {
                key: 'maxFeaturesLimit',
                label: 'Nombre maximum d’objets géographiques par requête',
                description:
                    'Plafond que le serveur applique lui-même, même si une requête en demande davantage.'
            },
            {
                key: 'maxSimplifyTolerance',
                label: 'Simplification maximale autorisée des formes',
                unit: 'm',
                step: 0.01,
                description:
                    'Tolérance en mètres : plus la valeur est élevée, plus une forme géographique peut être allégée (moins de détails, réponse plus rapide) quand un client le demande. Plafond appliqué à ce qu’un client peut demander, pas la valeur que le serveur applique lui-même.'
            },
            {
                key: 'maxPrecision',
                label: 'Grille de coordonnées la plus fine autorisée',
                unit: 'm',
                step: 0.01,
                description:
                    'Le serveur arrondit les coordonnées renvoyées à une grille de cette taille, en mètres (EPSG:3857). Plus la valeur est basse, plus les coordonnées sont fines ; un client ne peut pas demander plus fin que ce plafond.'
            },
            {
                key: 'tileCacheMaxAgeSec',
                label: 'Durée de mise en cache d’une tuile par le navigateur',
                unit: 's',
                description:
                    'Le navigateur réutilise une tuile de carte déjà téléchargée pendant cette durée avant de la redemander au serveur.',
                zeroMeaning:
                    'le navigateur revalide la tuile à chaque affichage, sans jamais la garder en cache.'
            }
        ]
    },
    {
        title: 'Avancé',
        description:
            'Réglage interne du serveur : la taille de ces caches est fixée au démarrage (config.yaml), seule leur durée de vie se règle ici.',
        advanced: true,
        fields: [
            {
                key: 'cacheTTLSec',
                label: 'Informations et structure des tables — durée avant relecture',
                unit: 's',
                description:
                    'Nombre de lignes, colonnes, types… gardés en mémoire pendant cette durée avant d’être relus en base.',
                zeroMeaning:
                    'aucune expiration automatique — une information reste en mémoire jusqu’à être remplacée faute de place.'
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
