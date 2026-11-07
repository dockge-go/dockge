/* YAML 能力：js-yaml 解析校验、服务名解析、规范化格式化。
 * 编辑器实时校验与部署前强制校验共用同一实现。 */
import yaml from 'js-yaml';

export interface YamlProblem { line: number | null; msg: string }

export function yamlProblem(code: string): YamlProblem | null {
  try {
    yaml.load(code);
    return null;
  } catch (e) {
    const err = e as { message?: string; mark?: { line?: number } };
    const line = err.mark && typeof err.mark.line === 'number' ? err.mark.line + 1 : null;
    const msg = String(err.message || '').replace(/\s*\(\d+:\d+\)\s*$/, '');
    return { line, msg };
  }
}

export function parseServices(code: string): string[] {
  try {
    const doc = yaml.load(code) as { services?: Record<string, unknown> } | null;
    if (!doc || typeof doc !== 'object' || !doc.services || typeof doc.services !== 'object') return [];
    return Object.keys(doc.services);
  } catch (e) {
    return [];
  }
}

export function normalizeYaml(code: string): string {
  return yaml.dump(yaml.load(code), { indent: 2, lineWidth: 120, noRefs: true });
}
