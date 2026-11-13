.pragma library

// One in-flight request per pane; cursor/edit/view events replace the queued request.
function create(call, apply) {
    let latest = null;
    let busy = false;
    let serial = 0;
    let key = "";
    let active = true;

    function send() {
        if (!active || busy || !latest)
            return;
        const sent = latest;
        busy = true;
        call(sent.params, result => {
            busy = false;
            if (!active)
                return;
            if (sent.serial === serial && result.ok && result.version === sent.params.version)
                apply(result.pair);
            if (latest && latest.serial !== sent.serial)
                send();
        });
    }

    return {
        update(params, lexerState) {
            if (!active)
                return;
            const nextKey = params ? [params.docId, params.version, params.offset, params.first, params.last, lexerState].join(":") : "";
            if (nextKey === key)
                return;
            key = nextKey;
            serial++;
            latest = params ? {params, serial} : null;
            apply(null);
            send();
        },
        stop() {
            active = false;
            latest = null;
            call = null;
            apply = null;
        }
    };
}
