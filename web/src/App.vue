<script setup lang="ts">
import { useStorage } from '@vueuse/core'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import {
    BookOpen,
    FolderOpen,
    LayoutDashboard,
    LayoutGrid,
    PanelLeftClose,
    PanelLeftOpen,
    Settings as SettingsIcon
} from '@lucide/vue'
import { useServerStatus, type ServerStatus } from './composables/useServerStatus'
import { useSettings } from './composables/useSettings'
import { useVersion } from './composables/useVersion'
import SessionMenu from './components/session/SessionMenu.vue'
import Toast from './components/ui/Toast.vue'
import Badge from './components/ui/Badge.vue'
import BaseButton from './components/ui/BaseButton.vue'
import Logo from './components/ui/Logo.vue'
import Typography from './components/ui/Typography.vue'

const route = useRoute()
const { status, label: statusLabel } = useServerStatus()
// Loaded here, once, so every list composable mounted afterwards
// (useSources, useTableRows, TablesView) reads a live cap instead of racing
// its own /settings fetch (§2.4's pagination trap).
useSettings()
const { current: version } = useVersion()

const statusBadgeTone: Record<ServerStatus, 'neutral' | 'success' | 'error'> = {
    checking: 'neutral',
    ok: 'success',
    down: 'error'
}

// Persisted so the operator's choice of a dense, icon-only sidebar survives a
// reload — it is a display preference, not screen state, so it stays out of
// the URL (§9).
const collapsed = useStorage('darow-nav-collapsed', false)

const links = [
    { to: '/dashboard', label: 'Tableau de bord', icon: LayoutDashboard },
    { to: '/tables', label: 'Tables disponibles', icon: LayoutGrid },
    { to: '/sources', label: 'Sources', icon: FolderOpen },
    { to: '/settings', label: 'Paramètres', icon: SettingsIcon }
]
</script>

<template>
    <div class="drawer lg:drawer-open h-screen bg-base-200">
        <input id="nav-drawer" type="checkbox" class="drawer-toggle" />

        <div class="drawer-content flex min-w-0 flex-1 flex-col overflow-hidden">
            <header
                class="navbar min-h-12 shrink-0 gap-1 border-b border-base-300 bg-base-100 px-4"
            >
                <div class="flex flex-1 items-center gap-1">
                    <!-- daisyUI's drawer needs a <label for> bound to its checkbox
               input, which BaseButton cannot be — the one deliberate daisyUI
               class outside components/ui/ (§0.10). -->
                    <label
                        for="nav-drawer"
                        class="btn btn-square btn-ghost btn-sm lg:hidden"
                        aria-label="Ouvrir la navigation"
                    >
                        <LayoutGrid :size="18" />
                    </label>
                    <BaseButton
                        variant="ghost"
                        size="sm"
                        square
                        class="hidden lg:inline-flex"
                        :aria-label="
                            collapsed ? 'Développer la navigation' : 'Réduire la navigation'
                        "
                        :title="collapsed ? 'Développer la navigation' : 'Réduire la navigation'"
                        @click="collapsed = !collapsed"
                    >
                        <PanelLeftOpen v-if="collapsed" :size="18" />
                        <PanelLeftClose v-else :size="18" />
                    </BaseButton>
                    <Typography variant="h4" class="ml-1">{{ route.meta.title }}</Typography>
                </div>
                <SessionMenu />
            </header>

            <RouterView class="flex min-h-0 flex-1 flex-col" />
        </div>

        <div class="drawer-side z-30">
            <label
                for="nav-drawer"
                aria-label="Fermer la navigation"
                class="drawer-overlay"
            ></label>

            <aside
                class="flex h-full flex-col border-r border-base-300 bg-base-100 transition-[width] duration-150"
                :class="collapsed ? 'w-16' : 'w-60'"
            >
                <div class="flex items-center gap-2.5 overflow-hidden px-4 py-4">
                    <span
                        class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-content"
                    >
                        <Logo :size="18" />
                    </span>
                    <div v-show="!collapsed" class="leading-tight whitespace-nowrap">
                        <Typography variant="h4">Darow</Typography>
                        <Typography variant="caption">Console d'administration</Typography>
                    </div>
                </div>

                <ul
                    class="menu menu-vertical w-full flex-1 gap-0.5 px-2"
                    :class="collapsed && 'px-2'"
                >
                    <li v-for="link in links" :key="link.to">
                        <RouterLink
                            :to="link.to"
                            active-class="menu-active"
                            :class="collapsed && 'tooltip tooltip-right justify-center px-0'"
                            :data-tip="collapsed ? link.label : undefined"
                            :aria-label="collapsed ? link.label : undefined"
                        >
                            <component :is="link.icon" :size="18" class="shrink-0" />
                            <span v-show="!collapsed">{{ link.label }}</span>
                        </RouterLink>
                    </li>
                </ul>

                <a
                    href="/swagger/"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="flex items-center gap-2.5 border-t border-base-300 px-4 py-2.5 text-sm text-base-content/70 hover:bg-base-200"
                    :class="collapsed && 'tooltip tooltip-right justify-center px-0'"
                    :data-tip="collapsed ? 'Documentation API' : undefined"
                    :aria-label="collapsed ? 'Documentation API' : undefined"
                >
                    <BookOpen :size="18" class="shrink-0" />
                    <span v-show="!collapsed">Documentation API</span>
                </a>

                <div
                    class="flex items-center gap-2 border-t border-base-300 px-3 py-3"
                    :class="collapsed && 'justify-center'"
                >
                    <Badge :tone="statusBadgeTone[status]" :title="statusLabel[status]">
                        <span v-show="!collapsed">{{ statusLabel[status] }}</span>
                    </Badge>
                </div>

                <div
                    v-if="version"
                    class="truncate px-4 pb-3 text-xs text-base-content/50"
                    :class="collapsed && 'text-center'"
                    :title="`${version.version} (${version.commit}) — ${version.buildDate}`"
                >
                    <template v-if="collapsed">{{ version.commit.slice(0, 4) }}</template>
                    <template v-else
                        >{{ version.version }} · {{ version.buildDate.slice(0, 10) }}</template
                    >
                </div>
            </aside>
        </div>

        <Toast />
    </div>
</template>
