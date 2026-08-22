<script setup lang="ts">
import { computed } from 'vue'
import { LogIn, LogOut, User } from '@lucide/vue'
import { useSession } from '../../composables/useSession'
import BaseButton from '../ui/BaseButton.vue'
import Badge from '../ui/Badge.vue'
import MenuDropdown from '../ui/MenuDropdown.vue'
import MenuItem from '../ui/MenuItem.vue'
import MenuTitle from '../ui/MenuTitle.vue'

// The navbar's account control. It renders nothing at all when the server
// runs with authentication off: there is no account to show, and an empty
// affordance would suggest one exists.
const { enabled, authenticated, role, displayName, signIn, signOut } = useSession()

// The badge is the one thing the menu could not say before this control
// existed: a guest can edit a table's data but never load one (back §6.7),
// while a signed-in user holding neither role gets nothing beyond an
// anonymous visitor — without this label either screen would look broken
// rather than merely restricted.
const roleBadge = computed<{ tone: 'primary' | 'warning'; label: string } | null>(() => {
    if (role.value === 'admin') return { tone: 'primary', label: 'Administrateur' }
    if (role.value === 'guest')
        return { tone: 'warning', label: 'Invité — pas de chargement de données' }
    return { tone: 'warning', label: 'Aucun rôle — lecture seule' }
})
</script>

<template>
    <BaseButton
        v-if="enabled && !authenticated"
        variant="secondary"
        size="sm"
        class="gap-1.5"
        @click="signIn"
    >
        <LogIn :size="14" />
    </BaseButton>

    <MenuDropdown
        v-else-if="enabled"
        :label="displayName"
        :icon="User"
        align="right"
        :title="displayName"
    >
        <MenuTitle>{{ displayName }}</MenuTitle>
        <li v-if="roleBadge" class="px-2 py-1">
            <Badge :tone="roleBadge.tone">{{ roleBadge.label }}</Badge>
        </li>
        <MenuItem :icon="LogOut" @click="signOut">Se déconnecter</MenuItem>
    </MenuDropdown>
</template>
