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
