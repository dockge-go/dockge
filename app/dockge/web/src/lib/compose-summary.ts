export interface ComposeSummary {
  readonly services: readonly string[];
  readonly ports: number;
  readonly volumes: number;
  readonly networks: number;
}

type Section = "services" | "volumes" | "networks" | null;

export function summarizeCompose(yaml: string): ComposeSummary {
  const services: string[] = [];
  let section: Section = null;
  let ports = 0;
  let volumes = 0;
  let networks = 0;
  let inPorts = false;

  for (const rawLine of yaml.split("\n")) {
    const line = rawLine.replace(/\s+$/, "");
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;

    const indent = line.length - line.trimStart().length;
    if (indent === 0) {
      inPorts = false;
      section = trimmed === "services:" ? "services"
        : trimmed === "volumes:" ? "volumes"
        : trimmed === "networks:" ? "networks"
        : null;
      continue;
    }

    if (section === "services") {
      if (indent === 2 && trimmed.endsWith(":")) {
        services.push(trimmed.slice(0, -1));
        inPorts = false;
      } else if (indent === 4) {
        inPorts = trimmed === "ports:";
      } else if (inPorts && indent >= 6 && trimmed.startsWith("- ")) {
        ports++;
      }
      continue;
    }

    if (indent === 2 && trimmed.endsWith(":")) {
      if (section === "volumes") volumes++;
      if (section === "networks") networks++;
    }
  }

  return { services, ports, volumes, networks };
}
