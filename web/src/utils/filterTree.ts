import type { FilterCondition, RowValue } from "../client/types";

// Operators that test the column itself: they take no value, and are the only
// way to match a NULL at all (SQL's "col = NULL" is never true).
const VALUELESS_OPERATORS = ["IS NULL", "IS NOT NULL"];

export function takesValue(operator: string): boolean {
  return !VALUELESS_OPERATORS.includes(operator);
}

export interface DraftLeaf {
  id: number;
  kind: "leaf";
  column: string;
  operator: string;
  value: string;
  // Negates this condition alone; a group's own `not` negates the whole group.
  not: boolean;
  active: boolean;
}

export interface DraftGroup {
  id: number;
  kind: "group";
  bool: "AND" | "OR";
  children: DraftNode[];
  not: boolean;
  active: boolean;
}

export type DraftNode = DraftLeaf | DraftGroup;

let nextId = 0;
export function nextNodeId(): number {
  return nextId++;
}

// The one shape a fresh, empty condition takes — shared by the "Filtres"
// button (creates one directly) and the panel's own "Ajouter un filtre"
// (so the two never drift apart on the default operator).
export function createLeaf(column: string): DraftLeaf {
  return { id: nextNodeId(), kind: "leaf", column, operator: "=", value: "", not: false, active: true };
}

// Drops inactive nodes entirely rather than emitting them: a group left
// with no active children after pruning is dropped too, since BuildFilter
// rejects an empty boolean group.
export function pruneActive(nodes: DraftNode[], coerce: (column: string, value: string) => RowValue): FilterCondition[] {
  const result: FilterCondition[] = [];
  for (const node of nodes) {
    if (!node.active) continue;
    if (node.kind === "leaf") {
      result.push({
        column: node.column,
        operator: node.operator,
        value: takesValue(node.operator) ? coerce(node.column, node.value) : undefined,
        not: node.not || undefined,
      });
      continue;
    }
    const children = pruneActive(node.children, coerce);
    if (children.length > 0) result.push({ bool: node.bool, conditions: children, not: node.not || undefined });
  }
  return result;
}

// Tree manipulation utilities — centralize all node updates here. Which
// filter is applied is not one of them: a filter is a stored object now, so
// the bar tracks it by its identifier rather than by an `active` flag walked
// through the tree.
export function removeNode(nodes: DraftNode[], id: number): DraftNode[] {
  return nodes.filter((n) => n.id !== id);
}

export function updateChildren(nodes: DraftNode[], groupId: number, children: DraftNode[]): DraftNode[] {
  return nodes.map((n) => (n.id === groupId ? { ...n, children } : n));
}

// One-line rendering of a node, for a pill label and for the name a filter is
// saved under. A group is spelled out in parentheses rather than summarized by
// its size ("2 conditions" names nothing a user would recognize in a list);
// the caller truncates, since only it knows the room it has.
function describeNode(node: DraftNode): string {
  const negation = node.not ? "NON " : "";
  if (node.kind === "leaf") {
    const value = takesValue(node.operator) ? ` ${node.value}` : "";
    return `${negation}${node.column} ${node.operator}${value}`;
  }
  return `${negation}(${describeDraft(node.bool, node.children)})`;
}

// The default name a new filter is saved under: the conditions themselves,
// joined by their connector. Every filter is saved, so this has to read as a
// name a user would recognize without having typed one.
export function describeDraft(bool: "AND" | "OR", children: DraftNode[]): string {
  return children.map(describeNode).join(bool === "AND" ? " et " : " ou ");
}

// The editor always works on a boolean operator plus a list of conditions,
// whatever the edited node's shape: a bare leaf is presented as a one-child
// AND, so the modal has a single form to render.
export function draftOf(node: DraftNode | null, column: string): { bool: "AND" | "OR"; children: DraftNode[] } {
  if (node === null) return { bool: "AND", children: [createLeaf(column)] };
  if (node.kind === "group") return { bool: node.bool, children: node.children };
  return { bool: "AND", children: [node] };
}

// The inverse of draftOf: a single condition collapses back to a bare leaf,
// so the pill row reads "height > 10" instead of a one-child group. An empty
// list yields null — BuildFilter rejects an empty boolean group.
export function nodeOf(bool: "AND" | "OR", children: DraftNode[], active: boolean): DraftNode | null {
  if (children.length === 0) return null;
  if (children.length === 1) return { ...children[0], active };
  return { id: nextNodeId(), kind: "group", bool, children, not: false, active };
}

// A leaf with no value filters nothing, and would be pruned away silently;
// the editor blocks on it instead of letting the user save a no-op.
export function isComplete(node: DraftNode): boolean {
  if (node.kind === "leaf") return node.column !== "" && (!takesValue(node.operator) || node.value.trim() !== "");
  return node.children.length > 0 && node.children.every(isComplete);
}

// Rebuilds an editable tree from the server DSL — the inverse of pruneActive,
// used when a saved filter is loaded back. Values come back typed (number,
// boolean, null) where the editor holds text, so they are stringified; null
// becomes the empty string, which is exactly what coerceByType maps back.
export function fromConditions(conditions: FilterCondition[], active: boolean): DraftNode[] {
  return conditions.map((cond) => {
    if (cond.bool && cond.conditions) {
      return {
        id: nextNodeId(),
        kind: "group",
        bool: cond.bool,
        children: fromConditions(cond.conditions, true),
        not: cond.not ?? false,
        active,
      } satisfies DraftGroup;
    }
    return {
      id: nextNodeId(),
      kind: "leaf",
      column: cond.column ?? "",
      operator: cond.operator ?? "=",
      value: cond.value === null || cond.value === undefined ? "" : String(cond.value),
      not: cond.not ?? false,
      active,
    } satisfies DraftLeaf;
  });
}
