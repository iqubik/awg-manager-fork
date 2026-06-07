import { describe, expect, it } from "vitest";
import type { DnsRoute } from "$lib/types";
import { exportRoutes, parseImportFile } from "./dns-export";

describe("dns-export", () => {
  it("exports custom route icons", () => {
    const routes = [
      {
        id: "dns-1",
        name: "Custom DNS",
        domains: [],
        manualDomains: ["example.com"],
        routes: [],
        enabled: true,
        createdAt: "2026-05-30T00:00:00Z",
        updatedAt: "2026-05-30T00:00:00Z",
        iconUrl: "data:image/png;base64,abc",
      },
    ] as DnsRoute[];

    expect(exportRoutes(routes)[0].iconUrl).toBe("data:image/png;base64,abc");
  });

  it("keeps custom route icons when parsing imported DNS route files", () => {
    const parsed = parseImportFile(
      JSON.stringify([
        {
          name: "Imported DNS",
          manualDomains: ["example.com"],
          enabled: true,
          iconUrl: "https://example.com/icon.png",
        },
      ]),
    );

    expect(parsed).toHaveLength(1);
    expect(parsed[0].iconUrl).toBe("https://example.com/icon.png");
  });

  it("rejects invalid iconUrl values in imported DNS route files", () => {
    const parsed = parseImportFile(
      JSON.stringify([
        {
          name: "Broken DNS",
          manualDomains: ["example.com"],
          enabled: true,
          iconUrl: 123,
        },
      ]),
    );

    expect(parsed).toHaveLength(0);
  });
});
