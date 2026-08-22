import { computed, ref, watch, type Ref } from 'vue'
import { previewColumnTransform, transformColumn } from '../client/tables'
import type { ColumnSchema, TransformSample, TransformSpec } from '../client/types'
import { useAsyncAction } from './useAsyncAction'
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

    const previewMatched = ref<number | null>(null)
    const previewSample = ref<TransformSample[]>([])
    const { isLoading: previewing, error: previewError, run: runAsPreview } = useAsyncAction()

    // The preview only ever answers for the spec it was run against — any
    // further edit makes it stale, and a stale preview must not be what the
    // confirmation step counts on.
    watch(spec, () => {
        previewMatched.value = null
        previewSample.value = []
    })

    async function runPreview() {
        const result = await runAsPreview(
            () => previewColumnTransform(tableId.value, column.value, spec.value),
            'Impossible de prévisualiser la mise à jour'
        )
        if (result === null) return
        previewMatched.value = result.matched
        previewSample.value = result.sample
    }

    const { isLoading, error, run } = useAsyncAction()

    function submit() {
        return run(
            () => transformColumn(tableId.value, column.value, spec.value),
            'Impossible de mettre à jour la colonne'
        )
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
