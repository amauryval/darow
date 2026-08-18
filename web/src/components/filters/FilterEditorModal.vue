<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ColumnSchema } from '../../client/types'
import { describeDraft, draftOf, isComplete, nodeOf, type DraftNode } from '../../utils/filterTree'
import Modal from '../ui/Modal.vue'
import FilterNodeEditor from './FilterNodeEditor.vue'
import BaseInput from '../ui/BaseInput.vue'
import FormError from '../ui/FormError.vue'
import ModalFormActions from '../ui/ModalFormActions.vue'
import Typography from '../ui/Typography.vue'

// node null = creating a filter; otherwise the root node being edited.
// savedName is set when the edited filter already exists in the catalogue, so
// the name field opens on its stored name rather than a fresh suggestion.
const props = defineProps<{
    tableId: string
    columns: ColumnSchema[]
    node?: DraftNode | null
    savedName?: string
    saving?: boolean
    error?: string
}>()
const emit = defineEmits<{
    close: []
    submit: [payload: { node: DraftNode; name: string }]
}>()

const initial = draftOf(props.node ?? null, props.columns[0]?.name ?? '')
const bool = ref<'AND' | 'OR'>(initial.bool)
const children = ref<DraftNode[]>(initial.children)
// Every filter is persisted — no draft state, hence no opt-out. The name
// fills itself from the conditions to keep saving one click; nameTouched stops
// that suggestion from overwriting a name the user typed.
const name = ref(props.savedName ?? '')
const nameTouched = ref(props.savedName !== undefined)

const complete = computed(() => children.value.length > 0 && children.value.every(isComplete))

// Only a complete predicate gets described: half-typed conditions would
// suggest a name like `height >  et ()`, which reads as a bug.
const suggestedName = computed(() =>
    complete.value ? describeDraft(bool.value, children.value) : ''
)
const effectiveName = computed(() => (nameTouched.value ? name.value : suggestedName.value))

function onNameInput(value: string) {
    nameTouched.value = true
    name.value = value
}

const nameMissing = computed(() => nameTouched.value && effectiveName.value.trim() === '')
const canSubmit = computed(
    () => complete.value && effectiveName.value.trim() !== '' && !props.saving
)

// Any edit already made is worth a confirmation before a stray backdrop click
// throws it away (§8.3).
const dirty = computed(() => complete.value || nameTouched.value)

function onSubmit() {
    if (!canSubmit.value) return
    const node = nodeOf(bool.value, children.value, true)
    if (node === null) return
    emit('submit', { node, name: effectiveName.value.trim() })
}
</script>

<template>
    <Modal
        :title="props.node ? 'Modifier le filtre' : 'Créer un filtre'"
        wide
        overflow-visible
        :dirty="dirty"
        @close="emit('close')"
        @submit="onSubmit"
    >
        <div class="flex flex-col gap-5">
            <FilterNodeEditor
                :table-id="tableId"
                :columns="columns"
                :nodes="children"
                :bool="bool"
                @update:nodes="children = $event"
                @update:bool="bool = $event"
            />

            <div class="border-t border-base-300 pt-4">
                <BaseInput
                    :model-value="effectiveName"
                    size="sm"
                    label="Nom du filtre"
                    placeholder="Complétez les conditions pour nommer le filtre"
                    :error="nameMissing ? 'Donnez un nom au filtre.' : undefined"
                    @update:model-value="onNameInput"
                />
                <Typography variant="caption" class="mt-1 block">
                    Tout filtre est enregistré et réutilisable depuis « Enregistrés ».
                </Typography>
            </div>

            <FormError :message="props.error" />
        </div>

        <ModalFormActions
            :confirm-label="props.saving ? 'Enregistrement…' : 'Appliquer'"
            :busy="props.saving"
            :disabled="!canSubmit"
            :reason="
                complete ? undefined : 'Complétez chaque condition avant d’appliquer le filtre.'
            "
            @cancel="emit('close')"
            @confirm="onSubmit"
        />
    </Modal>
</template>
