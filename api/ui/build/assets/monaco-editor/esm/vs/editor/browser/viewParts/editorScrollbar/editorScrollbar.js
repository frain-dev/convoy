/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import * as dom from '../../../../base/browser/dom.js';
import { createFastDomNode } from '../../../../base/browser/fastDomNode.js';
import { SmoothScrollableElement } from '../../../../base/browser/ui/scrollbar/scrollableElement.js';
import { PartFingerprints, ViewPart } from '../../view/viewPart.js';
import { registerThemingParticipant, getThemeTypeSelector } from '../../../../platform/theme/common/themeService.js';
import { scrollbarShadow, scrollbarSliderActiveBackground, scrollbarSliderBackground, scrollbarSliderHoverBackground } from '../../../../platform/theme/common/colorRegistry.js';
export class EditorScrollbar extends ViewPart {
    constructor(context, linesContent, viewDomNode, overflowGuardDomNode) {
        super(context);
        const options = this._context.configuration.options;
        const scrollbar = options.get(94 /* EditorOption.scrollbar */);
        const mouseWheelScrollSensitivity = options.get(69 /* EditorOption.mouseWheelScrollSensitivity */);
        const fastScrollSensitivity = options.get(36 /* EditorOption.fastScrollSensitivity */);
        const scrollPredominantAxis = options.get(97 /* EditorOption.scrollPredominantAxis */);
        const scrollbarOptions = {
            listenOnDomNode: viewDomNode.domNode,
            className: 'editor-scrollable' + ' ' + getThemeTypeSelector(context.theme.type),
            useShadows: false,
            lazyRender: true,
            vertical: scrollbar.vertical,
            horizontal: scrollbar.horizontal,
            verticalHasArrows: scrollbar.verticalHasArrows,
            horizontalHasArrows: scrollbar.horizontalHasArrows,
            verticalScrollbarSize: scrollbar.verticalScrollbarSize,
            verticalSliderSize: scrollbar.verticalSliderSize,
            horizontalScrollbarSize: scrollbar.horizontalScrollbarSize,
            horizontalSliderSize: scrollbar.horizontalSliderSize,
            handleMouseWheel: scrollbar.handleMouseWheel,
            alwaysConsumeMouseWheel: scrollbar.alwaysConsumeMouseWheel,
            arrowSize: scrollbar.arrowSize,
            mouseWheelScrollSensitivity: mouseWheelScrollSensitivity,
            fastScrollSensitivity: fastScrollSensitivity,
            scrollPredominantAxis: scrollPredominantAxis,
            scrollByPage: scrollbar.scrollByPage,
        };
        this.scrollbar = this._register(new SmoothScrollableElement(linesContent.domNode, scrollbarOptions, this._context.viewLayout.getScrollable()));
        PartFingerprints.write(this.scrollbar.getDomNode(), 5 /* PartFingerprint.ScrollableElement */);
        this.scrollbarDomNode = createFastDomNode(this.scrollbar.getDomNode());
        this.scrollbarDomNode.setPosition('absolute');
        this._setLayout();
        // When having a zone widget that calls .focus() on one of its dom elements,
        // the browser will try desperately to reveal that dom node, unexpectedly
        // changing the .scrollTop of this.linesContent
        const onBrowserDesperateReveal = (domNode, lookAtScrollTop, lookAtScrollLeft) => {
            const newScrollPosition = {};
            if (lookAtScrollTop) {
                const deltaTop = domNode.scrollTop;
                if (deltaTop) {
                    newScrollPosition.scrollTop = this._context.viewLayout.getCurrentScrollTop() + deltaTop;
                    domNode.scrollTop = 0;
                }
            }
            if (lookAtScrollLeft) {
                const deltaLeft = domNode.scrollLeft;
                if (deltaLeft) {
                    newScrollPosition.scrollLeft = this._context.viewLayout.getCurrentScrollLeft() + deltaLeft;
                    domNode.scrollLeft = 0;
                }
            }
            this._context.viewModel.viewLayout.setScrollPosition(newScrollPosition, 1 /* ScrollType.Immediate */);
        };
        // I've seen this happen both on the view dom node & on the lines content dom node.
        this._register(dom.addDisposableListener(viewDomNode.domNode, 'scroll', (e) => onBrowserDesperateReveal(viewDomNode.domNode, true, true)));
        this._register(dom.addDisposableListener(linesContent.domNode, 'scroll', (e) => onBrowserDesperateReveal(linesContent.domNode, true, false)));
        this._register(dom.addDisposableListener(overflowGuardDomNode.domNode, 'scroll', (e) => onBrowserDesperateReveal(overflowGuardDomNode.domNode, true, false)));
        this._register(dom.addDisposableListener(this.scrollbarDomNode.domNode, 'scroll', (e) => onBrowserDesperateReveal(this.scrollbarDomNode.domNode, true, false)));
    }
    dispose() {
        super.dispose();
    }
    _setLayout() {
        const options = this._context.configuration.options;
        const layoutInfo = options.get(133 /* EditorOption.layoutInfo */);
        this.scrollbarDomNode.setLeft(layoutInfo.contentLeft);
        const minimap = options.get(67 /* EditorOption.minimap */);
        const side = minimap.side;
        if (side === 'right') {
            this.scrollbarDomNode.setWidth(layoutInfo.contentWidth + layoutInfo.minimap.minimapWidth);
        }
        else {
            this.scrollbarDomNode.setWidth(layoutInfo.contentWidth);
        }
        this.scrollbarDomNode.setHeight(layoutInfo.height);
    }
    getOverviewRulerLayoutInfo() {
        return this.scrollbar.getOverviewRulerLayoutInfo();
    }
    getDomNode() {
        return this.scrollbarDomNode;
    }
    delegateVerticalScrollbarPointerDown(browserEvent) {
        this.scrollbar.delegateVerticalScrollbarPointerDown(browserEvent);
    }
    // --- begin event handlers
    onConfigurationChanged(e) {
        if (e.hasChanged(94 /* EditorOption.scrollbar */)
            || e.hasChanged(69 /* EditorOption.mouseWheelScrollSensitivity */)
            || e.hasChanged(36 /* EditorOption.fastScrollSensitivity */)) {
            const options = this._context.configuration.options;
            const scrollbar = options.get(94 /* EditorOption.scrollbar */);
            const mouseWheelScrollSensitivity = options.get(69 /* EditorOption.mouseWheelScrollSensitivity */);
            const fastScrollSensitivity = options.get(36 /* EditorOption.fastScrollSensitivity */);
            const scrollPredominantAxis = options.get(97 /* EditorOption.scrollPredominantAxis */);
            const newOpts = {
                vertical: scrollbar.vertical,
                horizontal: scrollbar.horizontal,
                verticalScrollbarSize: scrollbar.verticalScrollbarSize,
                horizontalScrollbarSize: scrollbar.horizontalScrollbarSize,
                scrollByPage: scrollbar.scrollByPage,
                handleMouseWheel: scrollbar.handleMouseWheel,
                mouseWheelScrollSensitivity: mouseWheelScrollSensitivity,
                fastScrollSensitivity: fastScrollSensitivity,
                scrollPredominantAxis: scrollPredominantAxis
            };
            this.scrollbar.updateOptions(newOpts);
        }
        if (e.hasChanged(133 /* EditorOption.layoutInfo */)) {
            this._setLayout();
        }
        return true;
    }
    onScrollChanged(e) {
        return true;
    }
    onThemeChanged(e) {
        this.scrollbar.updateClassName('editor-scrollable' + ' ' + getThemeTypeSelector(this._context.theme.type));
        return true;
    }
    // --- end event handlers
    prepareRender(ctx) {
        // Nothing to do
    }
    render(ctx) {
        this.scrollbar.renderNow();
    }
}
registerThemingParticipant((theme, collector) => {
    // Scrollbars
    const scrollbarShadowColor = theme.getColor(scrollbarShadow);
    if (scrollbarShadowColor) {
        collector.addRule(`
			.monaco-scrollable-element > .shadow.top {
				box-shadow: ${scrollbarShadowColor} 0 6px 6px -6px inset;
			}

			.monaco-scrollable-element > .shadow.left {
				box-shadow: ${scrollbarShadowColor} 6px 0 6px -6px inset;
			}

			.monaco-scrollable-element > .shadow.top.left {
				box-shadow: ${scrollbarShadowColor} 6px 6px 6px -6px inset;
			}
		`);
    }
    const scrollbarSliderBackgroundColor = theme.getColor(scrollbarSliderBackground);
    if (scrollbarSliderBackgroundColor) {
        collector.addRule(`
			.monaco-scrollable-element > .scrollbar > .slider {
				background: ${scrollbarSliderBackgroundColor};
			}
		`);
    }
    const scrollbarSliderHoverBackgroundColor = theme.getColor(scrollbarSliderHoverBackground);
    if (scrollbarSliderHoverBackgroundColor) {
        collector.addRule(`
			.monaco-scrollable-element > .scrollbar > .slider:hover {
				background: ${scrollbarSliderHoverBackgroundColor};
			}
		`);
    }
    const scrollbarSliderActiveBackgroundColor = theme.getColor(scrollbarSliderActiveBackground);
    if (scrollbarSliderActiveBackgroundColor) {
        collector.addRule(`
			.monaco-scrollable-element > .scrollbar > .slider.active {
				background: ${scrollbarSliderActiveBackgroundColor};
			}
		`);
    }
});
