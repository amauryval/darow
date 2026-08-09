import type {
    Settings,
    SettingsReadOnlyKey,
    TableSettings,
    TableSettingsFieldKey
} from '../client/types'

export type SettingsFieldKey = keyof Omit<Settings, SettingsReadOnlyKey>

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
    // The read-only pair the server resolves this limit to, shown under the
    // input so an operator sees what the server applies today — the built-in
    // values while the field is at 0, its own value once set.
    effectiveKeys?: { default: keyof Settings; max: keyof Settings; noun: string }
    // Whether a table can carry its own override of this field
    // (TableSettingsModal, back/todo.md TODO-18) — maxListLimit and
    // cacheTTLSec stay global on purpose (LIM-12) and are the two fields
    // without this flag.
    perTable?: boolean
    // Whether this field only applies to a geo table — absent from a
    // non-geo table's modal rather than shown disabled.
    geoOnly?: boolean
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
                key: 'maxRowLimit',
                label: 'Nombre de lignes par page dans une table',
                description:
                    'La valeur que vous fixez ici sert deux fois : c’est le nombre de lignes que le serveur renvoie quand une requête ne précise rien — ce que l’écran d’une table affiche à l’ouverture — et le plafond qu’il applique à une requête qui en demande davantage.',
                zeroMeaning: 'le serveur applique ses propres valeurs, rappelées ci-dessous.',
                effectiveKeys: {
                    default: 'effectiveDefaultRowLimit',
                    max: 'effectiveMaxRowLimit',
                    noun: 'lignes'
                },
                perTable: true
            },
            {
                key: 'maxListLimit',
                label: 'Nombre d’éléments par page de liste (tables, sources)',
                description:
                    'Même principe que ci-dessus, mais pour les catalogues (liste des tables, liste des sources) plutôt que pour les lignes d’une table.',
                zeroMeaning: 'le serveur applique ses propres valeurs, rappelées ci-dessous.',
                effectiveKeys: {
                    default: 'effectiveDefaultListLimit',
                    max: 'effectiveMaxListLimit',
                    noun: 'éléments'
                }
            }
        ]
    },
    {
        title: 'Cartographie et tuiles',
        description: 'Ce que renvoient les API cartographiques (OGC, tuiles vectorielles).',
        fields: [
            {
                key: 'maxFeaturesLimit',
                label: 'Nombre d’objets géographiques par requête',
                description:
                    'Nombre de points, lignes et polygones que le serveur renvoie à une requête cartographique qui ne précise pas de limite, et plafond qu’il applique à une requête qui en demande davantage.',
                zeroMeaning: 'le serveur applique ses propres valeurs, rappelées ci-dessous.',
                effectiveKeys: {
                    default: 'effectiveDefaultFeaturesLimit',
                    max: 'effectiveMaxFeaturesLimit',
                    noun: 'objets'
                },
                perTable: true,
                geoOnly: true
            },
            {
                key: 'maxSimplifyTolerance',
                label: 'Simplification maximale autorisée des formes',
                unit: 'm',
                step: 0.01,
                description:
                    'Tolérance en mètres : plus la valeur est élevée, plus une forme géographique peut être allégée (moins de détails, réponse plus rapide) quand un client le demande. Plafond appliqué à ce qu’un client peut demander, pas la valeur que le serveur applique lui-même.',
                perTable: true,
                geoOnly: true
            },
            {
                key: 'maxPrecision',
                label: 'Grille de coordonnées la plus fine autorisée',
                unit: 'm',
                step: 0.01,
                description:
                    'Le serveur arrondit les coordonnées renvoyées à une grille de cette taille, en mètres (EPSG:3857). Plus la valeur est basse, plus les coordonnées sont fines ; un client ne peut pas demander plus fin que ce plafond.',
                perTable: true,
                geoOnly: true
            },
            {
                key: 'tileCacheMaxAgeSec',
                label: 'Durée de mise en cache d’une tuile par le navigateur',
                unit: 's',
                description:
                    'Le navigateur réutilise une tuile de carte déjà téléchargée pendant cette durée avant de la redemander au serveur.',
                zeroMeaning:
                    'le navigateur revalide la tuile à chaque affichage, sans jamais la garder en cache.',
                perTable: true,
                geoOnly: true
            },
            {
                key: 'coordinateDecimals',
                label: 'Décimales des coordonnées géographiques renvoyées',
                description:
                    'Nombre de chiffres après la virgule des coordonnées GeoJSON renvoyées (hors tuiles vectorielles, qui ont leur propre grille). Réduire cette valeur allège la réponse sans changer la précision stockée.',
                perTable: true,
                geoOnly: true
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

// The per-table fields a TableSettingsModal renders, in the same order/
// grouping the global screen uses — labels, descriptions and units never
// retyped for the per-table surface.
export const TABLE_SETTINGS_FIELDS: SettingsFieldSpec[] = SETTINGS_FIELD_GROUPS.flatMap(
    (group) => group.fields
).filter((field) => field.perTable)

// tableFieldValue reads one field's override/effective pair off a
// TableSettings response, collapsing LimitField's default/max pair down to
// its default half — the number a per-table modal shows, the same one
// SettingsField's own effective caption shows for the global row.
export function tableFieldValue(
    settings: TableSettings,
    key: TableSettingsFieldKey
): { override: number | null; effective: number } | null {
    if (key === 'maxRowLimit' || key === 'maxFeaturesLimit') {
        const field = settings[key]
        return field ? { override: field.override, effective: field.effectiveDefault } : null
    }
    const value = settings[key]
    return value ? { override: value.override, effective: value.effective } : null
}

const FIELD_KEYS = new Set<string>(
    SETTINGS_FIELD_GROUPS.flatMap((group) => group.fields.map((field) => field.key))
)

// SettingsService.UpdateSettings joins every violation into one message
// ("maxRowLimit must not be negative; maxListLimit must be <= 5000"),
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
