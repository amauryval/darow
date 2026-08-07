<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useSettings } from '../composables/useSettings'
import { useDocumentTitle } from '../composables/useDocumentTitle'
import { SETTINGS_FIELD_GROUPS, type SettingsFieldKey } from '../utils/settingsFields'
import type { Settings, SettingsPatch } from '../client/types'
import Breadcrumbs from '../components/ui/Breadcrumbs.vue'
import Typography from '../components/ui/Typography.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import Skeleton from '../components/ui/Skeleton.vue'
import ErrorState from '../components/ui/ErrorState.vue'

useDocumentTitle(computed(() => 'Paramètres'))

const { current, loading, loadedOnce, error, retry, saving, fieldErrors, save } = useSettings()

// Every field's own input value, kept as a string so an in-progress edit is
// never coerced to a number mid-keystroke. Re-seeded from `current` on
// load and after every successful save — never on a rejection, which leaves
// the operator's own input on screen next to the refusal.
const draft = reactive<Record<SettingsFieldKey, string>>({} as Record<SettingsFieldKey, string>)
const savedValues = ref<Settings | null>(null)

const allFields = SETTINGS_FIELD_GROUPS.flatMap((group) => group.fields)

watch(
    current,
    (value) => {
        if (!value) return
        for (const field of allFields) draft[field.key] = String(value[field.key])
        savedValues.value = value
    },
    { immediate: true }
)

const dirtyKeys = computed<SettingsFieldKey[]>(() => {
    const saved = savedValues.value
    if (!saved) return []
    return allFields.map((field) => field.key).filter((key) => draft[key] !== String(saved[key]))
})
const isDirty = computed(() => dirtyKeys.value.length > 0)

// A field the server refused stays flagged until the operator either edits
// it again (cleared here so a stale refusal doesn't linger under a value
// that was never resubmitted) or resubmits it.
const localErrors = ref<Partial<Record<SettingsFieldKey, string>>>({})
watch(draft, () => {
    for (const key of Object.keys(localErrors.value) as SettingsFieldKey[]) {
        if (savedValues.value && draft[key] !== String(savedValues.value[key])) {
            delete localErrors.value[key]
        }
    }
})

const errors = computed(() => ({ ...fieldErrors.value, ...localErrors.value }))

async function onSave() {
    localErrors.value = {}
    const patch: SettingsPatch = {}
    for (const key of dirtyKeys.value) {
        const parsed = Number(draft[key])
        if (draft[key].trim() === '' || !Number.isFinite(parsed)) {
            localErrors.value[key] = 'Doit être un nombre'
            continue
        }
        patch[key] = parsed
    }
    if (Object.keys(localErrors.value).length > 0 || Object.keys(patch).length === 0) return
    await save(patch)
}

function onReset() {
    if (!savedValues.value) return
    for (const field of allFields) draft[field.key] = String(savedValues.value[field.key])
    localErrors.value = {}
}

// Groups that tune a process internal rather than a product decision
// (IMP-F11/TODO-F40) render under a collapsed <details>, so opening the
// screen to raise a page size doesn't first walk through a cache TTL.
const standardGroups = SETTINGS_FIELD_GROUPS.filter((group) => !group.advanced)
const advancedGroups = SETTINGS_FIELD_GROUPS.filter((group) => group.advanced)
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
        <Breadcrumbs :items="[{ label: 'Paramètres' }]" class="shrink-0" />

        <Skeleton v-if="loading && !loadedOnce" :rows="6" />
        <ErrorState v-else-if="error" :message="error" @retry="retry" />

        <form v-else class="flex min-h-0 flex-1 flex-col gap-4" @submit.prevent="onSave">
            <div class="min-h-0 flex-1 overflow-y-auto">
                <div class="flex flex-col gap-6 pb-2">
                    <section
                        v-for="group in standardGroups"
                        :key="group.title"
                        class="rounded-lg border border-base-300 bg-base-100 p-4"
                    >
                        <Typography variant="h4">{{ group.title }}</Typography>
                        <Typography variant="body-sm" class="mt-1 text-base-content/60">{{
                            group.description
                        }}</Typography>
                        <div class="mt-4 grid grid-cols-1 gap-5 lg:grid-cols-2">
                            <div v-for="field in group.fields" :key="field.key">
                                <BaseInput
                                    v-model="draft[field.key]"
                                    type="number"
                                    :inputmode="field.step ? 'decimal' : 'numeric'"
                                    :step="field.step"
                                    :label="
                                        field.unit ? `${field.label} (${field.unit})` : field.label
                                    "
                                    :error="errors[field.key]"
                                    :disabled="saving"
                                />
                                <Typography variant="caption" class="mt-1 block">{{
                                    field.description
                                }}</Typography>
                                <Typography
                                    v-if="field.zeroMeaning && Number(draft[field.key]) === 0"
                                    variant="caption"
                                    class="mt-1 block text-base-content/60"
                                >
                                    0 = {{ field.zeroMeaning }}
                                </Typography>
                            </div>
                        </div>
                    </section>

                    <details
                        v-for="group in advancedGroups"
                        :key="group.title"
                        class="rounded-lg border border-base-300 bg-base-100 p-4"
                    >
                        <summary class="cursor-pointer">
                            <Typography variant="h4" class="inline">{{ group.title }}</Typography>
                        </summary>
                        <Typography variant="body-sm" class="mt-1 text-base-content/60">{{
                            group.description
                        }}</Typography>
                        <div class="mt-4 grid grid-cols-1 gap-5 lg:grid-cols-2">
                            <div v-for="field in group.fields" :key="field.key">
                                <BaseInput
                                    v-model="draft[field.key]"
                                    type="number"
                                    :inputmode="field.step ? 'decimal' : 'numeric'"
                                    :step="field.step"
                                    :label="
                                        field.unit ? `${field.label} (${field.unit})` : field.label
                                    "
                                    :error="errors[field.key]"
                                    :disabled="saving"
                                />
                                <Typography variant="caption" class="mt-1 block">{{
                                    field.description
                                }}</Typography>
                                <Typography
                                    v-if="field.zeroMeaning && Number(draft[field.key]) === 0"
                                    variant="caption"
                                    class="mt-1 block text-base-content/60"
                                >
                                    0 = {{ field.zeroMeaning }}
                                </Typography>
                            </div>
                        </div>
                    </details>
                </div>
            </div>

            <div class="flex shrink-0 items-center justify-end gap-2 border-t border-base-300 pt-3">
                <BaseButton
                    type="button"
                    variant="secondary"
                    :disabled="!isDirty || saving"
                    @click="onReset"
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
        </form>
    </div>
</template>
