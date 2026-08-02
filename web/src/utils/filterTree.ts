import type { FilterCondition, RowValue } from "../client/types";

export interface DraftLeaf {
  id: number;
  kind: "leaf";
  column: string;
  operator: string;
  value: string;
  active: boolean;
}

export interface DraftGroup {
  id: number;
  kind: "group";
  bool: "AND" | "OR";
  children: DraftNode[];
  active: boolean;
}

export type DraftNode = DraftLeaf | DraftGroup;

let nextId = 0;
export function nextNodeId(): number {
  return nextId++;
}

// Drops inactive nodes entirely rather than emitting them: a group left
// with no active children after pruning is dropped too, since BuildFilter
// rejects an empty boolean group.
export function pruneActive(nodes: DraftNode[], coerce: (column: string, value: string) => RowValue): FilterCondition[] {
  const result: FilterCondition[] = [];
  for (const node of nodes) {
    if (!node.active) continue;
    if (node.kind === "leaf") {
      result.push({ column: node.column, operator: node.operator, value: coerce(node.column, node.value) });
      continue;
    }
    const children = pruneActive(node.children, coerce);
    if (children.length > 0) result.push({ bool: node.bool, conditions: children });
  }
  return result;
}

// Tree manipulation utilities — centralize all node updates here.
// Root-level nodes are independent filters (only one active at a time).
// Nested nodes (inside a group) toggle independently.
export function toggleNode(nodes: DraftNode[], id: number, isRoot: boolean): DraftNode[] {
  if (!isRoot) {
    return nodes.map((n) => (n.id === id ? { ...n, active: !n.active } : n));
  }
  const target = nodes.find((n) => n.id === id);
  return nodes.map((n) => ({ ...n, active: n.id === id ? !target?.active : false }));
}

export function removeNode(nodes: DraftNode[], id: number): DraftNode[] {
  return nodes.filter((n) => n.id !== id);
}

export function updateChildren(nodes: DraftNode[], groupId: number, children: DraftNode[]): DraftNode[] {
  return nodes.map((n) => (n.id === groupId ? { ...n, children } : n));
}
