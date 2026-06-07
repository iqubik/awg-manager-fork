import { describe, expect, it } from "vitest";
import type { StaticRouteList } from "$lib/types";
import {
  exportStaticRoutes,
  parseStaticRouteImport,
} from "./staticroute-export";

describe("staticroute-export", () => {
  it("exports custom route icons", () => {
    const routes = [
      {
        id: "ip-1",
        name: "Custom IP",
        tunnelID: "tun-1",
        subnets: ["10.0.0.0/8"],
        enabled: true,
        createdAt: "2026-05-30T00:00:00Z",
        updatedAt: "2026-05-30T00:00:00Z",
        iconUrl: "data:image/png;base64,abc",
      },
    ] as StaticRouteList[];

    expect(exportStaticRoutes(routes)[0].iconUrl).toBe(
      "data:image/png;base64,abc",
    );
  });

  it("keeps custom route icons when parsing imported static route files", () => {
    const parsed = parseStaticRouteImport(
      JSON.stringify([
        {
          name: "Imported IP",
          subnets: ["10.0.0.0/8"],
          enabled: true,
          iconUrl: "https://example.com/icon.png",
        },
      ]),
    );

    expect(parsed).toHaveLength(1);
    expect(parsed[0].iconUrl).toBe("https://example.com/icon.png");
  });

  it("rejects invalid iconUrl values in imported static route files", () => {
    const parsed = parseStaticRouteImport(
      JSON.stringify([
        {
          name: "Broken IP",
          subnets: ["10.0.0.0/8"],
          enabled: true,
          iconUrl: 123,
        },
      ]),
    );

    expect(parsed).toHaveLength(0);
  });
});
