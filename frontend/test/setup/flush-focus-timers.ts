import { afterEach } from 'vitest'

// reka-ui's FocusScope schedules a `setTimeout(() => focus(document.body …), 0)`
// on unmount to restore focus (see reka-ui/src/FocusScope/FocusScope.vue). Under
// `@vitest-environment nuxt` that macrotask can fire AFTER the test file's
// environment — and its `document` — has been torn down, throwing an uncaught
// `ReferenceError: document is not defined`. Vitest reports that as an unhandled
// error and fails the whole run with exit 1 even though every test passed, and
// it attributes it to whichever file happened to be running when the stray timer
// fired (so the actual leaking spec is unknowable). It is a benign framework
// teardown race: the component is already gone.
//
// Fix: after every test, drain one macrotask round while `document` still exists.
// - `afterEach` hooks run LIFO, and this setup file is imported before any spec,
//   so this hook runs AFTER the spec's own `enableAutoUnmount(afterEach)` unmount
//   (which is what schedules the FocusScope timer). The FocusScope `setTimeout(0)`
//   is therefore already queued before the one below; the event loop runs it
//   first, while `document` is alive, so it can never outlive the environment.
// - Environment teardown is per-file (after all tests + hooks), so `document` is
//   guaranteed present during any `afterEach`, including the last test's.
// - It uses the REAL `setTimeout`, captured at module load before any spec can
//   install fake timers, so a fake-timer spec cannot hang this wait. A FocusScope
//   timer left on a fake queue never leaks: fake timers are discarded (not run on
//   the real loop) when the spec restores real timers or the file environment ends.
const realSetTimeout: typeof setTimeout = globalThis.setTimeout.bind(globalThis)

afterEach(async () => {
  await new Promise<void>(resolve => realSetTimeout(resolve, 0))
})
