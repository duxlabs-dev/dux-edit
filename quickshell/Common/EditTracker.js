.pragma library

// Native substring comparison beats a charCodeAt loop by orders of magnitude on multi-megabyte text, so
// the common ends are found by galloping over chunks and then halving into the first mismatch.
const FIRST_STEP = 1024;

function isHighSurrogate(code) {
    return code >= 0xD800 && code <= 0xDBFF;
}

function isLowSurrogate(code) {
    return code >= 0xDC00 && code <= 0xDFFF;
}

// Longest n <= limit for which same(n, size) holds over [n, n + size) at every step.
function gallop(limit, same) {
    let n = 0;
    let step = FIRST_STEP;
    let growing = true;
    while (n < limit) {
        const size = Math.min(step, limit - n);
        if (same(n, size)) {
            n += size;
            step = growing ? step * 2 : Math.max(1, size >> 1);
            continue;
        }
        if (size === 1)
            return n;
        growing = false;
        step = size >> 1;
    }
    return n;
}

// The single replacement that turns `before` into `after`: {start, removed, text}, or null when equal.
// Offsets are UTF-16 units and never split a surrogate pair.
function diff(before, after) {
    if (before === after)
        return null;
    const shortest = Math.min(before.length, after.length);
    // One copy per probe: startsWith and endsWith compare in place.
    let prefix = gallop(shortest, (n, size) => after.startsWith(before.substring(n, n + size), n));
    if (prefix > 0 && isHighSurrogate(before.charCodeAt(prefix - 1)))
        prefix--;
    const beforeEnd = before.length;
    const afterEnd = after.length;
    let suffix = gallop(shortest - prefix, (n, size) => after.endsWith(before.substring(beforeEnd - n - size, beforeEnd - n), afterEnd - n));
    if (suffix > 0 && isLowSurrogate(before.charCodeAt(beforeEnd - suffix)))
        suffix--;
    return {
        "start": prefix,
        "removed": beforeEnd - prefix - suffix,
        "text": after.slice(prefix, afterEnd - suffix)
    };
}
