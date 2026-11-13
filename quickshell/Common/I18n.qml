pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Io
import qs.Services as App

// Catalog loader ported from Dux Files: poexports/<locale>.json for the app, DCommon's own catalogs after it.
Singleton {
    id: root

    readonly property var log: App.Log.scoped("I18n")

    property string _resolvedLocale: "en"

    readonly property string _rawLocale: SettingsData.language === "" ? Qt.locale().name : SettingsData.language
    readonly property string _lang: _rawLocale.split(/[_-]/)[0]
    readonly property var _candidates: {
        const fullUnderscore = _rawLocale;
        const fullHyphen = _rawLocale.replace("_", "-");
        return [fullUnderscore, fullHyphen, _lang].filter(c => c && c !== "en");
    }

    readonly property var _rtlLanguages: ["ar", "he", "iw", "fa", "ur", "ps", "sd", "dv", "yi", "ku"]
    readonly property bool isRtl: _rtlLanguages.includes(_lang)

    readonly property url translationsFolder: Qt.resolvedUrl("../translations/poexports")
    readonly property url commonTranslationsFolder: Qt.resolvedUrl("../DCommon/translations/poexports")

    property var presentLocales: ({
            "en": Qt.locale("en")
        })
    property var translations: ({})
    property bool translationsLoaded: false
    property var commonTranslations: ({})
    property bool commonTranslationsLoaded: false

    property url _selectedPath: ""
    property url _commonSelectedPath: ""
    property bool _listed: false
    property bool _commonListed: false
    property var _commonPresent: ({})

    // Bindings call tr() on every keystroke and a miss scans every context, so answers are memoized.
    readonly property var _memo: ({
            "translations": null,
            "common": null,
            "terms": new Map()
        })

    Connections {
        target: SettingsData
        function onLanguageChanged() {
            if (root._candidates.length === 0) {
                root._resolvedLocale = "en";
                root._fallbackToEnglish();
                root._pickCommonTranslation();
                return;
            }
            if (root._listed)
                root._pickTranslation();
            if (root._commonListed)
                root._pickCommonTranslation();
        }
    }

    // Listed once, only for a non-English locale: a FolderListModel keeps a directory watch while it has a
    // folder. The folder is cleared after this handler returns, not from inside the model's own signal.
    FolderListModel {
        id: dir
        folder: root._candidates.length > 0 && !root._listed ? root.translationsFolder : ""
        nameFilters: ["*.json"]
        showDirs: false
        showDotAndDotDot: false

        onStatusChanged: {
            if (status !== FolderListModel.Ready || root._listed || root._candidates.length === 0)
                return;
            root._loadPresentLocales();
            root._pickTranslation();
            Qt.callLater(() => root._listed = true);
        }
    }

    FolderListModel {
        id: commonDir
        folder: root._candidates.length > 0 && !root._commonListed ? root.commonTranslationsFolder : ""
        nameFilters: ["*.json"]
        showDirs: false
        showDotAndDotDot: false

        onStatusChanged: {
            if (status !== FolderListModel.Ready || root._commonListed || root._candidates.length === 0)
                return;
            const present = {};
            for (let i = 0; i < commonDir.count; i++) {
                const name = commonDir.get(i, "fileName");
                if (name && name.endsWith(".json"))
                    present[name.slice(0, -5)] = true;
            }
            root._commonPresent = present;
            root._pickCommonTranslation();
            Qt.callLater(() => root._commonListed = true);
        }
    }

    FileView {
        id: translationLoader
        path: root._selectedPath

        onLoaded: {
            try {
                root.translations = JSON.parse(text());
                root.translationsLoaded = true;
                root.log.info(`Loaded translations for '${root._resolvedLocale}' (${Object.keys(root.translations).length} contexts)`);
            } catch (e) {
                root.log.warn(`Error parsing '${root._resolvedLocale}':`, e, "- falling back to English");
                root._fallbackToEnglish();
            }
        }

        onLoadFailed: error => {
            root.log.warn(`Failed to load '${root._resolvedLocale}' (${error}), falling back to English`);
            root._fallbackToEnglish();
        }
    }

    FileView {
        id: commonTranslationLoader
        path: root._commonSelectedPath
        printErrors: false

        onLoaded: {
            try {
                root.commonTranslations = JSON.parse(text());
                root.commonTranslationsLoaded = true;
                root.log.info(`Loaded DCommon translations (${Object.keys(root.commonTranslations).length} contexts)`);
            } catch (e) {
                root.log.warn("Error parsing DCommon translations:", e);
            }
        }
    }

    function _loadPresentLocales() {
        if (Object.keys(presentLocales).length > 1)
            return;
        const found = {
            "en": Qt.locale("en")
        };
        for (let i = 0; i < dir.count; i++) {
            const name = dir.get(i, "fileName");
            if (!name || !name.endsWith(".json"))
                continue;
            const shortName = name.slice(0, -5);
            found[shortName] = Qt.locale(shortName);
        }
        presentLocales = found;
    }

    function _pickTranslation() {
        for (let i = 0; i < _candidates.length; i++) {
            const cand = _candidates[i];
            if (presentLocales[cand] === undefined)
                continue;
            if (cand === _resolvedLocale)
                return;
            useLocale(cand, cand.startsWith("en") ? "" : translationsFolder + "/" + cand + ".json");
            return;
        }

        _resolvedLocale = "en";
        _fallbackToEnglish();
    }

    function useLocale(localeTag, fileUrl) {
        _resolvedLocale = localeTag || "en";
        _selectedPath = fileUrl;
        translationsLoaded = false;
        translations = ({});
        log.info(`Using locale '${localeTag}' from ${fileUrl}`);
    }

    function _fallbackToEnglish() {
        _selectedPath = "";
        translationsLoaded = false;
        translations = ({});
    }

    function _pickCommonTranslation() {
        const present = _commonPresent;
        for (let i = 0; i < _candidates.length; i++) {
            if (!present[_candidates[i]])
                continue;
            const path = commonTranslationsFolder + "/" + _candidates[i] + ".json";
            if (path === _commonSelectedPath)
                return;
            commonTranslationsLoaded = false;
            commonTranslations = ({});
            _commonSelectedPath = path;
            return;
        }
        commonTranslationsLoaded = false;
        commonTranslations = ({});
        _commonSelectedPath = "";
    }

    function _lookup(table, term, context) {
        if (!table)
            return "";
        const ctx = context || term;
        if (table[ctx] && table[ctx][term])
            return table[ctx][term];
        for (const c in table) {
            if (table[c] && table[c][term])
                return table[c][term];
        }
        return "";
    }

    function tr(term, context) {
        // Read before the memo, so calling bindings still follow the catalogs.
        const table = translationsLoaded ? translations : null;
        const common = commonTranslationsLoaded ? commonTranslations : null;
        if (!table && !common)
            return term;
        const memo = _memo;
        if (memo.translations !== table || memo.common !== common) {
            memo.translations = table;
            memo.common = common;
            memo.terms.clear();
        }
        const key = (context || "") + "\u0004" + term;
        let hit = memo.terms.get(key);
        if (hit === undefined) {
            hit = _lookup(table, term, context) || _lookup(common, term, context) || term;
            memo.terms.set(key, hit);
        }
        return hit;
    }
}
