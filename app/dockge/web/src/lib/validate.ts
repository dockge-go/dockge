// 校验诊断映射：/stacks/validate 的结构化错误 → 编辑器诊断。
// 行号越界或为 0 时不做行内标记（from=to=0，仅保留面板列表项），避免错位。

import type { ValidateError, ValidateResult } from "../api/api";

/** 编辑器校验状态：检查中 / 已完成（含结果）。 */
export type ValidationState = { status: "checking" } | { status: "done"; result: ValidateResult };

/** 与 CodeMirror Diagnostic 结构对齐（from/to/message/severity），保持纯函数可测。 */
export interface EditorDiagnostic {
  from: number;
  to: number;
  message: string;
  severity: "error";
}

/** 把后端错误映射为编辑器诊断；doc 为当前 compose 文本，用于行号 → 偏移换算。 */
export function toDiagnostics(errors: ValidateError[], doc: string): EditorDiagnostic[] {
  const lines = doc.split("\n");
  return errors.map((error) => {
    if (error.line < 1 || error.line > lines.length) {
      return { from: 0, to: 0, message: error.message, severity: "error" };
    }
    let from = 0;
    for (let i = 0; i < error.line - 1; i++) from += lines[i].length + 1;
    return { from, to: from + lines[error.line - 1].length, message: error.message, severity: "error" };
  });
}
