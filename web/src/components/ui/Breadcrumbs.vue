<script setup lang="ts">
import { RouterLink } from 'vue-router'

defineProps<{ items: { label: string; to?: string }[] }>()
</script>

<template>
    <div class="breadcrumbs text-sm">
        <ul>
            <li v-for="(item, index) in items" :key="item.label">
                <RouterLink v-if="item.to" :to="item.to">{{ item.label }}</RouterLink>
                <!-- The trail's last, link-less entry is the page's only heading —
             no view repeats the object's name in a separate <h1>, since that
             duplicated it right below the breadcrumb for no benefit. -->
                <h1 v-else-if="index === items.length - 1" class="inline text-sm font-normal">
                    {{ item.label }}
                </h1>
                <span v-else>{{ item.label }}</span>
            </li>
        </ul>
    </div>
</template>
