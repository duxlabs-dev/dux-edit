pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

// Find and replace run in the core against the document mirror. Stub with the final API (ui-editor package).
Singleton {
    id: root

    // options: {regex, caseSensitive, wholeWord}
    function params(doc, query, options) {
        return Object.assign({
            "docId": doc.docId,
            "version": doc.version,
            "query": query
        }, options || {});
    }

    // callback({ok, ranges: [start, length, ...], total, capped}) or ({ok: false, code: "BADREGEX", message})
    function find(doc, query, options, callback) {
        Backend.call("search.find", params(doc, query, options), callback);
    }

    // callback({ok, start, end, text, count}): apply it to the editor as one edit.
    function replaceAll(doc, query, options, replacement, callback) {
        Backend.call("search.replaceAll", Object.assign(params(doc, query, options), {
            "replacement": replacement
        }), callback);
    }

    // callback({ok, text}): the text that replaces the match at [start, start + length).
    function expand(doc, query, options, replacement, start, length, callback) {
        Backend.call("search.expand", Object.assign(params(doc, query, options), {
            "replacement": replacement,
            "start": start,
            "length": length
        }), callback);
    }
}
