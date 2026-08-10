<script setup lang="ts">
import { LogIn, LogOut, ShieldAlert, User } from '@lucide/vue'
import { useSession } from '../../composables/useSession'
import BaseButton from '../ui/BaseButton.vue'
import MenuDropdown from '../ui/MenuDropdown.vue'
import MenuItem from '../ui/MenuItem.vue'
import MenuTitle from '../ui/MenuTitle.vue'

// The navbar's account control. It renders nothing at all when the server
// runs with authentication off: there is no account to show, and an empty
// affordance would suggest one exists.
const {
    enabled,
    authenticated,
    writeRefused,
    displayName,
    canSignIn,
    canSignOut,
    signIn,
    signOut
} = useSession()
</script>

<template>
    <BaseButton
        v-if="enabled && !authenticated"
        variant="secondary"
        size="sm"
        class="gap-1.5"
        :disabled="!canSignIn"
        :title="
            canSignIn
                ? 'Se connecter pour modifier les données'
                : 'Aucune page de connexion configurée sur ce serveur'
        "
        @click="signIn"
    >
        <LogIn :size="14" />
        Se connecter
    </BaseButton>

    <MenuDropdown
        v-else-if="enabled"
        :label="displayName"
        :icon="writeRefused ? ShieldAlert : User"
        align="right"
        :title="displayName"
    >
        <MenuTitle>{{ displayName }}</MenuTitle>
        <!-- Authenticated but outside every write group: saying so here is
             the only place the user learns it before a save fails. -->
        <MenuItem v-if="writeRefused" :icon="ShieldAlert" disabled>Lecture seule</MenuItem>
        <MenuItem v-if="canSignOut" :icon="LogOut" @click="signOut">Se déconnecter</MenuItem>
    </MenuDropdown>
</template>
