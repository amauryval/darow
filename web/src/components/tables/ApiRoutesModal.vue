<script setup lang="ts">
import { computed } from 'vue'
import { Copy } from '@lucide/vue'
import { buildTableApiRoutes } from '../../utils/apiRoutes'
import { useToast } from '../../composables/useToast'
import Modal from '../ui/Modal.vue'
import Badge from '../ui/Badge.vue'
import BaseButton from '../ui/BaseButton.vue'
import Typography from '../ui/Typography.vue'

const props = defineProps<{ tableId: string; isGeo: boolean }>()
const emit = defineEmits<{ close: [] }>()

const toast = useToast()

const groups = computed(() => {
    const routes = buildTableApiRoutes(props.tableId, props.isGeo)
    const order: string[] = []
    const byGroup = new Map<string, typeof routes>()
    for (const route of routes) {
        if (!byGroup.has(route.group)) {
            order.push(route.group)
            byGroup.set(route.group, [])
        }
        byGroup.get(route.group)?.push(route)
    }
    return order.map((group) => ({ group, routes: byGroup.get(group) ?? [] }))
})

async function copy(url: string) {
    try {
        await navigator.clipboard.writeText(url)
        toast.success('URL copiée.')
    } catch {
        toast.error("Impossible de copier l'URL.")
    }
}
</script>

<template>
    <Modal title="Routes API de la table" xl @close="emit('close')">
        <div class="flex max-h-96 flex-col gap-4 overflow-y-auto pr-1">
            <div v-for="g in groups" :key="g.group" class="flex flex-col gap-1.5">
                <Typography variant="eyebrow">{{ g.group }}</Typography>
                <div
                    v-for="route in g.routes"
                    :key="route.url"
                    class="flex items-center gap-2 rounded-md border border-base-300 px-3 py-2"
                >
                    <Badge tone="primary">{{ route.method }}</Badge>
                    <Typography variant="code" class="min-w-0 flex-1 truncate">{{
                        route.url
                    }}</Typography>
                    <BaseButton
                        variant="ghost"
                        size="xs"
                        :icon="Copy"
                        icon-only
                        title="Copier l'URL"
                        @click="copy(route.url)"
                    >
                        Copier l'URL
                    </BaseButton>
                </div>
            </div>
        </div>
    </Modal>
</template>
