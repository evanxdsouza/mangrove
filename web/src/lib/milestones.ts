// Rare, real milestone moments -- kept genuinely rare via a localStorage
// flag so a milestone toast fires once per browser, ever, not on every
// deploy. Deliberately narrow: only claims covered by state this client can
// actually observe (a deploy or rollback that just succeeded) -- no
// fabricated "30-day uptime streak" here, since accurately tracking
// continuous uptime needs server-side history this client doesn't have.

const PREFIX = "mangrove-milestone-";

/** Returns true the first time this key is checked, ever, on this browser;
    false every time after. Marks itself done as a side effect of the first
    (true) check, so callers never need a separate "mark done" step. */
export function firstTime(key: string): boolean {
  try {
    const storageKey = PREFIX + key;
    if (window.localStorage.getItem(storageKey)) return false;
    window.localStorage.setItem(storageKey, "1");
    return true;
  } catch {
    // localStorage unavailable (private browsing etc.) -- treat every
    // check as "not first" so a milestone never re-fires every render.
    return false;
  }
}
