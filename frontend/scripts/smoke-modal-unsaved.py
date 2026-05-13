#!/usr/bin/env python3
"""Smoke test for the Modal.hasUnsavedChanges flow.

Run with mock-proxy already up (``npm run dev:mock:proxy`` in another terminal):
    python3 frontend/scripts/smoke-modal-unsaved.py

For each SCENARIO, opens the modal, types into a designated field, then
verifies:
  - clicking the backdrop pops the ConfirmModal (original modal stays)
  - clicking "Продолжить редактирование" closes the confirm, original stays
  - clicking backdrop again + "Закрыть без сохранения" closes both modals
  - reopening + immediate backdrop click (clean form) closes silently
"""

import sys
import time
from dataclasses import dataclass
from typing import Callable

from playwright.sync_api import sync_playwright, Page, TimeoutError as PWTimeout

BASE = "http://localhost:5173"


@dataclass
class Scenario:
    name: str
    navigate: Callable[[Page], None]
    trigger: Callable[[Page], None]
    input_selector: str
    typed_value: str = "x"


def _open_edit_managed_server(p: Page):
    """Open EditManagedServerModal from /servers via the cog icon on a server card."""
    # The server detail card shows a settings cog icon button with aria-label="Настройки"
    loc = p.locator('button[aria-label="Настройки"]').first
    if loc.count() > 0:
        loc.click(timeout=4000)
        return
    raise RuntimeError("could not find cog icon to open EditManagedServerModal")


SCENARIOS: list[Scenario] = [
    Scenario(
        name="EditManagedServerModal",
        navigate=lambda p: goto(p, "/servers"),
        trigger=_open_edit_managed_server,
        # First text input in the open modal — usually the name (description) field
        input_selector='.modal-card input[type="text"]',
    ),
]

# Extend with three new modal scenarios
SCENARIOS.extend([
    Scenario(
        name="CreateManagedServerModal",
        navigate=lambda p: goto(p, "/servers"),
        trigger=lambda p: p.locator('button:has-text("Новый сервер")').first.click(timeout=4000),
        input_selector='.modal-card input[type="text"]',
    ),
    Scenario(
        name="AddManagedPeerModal",
        navigate=lambda p: goto(p, "/servers"),
        trigger=lambda p: p.locator('button:has-text("Добавить")').last.click(timeout=4000),
        input_selector='.modal-card input[type="text"]',
    ),
    Scenario(
        name="EditManagedPeerModal",
        navigate=lambda p: goto(p, "/servers"),
        trigger=lambda p: p.locator('.peer-row').first.locator('.peer-action-btn').nth(1).click(timeout=4000),
        input_selector='#emp-desc',
    ),
])


def goto(page: Page, path: str):
    page.goto(f"{BASE}{path}")
    page.wait_for_load_state("domcontentloaded")
    page.wait_for_timeout(1500)


def click_tab(page: Page, label: str):
    page.locator(f'.overflow-tabs .tab:has-text("{label}")').first.click(
        force=True, timeout=4000
    )
    page.wait_for_timeout(500)


def assert_modal_open(page: Page):
    page.locator(".modal-card").first.wait_for(state="visible", timeout=4000)


def assert_confirm_open(page: Page):
    page.locator('.modal-card:has-text("Несохранённые изменения")').wait_for(
        state="visible", timeout=3000
    )


def click_backdrop_corner(page: Page):
    page.mouse.move(10, 10)
    page.mouse.down()
    page.mouse.up()
    page.wait_for_timeout(400)


def run_scenario(page: Page, sc: Scenario) -> str:
    """Returns 'PASS' or a short error string."""
    try:
        sc.navigate(page)
        sc.trigger(page)
        assert_modal_open(page)

        # Type something to mark the form dirty.
        page.locator(sc.input_selector).first.fill(sc.typed_value, timeout=3000)
        page.wait_for_timeout(200)

        # Backdrop while dirty -> ConfirmModal appears.
        click_backdrop_corner(page)
        assert_confirm_open(page)

        # Cancel from confirm -> confirm closes, parent stays.
        page.locator('button:has-text("Продолжить редактирование")').first.click(timeout=2000)
        try:
            page.locator('.modal-card:has-text("Несохранённые изменения")').first.wait_for(state="detached", timeout=2000)
        except PWTimeout:
            return "confirm did not close on cancel"
        assert_modal_open(page)

        # Backdrop again + confirm-destroy.
        click_backdrop_corner(page)
        assert_confirm_open(page)
        page.locator('button:has-text("Закрыть без сохранения")').first.click(timeout=2000)
        try:
            page.locator('.modal-card').first.wait_for(state="detached", timeout=2000)
        except PWTimeout:
            return "parent did not close on destroy"

        # Reopen clean -> backdrop closes silently.
        sc.trigger(page)
        assert_modal_open(page)
        click_backdrop_corner(page)
        try:
            page.locator('.modal-card').first.wait_for(state="detached", timeout=2000)
        except PWTimeout:
            return "clean form did not close silently"

        return "PASS"
    except PWTimeout as e:
        return f"timeout: {str(e)[:120]}"
    except Exception as e:
        return f"{type(e).__name__}: {str(e)[:120]}"


def main():
    if not SCENARIOS:
        print("No SCENARIOS defined yet; nothing to run.")
        return 0
    with sync_playwright() as pw:
        browser = pw.chromium.launch(headless=True)
        ctx = browser.new_context(viewport={"width": 1600, "height": 1000})
        page = ctx.new_page()
        fails = 0
        for sc in SCENARIOS:
            res = run_scenario(page, sc)
            mark = "PASS" if res == "PASS" else "FAIL"
            if res != "PASS":
                fails += 1
            print(f"{mark}  {sc.name:40s} {res if res != 'PASS' else ''}")
        browser.close()
        return 0 if fails == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
