<script setup lang="ts">
import { Pencil } from '@lucide/vue'
import BaseButton from '../ui/BaseButton.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// A grid column's header, name plus its one action — a single button, not a
// menu (§0.2: the modal itself is owned by TableDataPanel, this only reports
// the intent). A one-item MenuDropdown here used to render its panel clipped:
// the grid's own scroll container (DataTable's overflow-auto) cuts off any
// absolutely-positioned content that overflows it, and a lone action needs
// no menu indirection anyway. The button sits inside a `@click.stop` span:
// the `<th>` it renders into also carries DataTable's own sort handler, and
// opening this modal must not toggle it.
defineProps<{ name: string }>()
const emit = defineEmits<{ transform: [] }>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard('data')
</script>

<template>
    <span class="inline-flex items-center gap-1">
        <span>{{ name }}</span>
        <span @click.stop @keydown.stop>
            <BaseButton
                icon-only
                :icon="Pencil"
                size="xs"
                variant="ghost"
                :disabled="writeDisabled"
                :aria-label="`Mettre à jour la colonne ${name}`"
                :title="writeDisabled ? writeGuardReason : `Mettre à jour la colonne ${name}`"
                @click="emit('transform')"
            />
        </span>
    </span>
</template>
