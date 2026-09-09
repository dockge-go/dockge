import { Show, createMemo } from "solid-js";

import { PAGE_SIZE, clampPage, pageCount } from "../lib/pagination";
import { t } from "../i18n";

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
    <nav class="pagination" aria-label={t("aria.pagination")}>
      <span class="pagination-meta">
        {t("pagination.range", { start: start(), end: end(), total: props.total })}
      </span>
      <Show when={pages() > 1}>
        <div class="pagination-controls">
          <button
            class="pagination-button"
            disabled={current() === 1}
            onClick={() => props.onPageChange(current() - 1)}
          >
            {t("pagination.prev")}
          </button>
          <span class="pagination-current" aria-current="page">{t("pagination.pageOf", { p: current(), n: pages() })}</span>
          <button
            class="pagination-button"
            disabled={current() === pages()}
            onClick={() => props.onPageChange(current() + 1)}
          >
            {t("pagination.next")}
          </button>
        </div>
      </Show>
    </nav>
  );
}
