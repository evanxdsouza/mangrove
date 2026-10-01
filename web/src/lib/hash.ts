// A small, fast, deterministic string hash (djb2 variant) -- not
// cryptographic, just stable: the same input always produces the same
// number, which is all a "give this project its own look" feature needs.
export function hashString(s: string): number {
  let h = 5381;
  for (let i = 0; i < s.length; i++) {
    h = (h * 33) ^ s.charCodeAt(i);
  }
  return h >>> 0;
}
