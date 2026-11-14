import { ref, shallowRef, type Ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { useConfirm } from '@myapplibs/web-platform/components/composables/useConfirm'
import type { TransformPreview } from '@/darow/api/engine.gen'
import * as api from '@/darow/api/darow'

const busy = ref(false)
const preview = shallowRef<TransformPreview | null>(null)

export function useColumns(table: Ref<string>, onChange: () => Promise<unknown>) {
    const { attempt } = useAttempt()
    const { confirmDialog } = useConfirm()

    const changing = (failure: string, action: () => Promise<unknown>, success: string) =>
        attempt(
            failure,
            async () => {
                await action()
                await onChange()
                return success
            },
            busy
        )

    const add = (column: string, type: string) =>
        changing(
            t('darow.columns.addFailed'),
            () => api.addColumn(table.value, column, type),
            t('darow.columns.added', { name: column })
        )

    const rename = (column: string, to: string) =>
        changing(
            t('darow.columns.renameFailed'),
            () => api.renameColumn(table.value, column, to),
            t('darow.columns.renamed')
        )

    const drop = async (column: string): Promise<boolean> => {
        if (!(await confirmDialog(t('darow.columns.confirmDelete', { name: column })))) return false
        return changing(
            t('darow.columns.deleteFailed'),
            () => api.dropColumn(table.value, column),
            t('darow.columns.deleted')
        )
    }

    const previewTransform = (column: string, expression: string, where: string) =>
        attempt(
            t('darow.columns.previewFailed'),
            async () => {
                preview.value = await api.previewTransform(table.value, column, expression, where)
            },
            busy
        )

    const transform = async (column: string, expression: string, where: string) => {
        let written = 0
        const done = await changing(
            t('darow.columns.transformFailed'),
            async () => {
                written = (await api.transformColumn(table.value, column, expression, where))
                    .written
                preview.value = null
            },
            t('darow.columns.transformed')
        )
        return done ? written : null
    }

    return {
        busy,
        preview,
        add,
        rename,
        drop,
        previewTransform,
        transform,
        clearPreview: () => (preview.value = null)
    }
}
