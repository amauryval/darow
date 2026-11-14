<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import Card from '@myapplibs/web-platform/components/base/Card.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSelect from '@myapplibs/web-platform/components/form/FormSelect.vue'
import FormTextarea from '@myapplibs/web-platform/components/form/FormTextarea.vue'
import PageTemplate from '@myapplibs/web-platform/components/layout/PageTemplate.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import { useSql } from '@/darow/composables/useSql'
import { useTables } from '@/darow/composables/useTables'
import { icons } from '@/darow/icons'
import { DarowRoutes } from '@/darow/routeNames'
import { columnIdentifier, sqlIdentifier } from '@/darow/utils/conditions'
import { DUCKDB_DOCS } from '@/darow/utils/duckdbDocs'

const router = useRouter()
const { name, sql, picked, read, tableNames, running, insert, run } = useSql()
const tables = useTables()

onMounted(() => tables.load())

const tableOptions = computed(() => tableNames.value.map((table) => ({ id: table, label: table })))
const ready = computed(() => name.value.trim() !== '' && sql.value.trim() !== '')

const setPicked = (value: string | number | (string | number)[]) => {
    picked.value = Array.isArray(value) ? value.map(String) : []
}

const onRun = async () => {
    const table = name.value.trim()
    if (await run()) await router.push({ name: DarowRoutes.TABLES, params: { name: table } })
}

const breadcrumbs = computed(() => [{ label: t('darow.pages.sql'), icon: icons.Sql }])
</script>

<template>
    <PageTemplate :breadcrumbs="breadcrumbs">
        <Card :title="t('darow.sql.title')">
            <div class="flex flex-col gap-4">
                <FormInput
                    v-model="name"
                    :label="t('darow.sql.name')"
                    :description="t('darow.sources.tableHint')"
                />
                <FormSelect
                    :model-value="picked"
                    multiple
                    :legend="t('darow.sql.tables')"
                    :placeholder="t('darow.sql.chooseTables')"
                    :description="t('darow.sql.insertHint')"
                    :options="tableOptions"
                    @update:model-value="setPicked"
                />
                <div
                    v-for="table in read"
                    :key="table.name"
                    class="flex flex-wrap items-center gap-1"
                >
                    <BaseButton
                        tone="soft"
                        size="xs"
                        class="font-mono"
                        @click="insert(sqlIdentifier(table.name))"
                    >
                        {{ table.name }}
                    </BaseButton>
                    <BaseButton
                        v-for="column in table.columns"
                        :key="column.name"
                        tone="ghost"
                        size="xs"
                        class="font-mono"
                        :title="column.type"
                        @click="insert(columnIdentifier(table.name, column.name))"
                    >
                        {{ column.name }}
                    </BaseButton>
                </div>
                <FormTextarea
                    v-model="sql"
                    :legend="t('darow.sql.query')"
                    :placeholder="t('darow.sql.placeholder')"
                    :rows="10"
                />
                <div class="flex flex-wrap items-center gap-3 text-sm">
                    <span class="text-base-content/60">{{ t('darow.sql.docs') }}</span>
                    <a
                        v-for="doc in DUCKDB_DOCS"
                        :key="doc.key"
                        :href="doc.url"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="link link-primary"
                    >
                        {{ t(`darow.sql.docLinks.${doc.key}`) }}
                    </a>
                </div>
                <div class="flex justify-end">
                    <BaseButton
                        variant="primary"
                        size="sm"
                        :disabled="!ready"
                        :loading="running"
                        @click="onRun"
                    >
                        <Icon :icon-name="icons.Run" />
                        {{ t('darow.sql.run') }}
                    </BaseButton>
                </div>
            </div>
        </Card>
    </PageTemplate>
</template>
