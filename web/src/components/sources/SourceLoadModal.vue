<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { LoadOptions } from '../../client/sources'
import type { TableProvenance } from '../../client/types'
import { useWriteGuard } from '../../composables/useWriteGuard'
import Modal from '../ui/Modal.vue'
import BaseInput from '../ui/BaseInput.vue'
import BaseSelect from '../ui/BaseSelect.vue'
import ModalFormActions from '../ui/ModalFormActions.vue'
import Typography from '../ui/Typography.vue'

// The whole load question in one form: the CRS to project from, the name the
// table takes, and — when this source already has tables — whether this load
// replaces one instead of creating yet another table of the same name.
const props = defineProps<{
    sourceLabel: string
    // The name a table loaded from this source is proposed, already free of
    // the names the catalogue holds.
    suggestedName: string
    existing: TableProvenance[]
    detectedSrid: string
    detecting?: boolean
    // A source with no geometry has no CRS to state, and is never asked for one.
    epsgApplies?: boolean
    loading?: boolean
}>()

const emit = defineEmits<{ close: []; confirm: [options: LoadOptions] }>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

const epsg = ref(props.detectedSrid)
const tableName = ref(props.suggestedName)
const replaceTableId = ref(props.existing[0]?.tableId ?? '')
const mode = ref<'new' | 'replace'>('new')

// The detection lands after the form opens: an untouched field takes the
// answer, a field the user has already corrected keeps their value.
const epsgTouched = ref(false)
watch(
    () => props.detectedSrid,
    (srid) => {
        if (!epsgTouched.value) epsg.value = srid
    }
)
watch(
    () => props.suggestedName,
    (name) => (tableName.value = name)
)

const replaceOptions = computed(() =>
    props.existing.map((entry) => ({ value: entry.tableId, label: entry.tableName }))
)

// The server takes an SRID, a positive integer. A source with no geometry has
// none to state and is confirmed as it is.
const epsgValid = computed(() => /^[1-9]\d*$/.test(epsg.value.trim()))
const replacing = computed(() => mode.value === 'replace' && replaceTableId.value !== '')
const canSubmit = computed(
    () =>
        !props.detecting &&
        !props.loading &&
        (props.epsgApplies === false || epsgValid.value) &&
        (replacing.value || tableName.value.trim() !== '')
)

function onSubmit() {
    if (!canSubmit.value || writeDisabled.value) return
    emit('confirm', {
        srid: epsgValid.value ? Number(epsg.value.trim()) : undefined,
        tableName: replacing.value ? undefined : tableName.value.trim(),
        replaceTableId: replacing.value ? replaceTableId.value : undefined
    })
}
</script>

<template>
    <Modal :title="`Charger ${sourceLabel}`" @close="emit('close')" @submit="onSubmit">
        <div class="flex flex-col gap-4">
            <BaseInput
                v-if="detecting || epsgApplies !== false"
                v-model="epsg"
                inputmode="numeric"
                size="sm"
                label="Code EPSG de la source"
                :placeholder="detecting ? 'Détection…' : 'Code EPSG'"
                :disabled="detecting"
                @update:model-value="epsgTouched = true"
            />
            <Typography v-else variant="caption">
                Cette source ne porte pas de géométrie : aucun système de coordonnées à indiquer.
            </Typography>

            <div v-if="existing.length > 0" class="flex flex-col gap-2">
                <Typography variant="caption">
                    {{ existing.length }} table{{ existing.length > 1 ? 's' : '' }} déjà chargée{{
                        existing.length > 1 ? 's' : ''
                    }}
                    depuis cette source.
                </Typography>
                <label class="flex items-center gap-2 text-sm">
                    <input v-model="mode" type="radio" value="new" class="radio radio-sm" />
                    Créer une nouvelle table
                </label>
                <label class="flex items-center gap-2 text-sm">
                    <input v-model="mode" type="radio" value="replace" class="radio radio-sm" />
                    Remplacer les données d'une table existante
                </label>
            </div>

            <BaseSelect
                v-if="replacing || (mode === 'replace' && existing.length > 0)"
                v-model="replaceTableId"
                size="sm"
                label="Table à recharger"
                :options="replaceOptions"
            />
            <BaseInput
                v-else
                v-model="tableName"
                size="sm"
                label="Nom de la table"
                placeholder="Nom de la table"
            />

            <Typography variant="caption">
                {{
                    replacing
                        ? 'Les données de la table sont remplacées ; son nom, ses paramètres et ses filtres enregistrés sont conservés.'
                        : 'La table est une copie des données : supprimer le fichier ensuite ne la touche pas.'
                }}
            </Typography>
        </div>

        <ModalFormActions
            :confirm-label="replacing ? 'Remplacer' : 'Charger'"
            :busy="loading"
            :disabled="!canSubmit || writeDisabled"
            :reason="writeDisabled ? writeGuardReason : undefined"
            @cancel="emit('close')"
            @confirm="onSubmit"
        />
    </Modal>
</template>
