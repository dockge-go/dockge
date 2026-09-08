import { Show, createMemo } from "solid-js";

import { PAGE_SIZE, clampPage, pageCount } from "../lib/pagination";

export function Pagination(props: {
  page: number;
  total: number;
  onPageChange: (page: number) => void;
}) {
  const pages = createMemo(() => pageCount(props.total));
  const current = createMemo(() => clampPage(props.page, props.total));
  const start = createMemo(() => (props.total === 0 ? 0 : (current() - 1) * PAGE_SIZE + 1));
  const end = createMemo(() => Math.min(current() * PAGE_SIZE, props.total));

  return (
    <nav class="pagination" aria-label="列表分页">
      <span class="pagination-meta">
        {start()}–{end()} / {props.total}
      </span>
      <Show when={pages() > 1}>
        <div class="pagination-controls">
          <button
            class="pagination-button"
            disabled={current() === 1}
            onClick={() => props.onPageChange(current() - 1)}
          >
            上一页
          </button>
          <span class="pagination-current" aria-current="page">第 {current()} / {pages()} 页</span>
          <button
            class="pagination-button"
            disabled={current() === pages()}
            onClick={() => props.onPageChange(current() + 1)}
          >
            下一页
          </button>
        </div>
      </Show>
    </nav>
  );
}
