import { computed, ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import * as api from '@/darow/api/darow'
import { useTablesStore } from '@/darow/stores/tables'
import { insertToken } from '@/darow/utils/conditions'

const name = ref('')
const sql = ref('')
const picked = ref<string[]>([])
const running = ref(false)

export function useSql() {
    const tables = useTablesStore()
    const { attempt } = useAttempt()

    const read = computed(() => tables.items.filter((table) => picked.value.includes(table.name)))

    const insert = (token: string) => {
        sql.value = insertToken(sql.value, token)
    }

    const run = () =>
        attempt(
            t('darow.sql.failed'),
            async () => {
                await api.deriveTable(name.value.trim(), sql.value)
                await tables.fetch()
                return t('darow.sql.created', { name: name.value })
            },
            running
        )

    return {
        name,
        sql,
        picked,
        read,
        tableNames: computed(() => tables.items.map((table) => table.name)),
        running,
        insert,
        run
    }
}
