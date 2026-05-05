#!/usr/bin/env python3
"""Smoke test for /subscriptions section.

Run with mock-proxy already up:
    cd frontend && npm run dev:mock:proxy &
    uv run --with playwright python3 scripts/smoke-subscription.py
"""

from __future__ import annotations
import os
import sys
from playwright.sync_api import sync_playwright

BASE = os.environ.get("BASE", "http://127.0.0.1:5173")
HEADLESS = os.environ.get("HEADLESS", "1") != "0"


def main() -> int:
    failed = False
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=HEADLESS)
        page = browser.new_page(viewport={"width": 1600, "height": 900})
        try:
            print("[smoke-sub] navigate / and switch to Подписки tab")
            page.goto(f"{BASE}/")
            page.wait_for_load_state("networkidle")

            # The Tabs component renders a hidden measure-row (aria-hidden) and a
            # visible tab-row. get_by_role excludes aria-hidden children.
            page.get_by_role("button", name="Подписки").click()
            page.wait_for_timeout(300)

            print("[smoke-sub] click + Добавить подписку")
            page.get_by_role("button", name="Добавить подписку").click()
            page.wait_for_timeout(300)

            print("[smoke-sub] fill form")
            page.locator('input[type="text"]').first.fill("Test Sub")
            page.locator('input[type="url"]').first.fill("https://example.com/sub")
            page.get_by_role("button", name="Создать").click()

            print("[smoke-sub] await detail page")
            page.wait_for_url("**/subscriptions/sub-*", timeout=10000)

            print("[smoke-sub] verify selector tag visible")
            page.wait_for_selector('text=/sub-/', timeout=5000)

            # ── Clash YAML subscription create ─────────────────────────────
            print("[smoke-sub] navigate / and create Clash YAML subscription")
            page.goto(f"{BASE}/")
            page.wait_for_load_state("networkidle")
            page.get_by_role("button", name="Подписки").click()
            page.wait_for_timeout(300)

            page.get_by_role("button", name="Добавить подписку").click()
            page.wait_for_timeout(300)

            page.locator('input[type="text"]').first.fill("Clash Mock")
            page.locator('input[type="url"]').first.fill(
                f"{BASE}/__mock__/clash-subscription.yaml"
            )
            page.get_by_role("button", name="Создать").click()

            print("[smoke-sub] await Clash detail page")
            page.wait_for_url("**/subscriptions/sub-*", timeout=10000)
            page.wait_for_load_state("networkidle")

            print("[smoke-sub] verify 3 Clash member labels visible")
            for label in ("🇺🇸 LA-1 (mock)", "🇩🇪 FRA-1 (mock)", "🇯🇵 TYO-1 (mock)"):
                if not page.get_by_text(label).first.is_visible(timeout=5000):
                    raise AssertionError(f"member label {label!r} not visible on Clash detail page")

            page.screenshot(path="/tmp/subscription-clash-yaml-created.png", full_page=True)
            print("[smoke-sub] Clash YAML create flow OK")

            # Active member surfacing on Sing-box tab + in-card swap
            print("[smoke-sub] navigate to / and switch to Sing-box tab")
            page.goto(f"{BASE}/")
            page.wait_for_load_state("networkidle")

            # get_by_role respects aria-hidden, so the measure-row duplicate is excluded.
            page.get_by_role("button", name="Sing-box").click()
            page.wait_for_timeout(500)

            print("[smoke-sub] verify SubscriptionActiveCard rendered")
            page.wait_for_selector("text=/Подписки — активные/", timeout=5000)
            page.wait_for_selector("text=/Provider Demo/", timeout=5000)

            print("[smoke-sub] open server picker on Provider Demo card")
            page.locator(".server-btn").first.click()
            page.wait_for_timeout(300)

            print("[smoke-sub] popover opens with member rows")
            popover = page.locator(".popover")
            popover.wait_for(state="visible", timeout=3000)

            rows = popover.locator(".row")
            total_rows = rows.count()
            if total_rows < 2:
                raise AssertionError(f"expected >= 2 member rows in popover, got {total_rows}")

            # Pick the second member (index 1; index 0 is the active one).
            rows.nth(1).click()

            print("[smoke-sub] popover closes after pick")
            popover.wait_for(state="hidden", timeout=5000)

            print("[smoke-sub] active member updated on card")
            page.wait_for_timeout(500)  # let store refetch settle

            print("[smoke-sub] OK")
        except Exception as e:
            failed = True
            print(f"[smoke-sub] FAIL: {e}", file=sys.stderr)
            try:
                page.screenshot(path="/tmp/smoke-subscription-fail.png", full_page=True)
            except Exception:
                pass
        browser.close()
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
