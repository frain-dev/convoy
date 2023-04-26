/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { activeContrastBorder, badgeBackground, badgeForeground, contrastBorder, inputActiveOptionBackground, inputActiveOptionBorder, inputActiveOptionForeground, inputBackground, inputBorder, inputForeground, inputValidationErrorBackground, inputValidationErrorBorder, inputValidationErrorForeground, inputValidationInfoBackground, inputValidationInfoBorder, inputValidationInfoForeground, inputValidationWarningBackground, inputValidationWarningBorder, inputValidationWarningForeground, listActiveSelectionBackground, listActiveSelectionForeground, listActiveSelectionIconForeground, listDropBackground, listFilterWidgetBackground, listFilterWidgetNoMatchesOutline, listFilterWidgetOutline, listFocusBackground, listFocusForeground, listFocusOutline, listHoverBackground, listHoverForeground, listInactiveFocusBackground, listInactiveFocusOutline, listInactiveSelectionBackground, listInactiveSelectionForeground, listInactiveSelectionIconForeground, menuBackground, menuBorder, menuForeground, menuSelectionBackground, menuSelectionBorder, menuSelectionForeground, menuSeparatorBackground, resolveColorValue, scrollbarShadow, scrollbarSliderActiveBackground, scrollbarSliderBackground, scrollbarSliderHoverBackground, tableColumnsBorder, tableOddRowsBackgroundColor, treeIndentGuidesStroke, widgetShadow, listFocusAndSelectionOutline, listFilterWidgetShadow } from './colorRegistry.js';
export function computeStyles(theme, styleMap) {
    const styles = Object.create(null);
    for (const key in styleMap) {
        const value = styleMap[key];
        if (value) {
            styles[key] = resolveColorValue(value, theme);
        }
    }
    return styles;
}
export function attachStyler(themeService, styleMap, widgetOrCallback) {
    function applyStyles() {
        const styles = computeStyles(themeService.getColorTheme(), styleMap);
        if (typeof widgetOrCallback === 'function') {
            widgetOrCallback(styles);
        }
        else {
            widgetOrCallback.style(styles);
        }
    }
    applyStyles();
    return themeService.onDidColorThemeChange(applyStyles);
}
export function attachBadgeStyler(widget, themeService, style) {
    return attachStyler(themeService, {
        badgeBackground: (style === null || style === void 0 ? void 0 : style.badgeBackground) || badgeBackground,
        badgeForeground: (style === null || style === void 0 ? void 0 : style.badgeForeground) || badgeForeground,
        badgeBorder: contrastBorder
    }, widget);
}
export function attachListStyler(widget, themeService, overrides) {
    return attachStyler(themeService, Object.assign(Object.assign({}, defaultListStyles), (overrides || {})), widget);
}
export const defaultListStyles = {
    listFocusBackground,
    listFocusForeground,
    listFocusOutline,
    listActiveSelectionBackground,
    listActiveSelectionForeground,
    listActiveSelectionIconForeground,
    listFocusAndSelectionOutline,
    listFocusAndSelectionBackground: listActiveSelectionBackground,
    listFocusAndSelectionForeground: listActiveSelectionForeground,
    listInactiveSelectionBackground,
    listInactiveSelectionIconForeground,
    listInactiveSelectionForeground,
    listInactiveFocusBackground,
    listInactiveFocusOutline,
    listHoverBackground,
    listHoverForeground,
    listDropBackground,
    listSelectionOutline: activeContrastBorder,
    listHoverOutline: activeContrastBorder,
    listFilterWidgetBackground,
    listFilterWidgetOutline,
    listFilterWidgetNoMatchesOutline,
    listFilterWidgetShadow,
    treeIndentGuidesStroke,
    tableColumnsBorder,
    tableOddRowsBackgroundColor,
    inputActiveOptionBorder,
    inputActiveOptionForeground,
    inputActiveOptionBackground,
    inputBackground,
    inputForeground,
    inputBorder,
    inputValidationInfoBackground,
    inputValidationInfoForeground,
    inputValidationInfoBorder,
    inputValidationWarningBackground,
    inputValidationWarningForeground,
    inputValidationWarningBorder,
    inputValidationErrorBackground,
    inputValidationErrorForeground,
    inputValidationErrorBorder,
};
export const defaultMenuStyles = {
    shadowColor: widgetShadow,
    borderColor: menuBorder,
    foregroundColor: menuForeground,
    backgroundColor: menuBackground,
    selectionForegroundColor: menuSelectionForeground,
    selectionBackgroundColor: menuSelectionBackground,
    selectionBorderColor: menuSelectionBorder,
    separatorColor: menuSeparatorBackground,
    scrollbarShadow: scrollbarShadow,
    scrollbarSliderBackground: scrollbarSliderBackground,
    scrollbarSliderHoverBackground: scrollbarSliderHoverBackground,
    scrollbarSliderActiveBackground: scrollbarSliderActiveBackground
};
export function attachMenuStyler(widget, themeService, style) {
    return attachStyler(themeService, Object.assign(Object.assign({}, defaultMenuStyles), style), widget);
}
