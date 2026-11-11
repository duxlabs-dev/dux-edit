pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// One text field: Go to Line, renaming a draft, a syntax color.
DDialog {
    id: root

    property string acceptLabel: I18n.tr("OK", "dialog button that confirms")
    property alias text: field.text
    property alias placeholderText: field.placeholderText
    property var submitHandler: null

    // submit gets the field's text when the user confirms.
    function show(dialogTitle, value, submit) {
        title = dialogTitle;
        field.text = value;
        submitHandler = submit;
        opened = true;
        field.forceActiveFocus();
        field.selectAll();
    }

    function submit() {
        const submitted = submitHandler;
        opened = false;
        submitHandler = null;
        submitted?.(field.text);
    }

    embedded: false
    opened: false
    acceptEnabled: field.text.trim() !== ""
    onAccepted: submit()
    onRejected: opened = false
    onOpenedChanged: {
        if (!opened)
            Documents.current?.editor?.focusEditor();
    }

    DTextField {
        id: field

        width: parent.width
        onAccepted: {
            if (root.acceptEnabled)
                root.submit();
        }
    }

    actions: [
        DButton {
            text: I18n.tr("Cancel", "dialog button to cancel")
            variant: "text"
            onClicked: root.opened = false
        },
        DButton {
            text: root.acceptLabel
            enabled: root.acceptEnabled
            onClicked: root.submit()
        }
    ]
}
