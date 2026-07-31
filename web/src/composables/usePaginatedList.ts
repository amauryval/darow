import { onMounted, onUnmounted, ref, type Ref } from "vue";
import type { SortingState, Updater } from "@tanstack/vue-table";

export interface Page<T> {
  items: T[];
  total: number;
}

export interface UsePaginatedListOptions<T> {
  limit?: number;
  initialSorting?: SortingState;
  sortColumnMap?: Record<string, string>;
  fetchPage: (offset: number, limit: number, sort: string) => Promise<Page<T>>;
  onError: (e: unknown) => void;
}

// Infinite-scroll list backed by offset/limit + server-side sorting, the
// pattern previously copied into TablesView, SourcesView and
// TableDataPanel (TODO-F4). Sorting is server-side, so any change to it
// invalidates every page already loaded — reloadSorted() re-fetches page 1
// but keeps the current items on screen until the new page lands, so a
// sort change never flashes an empty table (règle 8.1).
export function usePaginatedList<T>(options: UsePaginatedListOptions<T>) {
  const limit = options.limit ?? 50;
  const items = ref<T[]>([]) as Ref<T[]>;
  const total = ref(0);
  const offset = ref(0);
  const loading = ref(false);
  const loadedOnce = ref(false);
  const sorting = ref<SortingState>(options.initialSorting ?? []);

  function sortQuery(): string {
    return sorting.value.map((s) => `${s.desc ? "-" : ""}${options.sortColumnMap?.[s.id] ?? s.id}`).join(",");
  }

  async function loadMore() {
    if (loading.value || (loadedOnce.value && items.value.length >= total.value)) return;
    loading.value = true;
    try {
      const page = await options.fetchPage(offset.value, limit, sortQuery());
      items.value = [...items.value, ...page.items];
      total.value = page.total;
      offset.value += page.items.length;
      loadedOnce.value = true;
    } catch (e) {
      options.onError(e);
    } finally {
      loading.value = false;
    }
  }

  // A mutating action (add/edit/delete) can shift which items belong on
  // which page, and re-fetching only the pages already loaded wouldn't
  // re-derive the running total correctly either — resetting to page 1 is
  // the simplest correct behavior.
  async function reload() {
    items.value = [];
    total.value = 0;
    offset.value = 0;
    loadedOnce.value = false;
    await loadMore();
  }

  // Like reload(), but fetches page 1 before touching items.value, so the
  // list keeps showing its current items instead of flashing empty while
  // the request is in flight — used for sort changes.
  async function reloadSorted() {
    loading.value = true;
    try {
      const page = await options.fetchPage(0, limit, sortQuery());
      items.value = page.items;
      total.value = page.total;
      offset.value = page.items.length;
      loadedOnce.value = true;
    } catch (e) {
      options.onError(e);
    } finally {
      loading.value = false;
    }
  }

  function setSorting(updater: Updater<SortingState>) {
    sorting.value = typeof updater === "function" ? updater(sorting.value) : updater;
    reloadSorted();
  }

  // Lets a caller replace the current page's items in place after a
  // server-side action that returns a fresh page itself (e.g. a "refresh
  // from disk" endpoint), without going through fetchPage again.
  function replace(page: Page<T>) {
    items.value = page.items;
    total.value = page.total;
    offset.value = page.items.length;
    loadedOnce.value = true;
  }

  const sentinel = ref<HTMLElement | null>(null);
  let observer: IntersectionObserver | null = null;

  onMounted(() => {
    observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadMore();
      },
      { rootMargin: "200px" },
    );
    if (sentinel.value) observer.observe(sentinel.value);
    loadMore();
  });

  onUnmounted(() => observer?.disconnect());

  return { items, total, loading, loadedOnce, sorting, sentinel, loadMore, reload, reloadSorted, setSorting, replace, sortQuery };
}
