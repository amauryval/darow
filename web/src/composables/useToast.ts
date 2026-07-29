import { ref } from "vue";

export interface Toast {
  id: number;
  kind: "success" | "error";
  message: string;
}

// Module-level state: every component sharing this composable reads/writes
// the same toast queue, which is what lets Toast.vue (mounted once in
// App.vue) render toasts pushed from any view.
const toasts = ref<Toast[]>([]);
let nextId = 0;

function push(kind: Toast["kind"], message: string) {
  const id = nextId++;
  toasts.value.push({ id, kind, message });
  setTimeout(() => dismiss(id), 4000);
}

function dismiss(id: number) {
  toasts.value = toasts.value.filter((t) => t.id !== id);
}

export function useToast() {
  return {
    toasts,
    success: (message: string) => push("success", message),
    error: (message: string) => push("error", message),
    dismiss,
  };
}
