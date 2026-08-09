<script setup lang="ts">
import { computed, reactive, ref, toRef, watch } from 'vue'
import { useTableSettings } from '../../composables/useTableSettings'
import {
    TABLE_SETTINGS_FIELDS,
    tableFieldValue,
    type SettingsFieldKey
} from '../../utils/settingsFields'
import type { TableSettingsFieldKey, TableSettingsPatch } from '../../client/types'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import SettingsField from '../settings/SettingsField.vue'
import Skeleton from '../ui/Skeleton.vue'
import ErrorState from '../ui/ErrorState.vue'
import Typography from '../ui/Typography.vue'

const props = defineProps<{ tableId: string; isGeo: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { settings, loading, loadedOnce, error, retry, saving, patch, resetAll } = useTableSettings(
    toRef(props, 'tableId')
)

// The row-page field applies to every table; the geometry fields only to a
// geo one — absent from the form entirely rather than shown disabled, per
// §8.9's "a control that can never apply is a question the user has to
// answer to discover it was not one."
const fields = computed(() =>
    props.isGeo ? TABLE_SETTINGS_FIELDS : TABLE_SETTINGS_FIELDS.filter((f) => !f.geoOnly)
)

// Two parallel per-field states: whether the field currently follows the
// global value, and its own input text — kept as a string for the same
// reason SettingsView's draft is (an in-progress edit is never coerced
// mid-keystroke). Re-seeded from `settings` on every load, never on a
// rejected save.
const inherited = reactive<Record<string, boolean>>({})
const draft = reactive<Record<string, string>>({})
const savedInherited = ref<Record<string, boolean>>({})
const savedDraft = ref<Record<string, string>>({})
const localErrors = reactive<Partial<Record<SettingsFieldKey, string>>>({})

watch(
    settings,
    (value) => {
        if (!value) return
        const nextInherited: Record<string, boolean> = {}
        const nextDraft: Record<string, string> = {}
        for (const field of TABLE_SETTINGS_FIELDS) {
            const fieldValue = tableFieldValue(value, field.key as TableSettingsFieldKey)
            if (!fieldValue) continue
            const isInherited = fieldValue.override === null
            nextInherited[field.key] = isInherited
            nextDraft[field.key] = String(isInherited ? fieldValue.effective : fieldValue.override)
        }
        Object.assign(inherited, nextInherited)
        Object.assign(draft, nextDraft)
        savedInherited.value = nextInherited
        savedDraft.value = nextDraft
    },
    { immediate: true }
)

function toggleInherit(key: string) {
    if (!settings.value) return
    const fieldValue = tableFieldValue(settings.value, key as TableSettingsFieldKey)
    if (!fieldValue) return
    inherited[key] = !inherited[key]
    if (inherited[key]) {
        draft[key] = String(fieldValue.effective)
    }
    delete localErrors[key as SettingsFieldKey]
}

const isDirty = computed(() =>
    fields.value.some(
        (field) =>
            inherited[field.key] !== savedInherited.value[field.key] ||
            (!inherited[field.key] && draft[field.key] !== savedDraft.value[field.key])
    )
)

async function onSave() {
    for (const key of Object.keys(localErrors)) delete localErrors[key as SettingsFieldKey]

    const body: TableSettingsPatch = {}
    for (const field of fields.value) {
        const key = field.key as TableSettingsFieldKey
        const wasInherited = savedInherited.value[field.key]
        const nowInherited = inherited[field.key]
        if (
            nowInherited === wasInherited &&
            (nowInherited || draft[field.key] === savedDraft.value[field.key])
        ) {
            continue
        }
        if (nowInherited) {
            body[key] = null
            continue
        }
        const parsed = Number(draft[field.key])
        if (draft[field.key].trim() === '' || !Number.isFinite(parsed)) {
            localErrors[field.key] = 'Doit être un nombre'
            continue
        }
        body[key] = parsed
    }

    if (Object.keys(localErrors).length > 0 || Object.keys(body).length === 0) return
    await patch(body)
}

async function onResetAll() {
    await resetAll()
}
</script>

<template>
    <Modal title="Paramètres de la table" :dirty="isDirty" @close="emit('close')" @submit="onSave">
        <Skeleton v-if="loading && !loadedOnce" :rows="4" />
        <ErrorState v-else-if="error" :message="error" @retry="retry" />

        <div v-else class="flex flex-col gap-5">
            <Typography variant="body-sm" class="text-base-content/60">
                Chaque champ suit la valeur globale par défaut ; personnalisez-le pour cette table
                seulement si nécessaire.
            </Typography>

            <SettingsField
                v-for="field in fields"
                :key="field.key"
                v-model="draft[field.key]"
                :field="field"
                :error="localErrors[field.key]"
                :disabled="saving"
                :inherited="inherited[field.key]"
                show-inherit-toggle
                @toggle-inherit="toggleInherit(field.key)"
            />

            <div class="flex items-center justify-between border-t border-base-300 pt-3">
                <BaseButton
                    type="button"
                    variant="ghost"
                    size="sm"
                    :disabled="saving"
                    @click="onResetAll"
                >
                    Tout réinitialiser
                </BaseButton>
                <div class="flex gap-2">
                    <BaseButton type="button" variant="secondary" @click="emit('close')"
                        >Annuler</BaseButton
                    >
                    <BaseButton
                        type="submit"
                        variant="primary"
                        :busy="saving"
                        :disabled="!isDirty || saving"
                    >
                        {{ saving ? 'Enregistrement…' : 'Enregistrer' }}
                    </BaseButton>
                </div>
            </div>
        </div>
    </Modal>
</template>
