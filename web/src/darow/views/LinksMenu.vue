<script setup lang="ts">
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BasePopover from '@myapplibs/web-platform/components/base/BasePopover.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import { useCopy } from '@myapplibs/web-platform/components/composables/useCopy'
import { icons } from '@/darow/icons'
import type { TableLink } from '@/darow/utils/tableLinks'

defineProps<{ links: TableLink[] }>()
const copy = useCopy()
const onCopy = (link: TableLink) =>
    copy(link.url, t('darow.links.copied'), t('darow.links.copyFailed'))
</script>

<template>
    <BasePopover width="w-96" align="end">
        <template #trigger="{ toggle }">
            <BaseButton
                tone="ghost"
                size="sm"
                class="btn-square"
                :title="t('darow.links.title')"
                :aria-label="t('darow.links.title')"
                @click="toggle"
            >
                <Icon :icon-name="icons.Link" />
            </BaseButton>
        </template>
        <ul class="flex flex-col p-2 gap-1">
            <li v-for="link in links" :key="link.id">
                <button
                    type="button"
                    class="w-full flex items-start gap-2 rounded-field p-2 text-left hover:bg-base-200"
                    :title="t('darow.links.copy')"
                    @click="onCopy(link)"
                >
                    <Icon :icon-name="icons.Copy" class="mt-0.5 shrink-0" />
                    <span class="flex flex-col gap-0.5 min-w-0">
                        <span class="font-medium">{{ t(`darow.links.${link.id}.label`) }}</span>
                        <span class="text-xs text-base-content/70">
                            {{ t(`darow.links.${link.id}.hint`) }}
                            <template v-if="link.filtered"
                                >· {{ t('darow.links.filtered') }}</template
                            >
                        </span>
                        <span class="text-xs font-mono break-all select-all">{{ link.url }}</span>
                    </span>
                </button>
            </li>
        </ul>
    </BasePopover>
</template>
