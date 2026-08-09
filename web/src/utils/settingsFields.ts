import type { FieldBounds, OverrideField, SettingsFieldKey, TableSettings } from '../client/types'

export type { SettingsFieldKey }

export interface SettingsFieldSpec {
    key: SettingsFieldKey
    label: string
    // Plain-language explanation of what the field does and what changing it
    // costs — every field is a server internal by nature (there is no
    // business meaning to attach it to, unlike a table or a source), so the
    // label alone is not enough for someone with no notion of pagination
    // caps or LRU caches to act on it safely.
    description: string
    // Whole numbers only, except the two geometry bounds (fractional
    // tolerances/precisions). The input's granularity, a display choice —
    // unlike min/max, which are the server's own bounds and are read off the
    // payload (FieldBounds), never retyped here.
    step?: number
    // What the value 0 means for this field, when 0 is itself a valid,
    // meaningful input rather than just the bottom of the range — stated
    // here so SettingsView never has to special-case a field by key.
    zeroMeaning?: string
    // Whether a table can carry its own override of this field
    // (TableSettingsModal, back/todo.md TODO-18) — defaultListLimit and
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
                key: 'defaultRowLimit',
                label: 'Nombre de lignes par page dans une table',
                description:
                    'Le nombre de lignes que le serveur renvoie quand une requête ne précise rien — ce que l’écran d’une table affiche à l’ouverture. Une requête qui en demande davantage n’en obtient pas plus.',
                perTable: true
            },
            {
                key: 'defaultListLimit',
                label: 'Nombre d’éléments par page de liste (tables, sources)',
                description:
                    'Même principe que ci-dessus, mais pour les catalogues (liste des tables, liste des sources) plutôt que pour les lignes d’une table.'
            }
        ]
    },
    {
        title: 'Cartographie et tuiles',
        description: 'Ce que renvoient les API cartographiques (OGC, tuiles vectorielles).',
        fields: [
            {
                key: 'defaultFeaturesLimit',
                label: 'Nombre d’objets géographiques par requête',
                description:
                    'Nombre de points, lignes et polygones que le serveur renvoie à une requête cartographique. Une requête qui en demande davantage n’en obtient pas plus.',
                perTable: true,
                geoOnly: true
            },
            {
                key: 'maxSimplifyTolerance',
                label: 'Simplification maximale autorisée des formes',
                step: 0.01,
                description:
                    'Tolérance en mètres : plus la valeur est élevée, plus une forme géographique peut être allégée (moins de détails, réponse plus rapide) quand un client le demande. Plafond appliqué à ce qu’un client peut demander, pas la valeur que le serveur applique lui-même.',
                perTable: true,
                geoOnly: true
            },
            {
                key: 'maxPrecision',
                label: 'Grille de coordonnées la plus fine autorisée',
                step: 0.01,
                description:
                    'Le serveur arrondit les coordonnées renvoyées à une grille de cette taille, en mètres (EPSG:3857). Plus la valeur est basse, plus les coordonnées sont fines ; un client ne peut pas demander plus fin que ce plafond.',
                perTable: true,
                geoOnly: true
            },
            {
                key: 'tileCacheMaxAgeSec',
                label: 'Durée de mise en cache d’une tuile par le navigateur',
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
                description:
                    'Nombre de lignes, colonnes, types… gardés en mémoire pendant cette durée avant d’être relus en base.',
                zeroMeaning:
                    'aucune expiration automatique — une information reste en mémoire jusqu’à être remplacée faute de place.'
            }
        ]
    }
]

// inputRange turns the bounds the server publishes into what an <input>
// accepts: min is inclusive there, so an exclusive minimum becomes the first
// value the step grid actually allows.
export function inputRange(
    bounds: FieldBounds | undefined,
    step?: number
): { min?: number; max?: number } {
    if (!bounds) return {}
    const min =
        bounds.minimum ??
        (bounds.exclusiveMinimum === undefined ? undefined : bounds.exclusiveMinimum + (step ?? 1))
    return { min, max: bounds.maximum }
}

// The per-table fields a TableSettingsModal renders, in the same order/
// grouping the global screen uses — labels and descriptions never retyped
// for the per-table surface.
export const TABLE_SETTINGS_FIELDS: SettingsFieldSpec[] = SETTINGS_FIELD_GROUPS.flatMap(
    (group) => group.fields
).filter((field) => field.perTable)

// tableFieldValue reads one field off a TableSettings response — its
// override, its effective value and its bounds — or null for a field the
// table does not carry: a global-only field (LIM-12) or a geometry field on
// a non-geo table. Excluding the two global-only keys first is what lets the
// lookup type-check without an assertion (§0.9).
export function tableFieldValue(
    settings: TableSettings,
    key: SettingsFieldKey
): OverrideField | null {
    if (key === 'defaultListLimit' || key === 'cacheTTLSec') return null
    return settings[key] ?? null
}

const FIELD_KEYS = new Set<string>(
    SETTINGS_FIELD_GROUPS.flatMap((group) => group.fields.map((field) => field.key))
)

// SettingsService.UpdateSettings joins every violation into one message
// ("defaultRowLimit must be >= 50; defaultListLimit must be <= 5000"),
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
