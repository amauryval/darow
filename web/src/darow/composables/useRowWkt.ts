import { ref, type Ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import * as api from '@/darow/api/darow'
import { useCopy } from '@myapplibs/web-platform/components/composables/useCopy'
import { sridOf } from '@/darow/utils/crs'

// useRowWkt reads a row's geometry as WKT in the EPSG code typed, and copies it.
export function useRowWkt(table: Ref<string>) {
    const { attempt } = useAttempt()
    const copyText = useCopy()
    const srid = ref('')
    const wkt = ref<string | null>(null)
    const busy = ref(false)

    const load = (id: number) =>
        attempt(
            t('darow.wkt.loadFailed'),
            async () => {
                wkt.value = (await api.getRowWkt(table.value, id, sridOf(srid.value))).wkt ?? null
            },
            busy
        )

    const copy = () => copyText(wkt.value ?? '', t('darow.wkt.copied'), t('darow.wkt.copyFailed'))

    return { srid, wkt, busy, load, copy }
}
