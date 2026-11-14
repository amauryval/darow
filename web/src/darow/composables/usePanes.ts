import { computed, ref, type Ref } from 'vue'

// The map and the table of the open table, each shown or hidden, never both hidden.
export function usePanes(hasGeometry: Ref<boolean>) {
    const map = ref(false)
    const table = ref(true)
    const showMap = computed(() => hasGeometry.value && map.value)
    const showTable = computed(() => !showMap.value || table.value)
    const split = computed(() => showMap.value && showTable.value)

    const toggleMap = () => {
        map.value = !showMap.value
        if (!map.value) table.value = true
    }

    const toggleTable = () => {
        if (showMap.value) {
            table.value = !table.value
        } else if (hasGeometry.value) {
            map.value = true
            table.value = false
        }
    }

    return { showMap, showTable, split, toggleMap, toggleTable }
}
