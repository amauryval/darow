<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus } from '@lucide/vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import ListDetailLayout from '@myapplibs/web-platform/components/layout/ListDetailLayout.vue'
import ListRow from '@myapplibs/web-platform/components/layout/ListRow.vue'
import PageTemplate from '@myapplibs/web-platform/components/layout/PageTemplate.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import { useCanEdit } from '@/darow/composables/useCanEdit'
import { useFilterEditor } from '@/darow/composables/useFilterEditor'
import { useSavedFilters } from '@/darow/composables/useSavedFilters'
import { useTables } from '@/darow/composables/useTables'
import { icons } from '@/darow/icons'
import { DarowRoutes } from '@/darow/routeNames'
import FilterForm from './FilterForm.vue'

const router = useRouter()
const { canEdit } = useCanEdit()
const tables = useTables()
const saved = useSavedFilters()
const editor = useFilterEditor()
const { search, order, items, draft, columns, ready } = editor

onMounted(() => Promise.all([tables.load(), saved.load()]))

const openTable = () => {
    if (!draft.value) return
    return router.push({
        name: DarowRoutes.TABLES,
        params: { name: draft.value.table },
        query: { filter: draft.value.id }
    })
}

const breadcrumbs = computed(() => [{ label: t('darow.pages.filters'), icon: icons.Filter }])
</script>

<template>
    <PageTemplate fill-height :breadcrumbs="breadcrumbs">
        <ListDetailLayout
            v-model:query="search"
            v-model:order="order"
            :has-selection="!!draft"
            :search-placeholder="t('darow.filters.search')"
            :empty-detail-message="t('darow.filters.select')"
            @close="editor.close"
        >
            <template v-if="canEdit" #list-actions>
                <button
                    type="button"
                    class="btn btn-sm btn-square btn-success"
                    :title="t('darow.filters.new')"
                    :aria-label="t('darow.filters.new')"
                    @click="editor.create"
                >
                    <Plus class="size-4" />
                </button>
            </template>

            <template #list>
                <ListRow
                    v-for="item in items"
                    :key="item.id"
                    :label="`${item.name} · ${item.table || t('darow.filters.anyTable')}`"
                    :selected="item.id === draft?.id"
                    @select="editor.open(item.id)"
                />
                <p v-if="!items.length" class="px-3 py-2 text-sm text-base-content/70">
                    {{ t('darow.filters.empty') }}
                </p>
            </template>

            <template #detail-header>
                <div v-if="draft" class="flex flex-wrap items-center gap-2 w-full">
                    <h2 class="text-lg font-semibold truncate flex-1">
                        {{ draft.name || t('darow.filters.new') }}
                    </h2>
                    <BaseButton
                        v-if="draft.id && draft.table"
                        tone="ghost"
                        size="sm"
                        :title="t('darow.filters.openTable')"
                        :aria-label="t('darow.filters.openTable')"
                        @click="openTable"
                    >
                        <Icon :icon-name="icons.Tables" />
                    </BaseButton>
                    <BaseButton
                        v-if="canEdit"
                        variant="primary"
                        tone="soft"
                        size="sm"
                        :disabled="!ready"
                        :title="t('darow.filters.save')"
                        :aria-label="t('darow.filters.save')"
                        @click="editor.save"
                    >
                        <Icon :icon-name="icons.Save" />
                    </BaseButton>
                    <BaseButton
                        v-if="canEdit && draft.id"
                        variant="danger"
                        tone="ghost"
                        size="sm"
                        :title="t('darow.filters.delete')"
                        :aria-label="t('darow.filters.delete')"
                        @click="editor.remove"
                    >
                        <Icon :icon-name="icons.Delete" />
                    </BaseButton>
                </div>
            </template>

            <template #detail>
                <FilterForm
                    v-if="draft"
                    v-model="draft"
                    :tables="tables.items.value"
                    :columns="columns"
                    :can-edit="canEdit"
                    @table="editor.setTable"
                />
            </template>
        </ListDetailLayout>
    </PageTemplate>
</template>
