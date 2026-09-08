import type { ContainerRow, ContainerStatusFrame } from "../api/api";

export function mergeContainerStatus(
  containers: readonly ContainerRow[],
  frame: ContainerStatusFrame,
): ContainerRow[] {
  const statuses = new Map(frame.containers.map((container) => [container.id, container]));
  return containers.map((container) => {
    const status = statuses.get(container.id);
    return status ? { ...container, state: status.state, status: status.status } : container;
  });
}
