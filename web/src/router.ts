import { createRouter, createWebHistory } from "vue-router";
import SourcesView from "./views/SourcesView.vue";
import TablesView from "./views/TablesView.vue";
import TableDetailView from "./views/TableDetailView.vue";

export const router = createRouter({
  history: createWebHistory("/ui/"),
  routes: [
    { path: "/", redirect: "/tables" },
    { path: "/sources", name: "sources", component: SourcesView },
    { path: "/tables", name: "tables", component: TablesView },
    { path: "/tables/:tableId", name: "table-detail", component: TableDetailView, props: true },
  ],
});
