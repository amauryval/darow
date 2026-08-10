import { computed, ref, watch, type Ref } from 'vue'
import { previewColumnTransform, transformColumn } from '../client/tables'
import { errorMessage } from '../client/http'
import type { ColumnSchema, TransformSample, TransformSpec } from '../client/types'
import { coerceByType, columnType } from '../utils/columnTypes'
import {
    createAssignmentDraft,
    isAssignmentComplete,
    toAssignment,
    type AssignmentDraft
} from '../utils/transformAssignment'
import { nextNodeId, pruneActive, type DraftNode } from '../utils/filterTree'

interface TransformCaseDraft {
    id: number
    conditions: DraftNode[]
    conditionsBool: 'AND' | 'OR'
    assignment: AssignmentDraft
}

function createCaseDraft(): TransformCaseDraft {
    return {
        id: nextNodeId(),
        conditions: [],
        conditionsBool: 'AND',
        assignment: createAssignmentDraft()
    }
}

// The draft behind TransformColumnModal: cases, the default assignment and
// the rows the whole transform is restricted to, plus the preview that
// stands in for an undo (spec.md §8.3, §5.11) and the write itself. Column
// values are coerced against targetType, the same rule useDeriveTable
// applies through coerceByType.
export function useColumnTransform(
    tableId: Ref<string>,
    column: Ref<string>,
    targetType: Ref<string>,
    columns: Ref<ColumnSchema[]>
) {
    const cases = ref<TransformCaseDraft[]>([])
    const defaultAssignment = ref<AssignmentDraft>(createAssignmentDraft())
    const conditions = ref<DraftNode[]>([])
    const conditionsBool = ref<'AND' | 'OR'>('AND')

    const coerce = (col: string, value: string) =>
        coerceByType(columnType(columns.value, col), value)

    const spec = computed<TransformSpec>(() => ({
        cases: cases.value.map((c) => ({
            conditions: pruneActive(c.conditions, coerce),
            assignment: toAssignment(c.assignment, targetType.value)
        })),
        default: toAssignment(defaultAssignment.value, targetType.value),
        conditions: pruneActive(conditions.value, coerce)
    }))

    // A case with no condition is what the server's ValidationError names
    // ("case N has no conditions — use the default assignment for the
    // rest"); blocking it here points at the case rather than surfacing a
    // 400 after a round trip.
    const incomplete = computed(
        () =>
            spec.value.cases.some((c) => c.conditions.length === 0) ||
            cases.value.some((c) => !isAssignmentComplete(c.assignment)) ||
            !isAssignmentComplete(defaultAssignment.value)
    )

    function addCase() {
        cases.value = [...cases.value, createCaseDraft()]
    }

    function removeCase(id: number) {
        cases.value = cases.value.filter((c) => c.id !== id)
    }

    function updateCaseConditions(id: number, nodes: DraftNode[]) {
        cases.value = cases.value.map((c) => (c.id === id ? { ...c, conditions: nodes } : c))
    }

    function updateCaseConditionsBool(id: number, bool: 'AND' | 'OR') {
        cases.value = cases.value.map((c) => (c.id === id ? { ...c, conditionsBool: bool } : c))
    }

    function updateCaseAssignment(id: number, draft: AssignmentDraft) {
        cases.value = cases.value.map((c) => (c.id === id ? { ...c, assignment: draft } : c))
    }

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
        cases,
        defaultAssignment,
        conditions,
        conditionsBool,
        incomplete,
        addCase,
        removeCase,
        updateCaseConditions,
        updateCaseConditionsBool,
        updateCaseAssignment,
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
