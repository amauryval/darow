<script setup lang="ts">
// A row of buttons welded into one control (daisyUI `join`). The children
// carry the `joined` prop of BaseButton — daisyUI needs the modifier on each
// item, and a wrapper cannot put it there without reaching into the slot.
</script>

<template>
    <div class="join">
        <slot />
    </div>
</template>
