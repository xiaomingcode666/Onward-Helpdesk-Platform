import { expect, test } from "@playwright/test"

test.use({ video: "off" })

for (const interruptTransitions of [false, true]) {
  test(`homepage carousel keeps looping ${interruptTransitions ? "without transition events" : "normally"}`, async ({ page }) => {
    const errors: string[] = []
    page.on("pageerror", (error) => errors.push(error.message))
    await page.clock.install()
    await page.goto("/", { waitUntil: "networkidle" })
    await page.clock.pauseAt(await page.evaluate(() => Date.now() + 100))

    const viewport = page.getByLabel("产品界面自动轮播")
    const track = viewport.locator(":scope > div")
    const counter = page.getByText(/^LIVE · /)
    await expect(counter).toHaveText("LIVE · 01 / 05")

    if (interruptTransitions) {
      await viewport.evaluate((element) => {
        element.addEventListener("transitionend", (event) => event.stopImmediatePropagation(), true)
      })
      await page.addStyleTag({ content: '[aria-label="产品界面自动轮播"] > div { transition: none !important; }' })
    }

    const seen = new Set<string>()
    let loopResets = 0
    for (let step = 0; step < 12; step += 1) {
      await page.clock.runFor(step === 0 ? 4200 : 3350)
      // Give React a render before advancing the fallback timeout.
      await expect(track).not.toHaveCSS("transform", "none")
      await page.clock.runFor(850)
      await expect(counter).toHaveText(/^LIVE · 0[1-5] \/ 05$/)
      const caption = (await counter.textContent()) ?? ""
      seen.add(caption)
      if (caption === "LIVE · 01 / 05") {
        loopResets += 1
        await expect(track).toHaveAttribute("style", /translate3d\(0%,/)
      }
      const offset = await track.evaluate((element) => {
        const transform = (element as HTMLElement).style.transform
        return Number(transform.match(/translate3d\((-?[\d.]+)%/)?.[1])
      })
      expect(offset).toBeGreaterThanOrEqual(-500)
      expect(offset).toBeLessThanOrEqual(0)
    }

    expect(seen.size).toBe(5)
    expect(loopResets).toBeGreaterThanOrEqual(2)
    expect(errors).toEqual([])
  })
}
