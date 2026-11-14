<script setup lang="ts">
import { ref, watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'

const props = defineProps<{ isOpen: boolean; title: string; label: string; initial: string }>()
const emit = defineEmits<{ submit: [name: string]; close: [] }>()

const name = ref(props.initial)

watch(
    () => props.isOpen,
    (open) => {
        if (open) name.value = props.initial
    }
)
</script>

<template>
    <BaseModal :is-open="isOpen" :title="title" max-width="max-w-md" @close="emit('close')">
        <form class="flex flex-col gap-4" @submit.prevent="emit('submit', name.trim())">
            <FormInput v-model="name" :label="label" />
            <div class="flex justify-end gap-2">
                <BaseButton tone="ghost" size="sm" @click="emit('close')">
                    {{ t('darow.actions.cancel') }}
                </BaseButton>
                <BaseButton type="submit" variant="primary" size="sm" :disabled="!name.trim()">
                    {{ t('darow.actions.confirm') }}
                </BaseButton>
            </div>
        </form>
    </BaseModal>
</template>
