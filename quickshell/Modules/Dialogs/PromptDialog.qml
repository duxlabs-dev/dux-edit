pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// Asks one question at a time from a queue of {title, message, actions: [{label, run, danger}], cancelLabel, cancel}.
// Enter runs the last action, Escape cancels.
DDialog {
    id: root

    property var queue: []
    readonly property var prompt: queue[0] ?? null
    readonly property var promptActions: prompt?.actions ?? []

    function ask(next) {
        queue = queue.concat([next]);
    }

    function finish(run) {
        const head = prompt;
        queue = queue.slice(1);
        if (run) {
            run();
            return;
        }
        head?.cancel?.();
    }

    embedded: false
    opened: prompt !== null
    title: prompt?.title ?? ""
    supportingText: prompt?.message ?? ""
    onOpenedChanged: {
        if (opened) {
            forceActiveFocus();
            return;
        }
        Documents.current?.editor?.focusEditor();
    }
    onAccepted: finish(promptActions[promptActions.length - 1]?.run ?? null)
    onRejected: finish(null)

    actions: [
        DButton {
            text: root.prompt?.cancelLabel || I18n.tr("Cancel", "dialog button to cancel")
            variant: "text"
            onClicked: root.finish(null)
        },
        Repeater {
            model: root.promptActions

            DButton {
                required property var modelData

                text: modelData.label
                variant: modelData.danger ? "tonal" : "filled"
                danger: modelData.danger === true
                onClicked: root.finish(modelData.run)
            }
        }
    ]
}
