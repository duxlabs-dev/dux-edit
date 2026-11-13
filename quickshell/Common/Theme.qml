pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.DCommon.Common
import qs.Services as App
import "../DCommon/Common/StockTheme.js" as StockTheme
import "../DCommon/Common/Surface.js" as Surface
import "../DCommon/Common/Contrast.js" as Contrast
import "../DCommon/Common/Accents.js" as Accents
import "../DCommon/Common/Tonal.js" as Tonal

// Ported from Dux Files. Only feeds DC.Style; app code reads Style.
Singleton {
    id: root

    readonly property string defaultFontFamily: Fonts.sans
    readonly property var log: App.Log.scoped("Theme")

    // The shell clears this while no window exists so the DMS cache files are not watched at idle.
    property bool watching: true

    readonly property bool isLightMode: {
        switch (SettingsData.themeMode) {
        case "light":
            return true;
        case "dark":
            return false;
        default:
            return App.PortalService.prefersLight;
        }
    }

    // Inside Flatpak XDG_CACHE_HOME is the sandbox cache; DMS writes to the host one.
    readonly property string xdgCacheDir: {
        const xdg = Quickshell.env(Quickshell.env("FLATPAK_ID") ? "HOST_XDG_CACHE_HOME" : "XDG_CACHE_HOME");
        return xdg ? xdg : AppPaths.home + "/.cache";
    }

    property bool useLegacyDmsCache: false
    readonly property string dmsColorsPath: xdgCacheDir + (useLegacyDmsCache ? "/DankMaterialShell" : "/dms") + "/dms-colors.json"
    readonly property string dmsTypographyPath: xdgCacheDir + "/dms/dms-typography.json"

    property var matugenColors: ({})
    property bool colorsLoaded: false

    function getMatugenColor(path, fallback) {
        let cur = matugenColors?.colors?.[isLightMode ? "light" : "dark"];
        if (!cur)
            return fallback;
        for (const part of path.split(".")) {
            if (!cur || typeof cur !== "object" || !(part in cur))
                return fallback;
            cur = cur[part];
        }
        return cur || fallback;
    }

    function buildDmsTheme(fallback) {
        return {
            "primary": getMatugenColor("primary", fallback.primary),
            "primaryText": getMatugenColor("on_primary", fallback.primaryText),
            "primaryContainer": getMatugenColor("primary_container", fallback.primaryContainer),
            "onPrimaryContainer": getMatugenColor("on_primary_container", fallback.onPrimaryContainer),
            "secondary": getMatugenColor("secondary", fallback.secondary),
            "secondaryContainer": getMatugenColor("secondary_container", fallback.secondaryContainer),
            "onSecondaryContainer": getMatugenColor("on_secondary_container", fallback.onSecondaryContainer),
            "tertiary": getMatugenColor("tertiary", fallback.tertiary),
            "tertiaryContainer": getMatugenColor("tertiary_container", fallback.tertiaryContainer),
            "onTertiaryContainer": getMatugenColor("on_tertiary_container", fallback.onTertiaryContainer),
            "surface": getMatugenColor("surface", fallback.surface),
            "surfaceText": getMatugenColor("on_surface", fallback.surfaceText),
            "surfaceVariant": getMatugenColor("surface_variant", fallback.surfaceVariant),
            "surfaceVariantText": getMatugenColor("on_surface_variant", fallback.surfaceVariantText),
            "surfaceTint": getMatugenColor("surface_tint", fallback.surfaceTint),
            "surfaceBright": getMatugenColor("surface_bright", fallback.surfaceBright),
            "surfaceDim": getMatugenColor("surface_dim", fallback.surfaceDim),
            "background": getMatugenColor("background", fallback.background),
            "backgroundText": getMatugenColor("on_background", fallback.backgroundText),
            "outline": getMatugenColor("outline", fallback.outline),
            "outlineVariant": getMatugenColor("outline_variant", fallback.outlineVariant),
            "surfaceContainerLowest": getMatugenColor("surface_container_lowest", fallback.surfaceContainerLowest),
            "surfaceContainerLow": getMatugenColor("surface_container_low", fallback.surfaceContainerLow),
            "surfaceContainer": getMatugenColor("surface_container", fallback.surfaceContainer),
            "surfaceContainerHigh": getMatugenColor("surface_container_high", fallback.surfaceContainerHigh),
            "surfaceContainerHighest": getMatugenColor("surface_container_highest", fallback.surfaceContainerHighest),
            "inverseSurface": getMatugenColor("inverse_surface", fallback.inverseSurface),
            "inverseOnSurface": getMatugenColor("inverse_on_surface", fallback.inverseOnSurface),
            "scrim": getMatugenColor("scrim", fallback.scrim),
            "error": getMatugenColor("error", fallback.error),
            "errorText": getMatugenColor("on_error", fallback.errorText),
            "errorContainer": getMatugenColor("error_container", fallback.errorContainer),
            "errorContainerText": getMatugenColor("on_error_container", fallback.errorContainerText),
            "warning": fallback.warning,
            "info": fallback.info,
            "success": fallback.success
        };
    }

    readonly property var currentThemeData: {
        const preset = StockTheme.getPreset(SettingsData.presetTheme, isLightMode);
        if (SettingsData.colorSource === "preset" || !colorsLoaded)
            return preset;
        return buildDmsTheme(preset);
    }

    property color primary: currentThemeData.primary
    property color primaryText: currentThemeData.primaryText
    readonly property real containerTint: currentThemeData.containerTint ?? Tonal.defaultTint(surfaceContainer)
    property color primaryContainer: currentThemeData.softPrimaryContainer || Tonal.softContainer(primary, surfaceContainer, containerTint)
    property color secondary: currentThemeData.secondary
    property color secondaryContainer: currentThemeData.secondaryContainer || blend(surfaceContainerHigh, secondary, 0.35)
    property color tertiary: currentThemeData.tertiary || currentThemeData.secondary
    property color tertiaryContainer: currentThemeData.tertiaryContainer || blend(surfaceContainerHigh, tertiary, 0.35)
    readonly property real selectedContainerTint: currentThemeData.selectedContainerTint ?? 0.2
    // DMS exports surfaceContainerHigh as secondary_container when the theme sets none
    readonly property bool themedSelectedContainer: !!currentThemeData.secondaryContainer && !Qt.colorEqual(secondaryContainer, surfaceContainerHigh) && Contrast.isTonal(secondaryContainer, onSecondaryContainer) && Contrast.isTonal(secondaryContainer, surfaceText)
    readonly property color selectedContainer: currentThemeData.selectedContainer || (themedSelectedContainer ? secondaryContainer : Contrast.subtleTint(surfaceContainerHigh, primary, surfaceText, selectedContainerTint))
    readonly property color accentOnSelectedContainer: currentThemeData.accentOnSelectedContainer || (Contrast.ratio(primary, selectedContainer) >= 3 ? primary : onSelectedContainer)
    readonly property color accentOnPrimaryContainer: currentThemeData.accentOnPrimaryContainer || (Contrast.ratio(primary, primaryContainer) >= 3 ? primary : onPrimaryContainer)
    readonly property var accents: Accents.derive(primary, isLightMode, currentThemeData.accents ?? null)
    property color surface: currentThemeData.surface
    property color surfaceText: currentThemeData.surfaceText
    property color surfaceVariant: currentThemeData.surfaceVariant
    property color surfaceVariantText: currentThemeData.surfaceVariantText
    property color surfaceTint: currentThemeData.surfaceTint
    property color surfaceBright: currentThemeData.surfaceBright || (isLightMode ? surface : surfaceContainerHighest)
    property color surfaceDim: currentThemeData.surfaceDim || (isLightMode ? surfaceContainer : background)
    property color background: currentThemeData.background
    property color outline: currentThemeData.outline
    property color outlineVariant: currentThemeData.outlineVariant || withAlpha(outline, 0.6)
    property color surfaceContainerLowest: currentThemeData.surfaceContainerLowest || blend(surfaceContainer, surface, 1.2)
    property color surfaceContainerLow: currentThemeData.surfaceContainerLow || blend(surface, surfaceContainer, 0.667)
    property color surfaceContainer: currentThemeData.surfaceContainer
    property color surfaceContainerHigh: currentThemeData.surfaceContainerHigh
    property color surfaceContainerHighest: currentThemeData.surfaceContainerHighest || surfaceContainerHigh
    readonly property color tooltipSurface: inverseSurface
    readonly property color tooltipText: Contrast.readableOn(tooltipSurface, [inverseOnSurface].concat(onContainerCandidates))
    property color inverseSurface: currentThemeData.inverseSurface || surfaceText
    property color inverseOnSurface: currentThemeData.inverseOnSurface || surface
    readonly property color contrastDark: "#000000"
    readonly property color contrastLight: "#ffffff"

    property color onSurface
    property color onSurfaceVariant
    property color onPrimaryContainer
    property color onSecondaryContainer
    property color onTertiaryContainer
    property color onSelectedContainer
    property color onError
    property color onErrorContainer
    property color onSurfaceVariant_40: withAlpha(onSurfaceVariant, 0.4)
    readonly property list<QtObject> roleBindings: [
        Binding {
            target: root
            property: "onSurface"
            value: root.surfaceText
        },
        Binding {
            target: root
            property: "onSurfaceVariant"
            value: root.surfaceVariantText
        },
        Binding {
            target: root
            property: "onPrimaryContainer"
            value: {
                const explicit = root.currentThemeData.onPrimaryContainer;
                if (!explicit)
                    return Contrast.readableOn(root.primaryContainer, root.onContainerCandidates);
                if (root.currentThemeData.softPrimaryContainer)
                    return explicit;
                return Contrast.readableOn(root.primaryContainer, [Qt.color(explicit)].concat(root.onContainerCandidates));
            }
        },
        Binding {
            target: root
            property: "onSecondaryContainer"
            value: root.currentThemeData.onSecondaryContainer || Contrast.readableOn(root.secondaryContainer, root.onContainerCandidates)
        },
        Binding {
            target: root
            property: "onTertiaryContainer"
            value: root.currentThemeData.onTertiaryContainer || Contrast.readableOn(root.tertiaryContainer, root.onContainerCandidates)
        },
        Binding {
            target: root
            property: "onError"
            value: root.currentThemeData.errorText || root.surface
        },
        Binding {
            target: root
            property: "onErrorContainer"
            value: root.currentThemeData.errorContainerText || root.onSurface
        },
        Binding {
            target: root
            property: "onSelectedContainer"
            value: root.currentThemeData.onSelectedContainer || (root.currentThemeData.selectedContainer ? Contrast.readableOn(root.selectedContainer, root.onContainerCandidates) : root.themedSelectedContainer ? root.onSecondaryContainer : root.surfaceText)
        }
    ]
    readonly property var onContainerCandidates: [surfaceText, surface, contrastLight, contrastDark]

    property color error: currentThemeData.error
    property color errorContainer: currentThemeData.errorContainer || surfaceContainerHigh
    property color warning: currentThemeData.warning

    readonly property real layerOutlineOpacity: Math.max(0, Math.min(1, SettingsData.blurLayerOutlineOpacity))

    readonly property FontSource typography: FontSource {
        source: "auto"
        dmsPath: root.watching ? root.dmsTypographyPath : ""
    }
    readonly property real fontScale: typography.fontScale
    property real fontSizeSmall: Math.round(fontScale * 12)
    property real fontSizeMedium: Math.round(fontScale * 14)
    property real fontSizeLarge: Math.round(fontScale * 16)
    property real fontSizeXLarge: Math.round(fontScale * 20)
    property real fontSizeXXLarge: Math.round(fontScale * 28)
    property real fontSizeDisplay: Math.round(fontScale * 36)
    property real fontSizeDisplayLarge: Math.round(fontScale * 57)

    readonly property real radiusStrength: 50

    readonly property real sliderHandleHeightS: 44
    readonly property real sliderHandleHeightM: 44
    readonly property real focusRingWidth: 1.5
    readonly property real focusRingOffset: 3
    readonly property color scrimColor: currentThemeData.scrim || "#000000"
    readonly property color backdropColor: surface.hslLightness >= 0.5 ? surfaceDim : scrimColor
    readonly property real tooltipMaxWidth: 500
    readonly property real clockFaceSize: 250
    readonly property real clockOuterRingRatio: 0.34
    readonly property real clockInnerRingRatio: 0.2
    readonly property real clockHandleSize: 40

    readonly property string fontFamily: typography.fontFamily
    readonly property string monoFontFamily: typography.monoFontFamily
    readonly property string displayFontFamily: typography.displayFontFamily
    readonly property int fontWeight: typography.fontWeight

    readonly property real floatingWindowTransparency: 1
    readonly property bool floatingWindowForegroundLayers: true
    readonly property real floatingWindowForegroundTransparency: 1
    readonly property real floatingWindowForegroundAlpha: Surface.foregroundAlpha(floatingWindowForegroundLayers, floatingWindowForegroundTransparency)
    readonly property real popupTransparency: floatingWindowTransparency
    readonly property real foregroundAlpha: floatingWindowForegroundAlpha
    readonly property bool blurLayersActive: false

    readonly property color floatingSurface: withAlpha(surfaceContainer, popupTransparency)
    readonly property color nestedSurface: withAlpha(surfaceContainerHigh, foregroundAlpha)
    readonly property color floatingWindowSurface: withAlpha(surfaceContainer, floatingWindowTransparency)
    readonly property color floatingWindowNestedSurface: withAlpha(surfaceContainerHigh, floatingWindowForegroundAlpha)
    readonly property color floatingWindowFieldColor: floatingWindowNestedSurface
    readonly property color popupFieldColor: nestedSurface

    property color widgetBaseHoverColor: {
        const blended = blend(surfaceContainerHigh, primary, 0.1);
        return withAlpha(blended, Math.max(0.3, blended.a));
    }

    readonly property int currentAnimationBaseDuration: 500
    readonly property int currentAnimationSpeed: Style.AnimationSpeed.Custom
    readonly property int shorterDuration: Math.round(currentAnimationBaseDuration * 0.2)
    readonly property int shortDuration: Math.round(currentAnimationBaseDuration * 0.3)
    readonly property int mediumDuration: Math.round(currentAnimationBaseDuration * 0.6)

    readonly property real stateLayerFocus: 0.12
    readonly property real stateLayerPressed: 0.12

    function withAlpha(c, a) {
        return Style.withAlpha(c, a);
    }

    function blend(c1, c2, r) {
        return Qt.rgba(c1.r * (1 - r) + c2.r * r, c1.g * (1 - r) + c2.g * r, c1.b * (1 - r) + c2.b * r, c1.a * (1 - r) + c2.a * r);
    }

    FileView {
        id: dmsColorsView

        path: root.watching && SettingsData.colorSource === "auto" ? root.dmsColorsPath : ""
        blockLoading: false
        watchChanges: true
        printErrors: false

        onLoaded: {
            try {
                const text = dmsColorsView.text();
                if (!text)
                    return;
                root.matugenColors = JSON.parse(text);
                root.colorsLoaded = true;
            } catch (e) {
                root.log.warn(`ignoring unreadable dms colors: ${e}`);
                root.colorsLoaded = false;
            }
        }
        onFileChanged: dmsColorsView.reload()
        onLoadFailed: {
            if (dmsColorsView.path === "")
                return;
            if (!root.useLegacyDmsCache) {
                root.useLegacyDmsCache = true;
                return;
            }
            root.colorsLoaded = false;
        }
    }
}
