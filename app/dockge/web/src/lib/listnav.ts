// j/k 列表导航（Unix 惯例）：在容器内按 j/k 上下移动行焦点。
// 行元素须有 tabindex（resource-row 已有；表格行由各视图补充 tabindex="0"）。
// 输入框聚焦时不拦截，Enter/点击行为由行自身承担。

/** 绑定容器：返回解绑函数。rowSelector 限定可聚焦行（可见的）。 */
export function listNav(container: HTMLElement, rowSelector = ".resource-row[tabindex], tbody tr[tabindex]"): () => void {
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== "j" && e.key !== "k") return;
    const target = e.target as HTMLElement;
    if (target.closest("input, textarea, select, [contenteditable]")) return;
    const rows = Array.from(container.querySelectorAll<HTMLElement>(rowSelector)).filter(
      (row) => row.offsetParent !== null,
    );
    if (rows.length === 0) return;
    e.preventDefault();
    const idx = rows.indexOf(document.activeElement as HTMLElement);
    const next =
      e.key === "j"
        ? idx < 0 ? 0 : Math.min(idx + 1, rows.length - 1)
        : idx < 0 ? rows.length - 1 : Math.max(idx - 1, 0);
    rows[next].focus();
  };
  container.addEventListener("keydown", onKeyDown);
  return () => container.removeEventListener("keydown", onKeyDown);
}
