import { computed, ref, watch, type Ref } from 'vue'
import { previewColumnTransform, transformColumn } from '../client/tables'
import { errorMessage } from '../client/http'
import type { ColumnSchema, TransformSample, TransformSpec } from '../client/types'
import { coerceByType } from '../utils/columnTypes'
import { pruneActive, type DraftNode } from '../utils/filterTree'

// The draft behind TransformColumnModal: a free SQL expression computed for
// every row conditions names, plus the preview that stands in for an undo
// (spec.md §8.3, §5.11) and the write itself. There is no per-row branching
// (no CASE WHEN, no arm picker) — a column update is one value, expressed
// once; a caller who needs different values on different rows runs the
// transform again with a narrower filter. Column values in conditions are
// coerced against their own type, the same rule useDeriveTable applies
// through coerceByType.
export function useColumnTransform(
    tableId: Ref<string>,
    column: Ref<string>,
    columns: Ref<ColumnSchema[]>
) {
    const expression = ref('')
    const conditions = ref<DraftNode[]>([])
    const conditionsBool = ref<'AND' | 'OR'>('AND')

    const coerce = (col: string, value: string) =>
        coerceByType(columns.value.find((c) => c.name === col)?.type ?? '', value)

    const spec = computed<TransformSpec>(() => ({
        cases: [],
        default: { kind: 'expression', expression: expression.value.trim() },
        conditions: pruneActive(conditions.value, coerce)
    }))

    const incomplete = computed(() => expression.value.trim() === '')

    const previewing = ref(false)
    const previewError = ref('')
    const previewMatched = ref<number | null>(null)
    const previewSample = ref<TransformSample[]>([])

    // The preview only ever answers for the spec it was run against — any
    // further edit makes it stale, and a stale preview must not be what the
    // confirmation step counts on.
    watch(spec, () => {
        previewMatched.value = null
        previewSample.value = []
    })

    async function runPreview() {
        previewing.value = true
        previewError.value = ''
        try {
            const result = await previewColumnTransform(tableId.value, column.value, spec.value)
            previewMatched.value = result.matched
            previewSample.value = result.sample
        } catch (e) {
            previewError.value = errorMessage(e, 'Impossible de prévisualiser la mise à jour')
        } finally {
            previewing.value = false
        }
    }

    const isLoading = ref(false)
    const error = ref('')

    async function submit(): Promise<{ written: number } | null> {
        isLoading.value = true
        error.value = ''
        try {
            return await transformColumn(tableId.value, column.value, spec.value)
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de mettre à jour la colonne')
            return null
        } finally {
            isLoading.value = false
        }
    }

    return {
        expression,
        conditions,
        conditionsBool,
        incomplete,
        previewing,
        previewError,
        previewMatched,
        previewSample,
        runPreview,
        isLoading,
        error,
        submit
    }
}
