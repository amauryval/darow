<script setup lang="ts">
import { ref, watch } from "vue";

// A free-text input that offers known values without constraining to them:
// the caller loads `options` in reaction to `search`, and the field stays
// typable when nothing matches. Options are opened on focus and on every
// keystroke, so the list always describes what is currently typed.
const props = withDefaults(
  defineProps<{
    modelValue?: string;
    options: string[];
    loading?: boolean;
    size?: "xs" | "sm" | "md";
    placeholder?: string;
    ariaLabel?: string;
  }>(),
  { size: "md" },
);

const emit = defineEmits<{ "update:modelValue": [value: string]; search: [text: string] }>();

const sizes = {
  xs: "input-xs",
  sm: "input-sm",
  md: "",
};

const open = ref(false);
const highlighted = ref(-1);

// A refreshed list must not keep pointing at the row that used to be there.
watch(
  () => props.options,
  () => (highlighted.value = -1),
);

function onInput(event: Event) {
  if (!(event.target instanceof HTMLInputElement)) return;
  emit("update:modelValue", event.target.value);
  emit("search", event.target.value);
  open.value = true;
}

function onFocus() {
  open.value = true;
  emit("search", props.modelValue ?? "");
}

function select(option: string) {
  emit("update:modelValue", option);
  open.value = false;
}

// mousedown, not click: blur fires first and would close the list before the
// click ever lands on an option.
function onOptionMouseDown(event: MouseEvent, option: string) {
  event.preventDefault();
  select(option);
}

function move(delta: number) {
  if (props.options.length === 0) return;
  open.value = true;
  const next = highlighted.value + delta;
  highlighted.value = (next + props.options.length) % props.options.length;
}

function onEnter(event: KeyboardEvent) {
  const option = props.options[highlighted.value];
  if (!open.value || option === undefined) return;
  // The modal submits on Enter; picking a suggestion is what this key means
  // while the list is open.
  event.preventDefault();
  event.stopPropagation();
  select(option);
}
</script>

<template>
  <!-- The list is positioned out of the flow so it never lengthens what
       contains it; its container (Modal, with `overflowVisible`) is what has
       to let it spill over. -->
  <div class="relative">
    <input
      :value="modelValue"
      type="text"
      role="combobox"
      aria-autocomplete="list"
      :aria-expanded="open"
      :placeholder="placeholder"
      :aria-label="ariaLabel"
      class="input input-bordered w-full"
      :class="sizes[size]"
      autocomplete="off"
      @input="onInput"
      @focus="onFocus"
      @blur="open = false"
      @keydown.down.prevent="move(1)"
      @keydown.up.prevent="move(-1)"
      @keydown.esc="open = false"
      @keydown.enter="onEnter"
    />

    <ul
      v-if="open && (loading || options.length > 0)"
      class="absolute z-30 mt-1 max-h-56 w-full overflow-y-auto rounded-md border border-base-300 bg-base-100 py-1 shadow-lg"
      role="listbox"
    >
      <li v-if="loading" class="px-2 py-1 text-xs text-base-content/40">Chargement…</li>
      <li
        v-for="(option, index) in options"
        v-else
        :key="option"
        role="option"
        :aria-selected="index === highlighted"
        class="cursor-pointer truncate px-2 py-1 text-sm"
        :class="index === highlighted ? 'bg-primary text-primary-content' : 'hover:bg-base-200'"
        @mousedown="onOptionMouseDown($event, option)"
        @mousemove="highlighted = index"
      >
        {{ option }}
      </li>
    </ul>
  </div>
</template>
