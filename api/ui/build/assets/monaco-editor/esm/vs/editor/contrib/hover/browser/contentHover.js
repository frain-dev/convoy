/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
var __decorate = (this && this.__decorate) || function (decorators, target, key, desc) {
    var c = arguments.length, r = c < 3 ? target : desc === null ? desc = Object.getOwnPropertyDescriptor(target, key) : desc, d;
    if (typeof Reflect === "object" && typeof Reflect.decorate === "function") r = Reflect.decorate(decorators, target, key, desc);
    else for (var i = decorators.length - 1; i >= 0; i--) if (d = decorators[i]) r = (c < 3 ? d(r) : c > 3 ? d(target, key, r) : d(target, key)) || r;
    return c > 3 && r && Object.defineProperty(target, key, r), r;
};
var __param = (this && this.__param) || function (paramIndex, decorator) {
    return function (target, key) { decorator(target, key, paramIndex); }
};
import * as dom from '../../../../base/browser/dom.js';
import { HoverAction, HoverWidget } from '../../../../base/browser/ui/hover/hoverWidget.js';
import { coalesce } from '../../../../base/common/arrays.js';
import { Disposable, DisposableStore, toDisposable } from '../../../../base/common/lifecycle.js';
import { Position } from '../../../common/core/position.js';
import { Range } from '../../../common/core/range.js';
import { ModelDecorationOptions } from '../../../common/model/textModel.js';
import { TokenizationRegistry } from '../../../common/languages.js';
import { HoverOperation } from './hoverOperation.js';
import { HoverParticipantRegistry, HoverRangeAnchor } from './hoverTypes.js';
import { IContextKeyService } from '../../../../platform/contextkey/common/contextkey.js';
import { IInstantiationService } from '../../../../platform/instantiation/common/instantiation.js';
import { IKeybindingService } from '../../../../platform/keybinding/common/keybinding.js';
import { Context as SuggestContext } from '../../suggest/browser/suggest.js';
import { AsyncIterableObject } from '../../../../base/common/async.js';
import { EditorContextKeys } from '../../../common/editorContextKeys.js';
const $ = dom.$;
let ContentHoverController = class ContentHoverController extends Disposable {
    constructor(_editor, _instantiationService, _keybindingService) {
        super();
        this._editor = _editor;
        this._instantiationService = _instantiationService;
        this._keybindingService = _keybindingService;
        this._widget = this._register(this._instantiationService.createInstance(ContentHoverWidget, this._editor));
        this._isChangingDecorations = false;
        this._messages = [];
        this._messagesAreComplete = false;
        // Instantiate participants and sort them by `hoverOrdinal` which is relevant for rendering order.
        this._participants = [];
        for (const participant of HoverParticipantRegistry.getAll()) {
            this._participants.push(this._instantiationService.createInstance(participant, this._editor));
        }
        this._participants.sort((p1, p2) => p1.hoverOrdinal - p2.hoverOrdinal);
        this._computer = new ContentHoverComputer(this._editor, this._participants);
        this._hoverOperation = this._register(new HoverOperation(this._editor, this._computer));
        this._register(this._hoverOperation.onResult((result) => {
            this._withResult(result.value, result.isComplete, result.hasLoadingMessage);
        }));
        this._register(this._editor.onDidChangeModelDecorations(() => {
            if (this._isChangingDecorations) {
                return;
            }
            this._onModelDecorationsChanged();
        }));
        this._register(dom.addStandardDisposableListener(this._widget.getDomNode(), 'keydown', (e) => {
            if (e.equals(9 /* KeyCode.Escape */)) {
                this.hide();
            }
        }));
        this._register(TokenizationRegistry.onDidChange(() => {
            if (this._widget.position && this._computer.anchor && this._messages.length > 0) {
                this._widget.clear();
                this._renderMessages(this._computer.anchor, this._messages);
            }
        }));
    }
    _onModelDecorationsChanged() {
        if (this._widget.position) {
            // The decorations have changed and the hover is visible,
            // we need to recompute the displayed text
            this._hoverOperation.cancel();
            if (!this._widget.isColorPickerVisible) { // TODO@Michel ensure that displayed text for other decorations is computed even if color picker is in place
                this._hoverOperation.start(0 /* HoverStartMode.Delayed */);
            }
        }
    }
    maybeShowAt(mouseEvent) {
        const anchorCandidates = [];
        for (const participant of this._participants) {
            if (participant.suggestHoverAnchor) {
                const anchor = participant.suggestHoverAnchor(mouseEvent);
                if (anchor) {
                    anchorCandidates.push(anchor);
                }
            }
        }
        const target = mouseEvent.target;
        if (target.type === 6 /* MouseTargetType.CONTENT_TEXT */) {
            anchorCandidates.push(new HoverRangeAnchor(0, target.range));
        }
        if (target.type === 7 /* MouseTargetType.CONTENT_EMPTY */) {
            const epsilon = this._editor.getOption(46 /* EditorOption.fontInfo */).typicalHalfwidthCharacterWidth / 2;
            if (!target.detail.isAfterLines && typeof target.detail.horizontalDistanceToText === 'number' && target.detail.horizontalDistanceToText < epsilon) {
                // Let hover kick in even when the mouse is technically in the empty area after a line, given the distance is small enough
                anchorCandidates.push(new HoverRangeAnchor(0, target.range));
            }
        }
        if (anchorCandidates.length === 0) {
            return false;
        }
        anchorCandidates.sort((a, b) => b.priority - a.priority);
        this._startShowingAt(anchorCandidates[0], 0 /* HoverStartMode.Delayed */, false);
        return true;
    }
    startShowingAtRange(range, mode, focus) {
        this._startShowingAt(new HoverRangeAnchor(0, range), mode, focus);
    }
    _startShowingAt(anchor, mode, focus) {
        if (this._computer.anchor && this._computer.anchor.equals(anchor)) {
            // We have to show the widget at the exact same range as before, so no work is needed
            return;
        }
        this._hoverOperation.cancel();
        if (this._widget.position) {
            // The range might have changed, but the hover is visible
            // Instead of hiding it completely, filter out messages that are still in the new range and
            // kick off a new computation
            if (!this._computer.anchor || !anchor.canAdoptVisibleHover(this._computer.anchor, this._widget.position)) {
                this.hide();
            }
            else {
                const filteredMessages = this._messages.filter((m) => m.isValidForHoverAnchor(anchor));
                if (filteredMessages.length === 0) {
                    this.hide();
                }
                else if (filteredMessages.length === this._messages.length && this._messagesAreComplete) {
                    // no change
                    return;
                }
                else {
                    this._renderMessages(anchor, filteredMessages);
                }
            }
        }
        this._computer.anchor = anchor;
        this._computer.shouldFocus = focus;
        this._hoverOperation.start(mode);
    }
    hide() {
        this._computer.anchor = null;
        this._hoverOperation.cancel();
        this._widget.hide();
    }
    isColorPickerVisible() {
        return this._widget.isColorPickerVisible;
    }
    containsNode(node) {
        return this._widget.getDomNode().contains(node);
    }
    _addLoadingMessage(result) {
        if (this._computer.anchor) {
            for (const participant of this._participants) {
                if (participant.createLoadingMessage) {
                    const loadingMessage = participant.createLoadingMessage(this._computer.anchor);
                    if (loadingMessage) {
                        return result.slice(0).concat([loadingMessage]);
                    }
                }
            }
        }
        return result;
    }
    _withResult(result, isComplete, hasLoadingMessage) {
        this._messages = (hasLoadingMessage ? this._addLoadingMessage(result) : result);
        this._messagesAreComplete = isComplete;
        if (this._computer.anchor && this._messages.length > 0) {
            this._renderMessages(this._computer.anchor, this._messages);
        }
        else if (isComplete) {
            this.hide();
        }
    }
    _renderMessages(anchor, messages) {
        const { showAtPosition, showAtRange, highlightRange } = ContentHoverController.computeHoverRanges(anchor.range, messages);
        const disposables = new DisposableStore();
        const statusBar = disposables.add(new EditorHoverStatusBar(this._keybindingService));
        const fragment = document.createDocumentFragment();
        let colorPicker = null;
        const context = {
            fragment,
            statusBar,
            setColorPicker: (widget) => colorPicker = widget,
            onContentsChanged: () => this._widget.onContentsChanged(),
            hide: () => this.hide()
        };
        for (const participant of this._participants) {
            const hoverParts = messages.filter(msg => msg.owner === participant);
            if (hoverParts.length > 0) {
                disposables.add(participant.renderHoverParts(context, hoverParts));
            }
        }
        if (statusBar.hasContent) {
            fragment.appendChild(statusBar.hoverElement);
        }
        if (fragment.hasChildNodes()) {
            if (highlightRange) {
                const highlightDecoration = this._editor.createDecorationsCollection();
                try {
                    this._isChangingDecorations = true;
                    highlightDecoration.set([{
                            range: highlightRange,
                            options: ContentHoverController._DECORATION_OPTIONS
                        }]);
                }
                finally {
                    this._isChangingDecorations = false;
                }
                disposables.add(toDisposable(() => {
                    try {
                        this._isChangingDecorations = true;
                        highlightDecoration.clear();
                    }
                    finally {
                        this._isChangingDecorations = false;
                    }
                }));
            }
            this._widget.showAt(fragment, new ContentHoverVisibleData(colorPicker, showAtPosition, showAtRange, this._editor.getOption(55 /* EditorOption.hover */).above, this._computer.shouldFocus, disposables));
        }
        else {
            disposables.dispose();
        }
    }
    static computeHoverRanges(anchorRange, messages) {
        // The anchor range is always on a single line
        const anchorLineNumber = anchorRange.startLineNumber;
        let renderStartColumn = anchorRange.startColumn;
        let renderEndColumn = anchorRange.endColumn;
        let highlightRange = messages[0].range;
        let forceShowAtRange = null;
        for (const msg of messages) {
            highlightRange = Range.plusRange(highlightRange, msg.range);
            if (msg.range.startLineNumber === anchorLineNumber && msg.range.endLineNumber === anchorLineNumber) {
                // this message has a range that is completely sitting on the line of the anchor
                renderStartColumn = Math.min(renderStartColumn, msg.range.startColumn);
                renderEndColumn = Math.max(renderEndColumn, msg.range.endColumn);
            }
            if (msg.forceShowAtRange) {
                forceShowAtRange = msg.range;
            }
        }
        return {
            showAtPosition: forceShowAtRange ? forceShowAtRange.getStartPosition() : new Position(anchorRange.startLineNumber, renderStartColumn),
            showAtRange: forceShowAtRange ? forceShowAtRange : new Range(anchorLineNumber, renderStartColumn, anchorLineNumber, renderEndColumn),
            highlightRange
        };
    }
};
ContentHoverController._DECORATION_OPTIONS = ModelDecorationOptions.register({
    description: 'content-hover-highlight',
    className: 'hoverHighlight'
});
ContentHoverController = __decorate([
    __param(1, IInstantiationService),
    __param(2, IKeybindingService)
], ContentHoverController);
export { ContentHoverController };
class ContentHoverVisibleData {
    constructor(colorPicker, showAtPosition, showAtRange, preferAbove, stoleFocus, disposables) {
        this.colorPicker = colorPicker;
        this.showAtPosition = showAtPosition;
        this.showAtRange = showAtRange;
        this.preferAbove = preferAbove;
        this.stoleFocus = stoleFocus;
        this.disposables = disposables;
    }
}
let ContentHoverWidget = class ContentHoverWidget extends Disposable {
    constructor(_editor, _contextKeyService) {
        super();
        this._editor = _editor;
        this._contextKeyService = _contextKeyService;
        this.allowEditorOverflow = true;
        this._hoverVisibleKey = EditorContextKeys.hoverVisible.bindTo(this._contextKeyService);
        this._hover = this._register(new HoverWidget());
        this._visibleData = null;
        this._register(this._editor.onDidLayoutChange(() => this._layout()));
        this._register(this._editor.onDidChangeConfiguration((e) => {
            if (e.hasChanged(46 /* EditorOption.fontInfo */)) {
                this._updateFont();
            }
        }));
        this._setVisibleData(null);
        this._layout();
        this._editor.addContentWidget(this);
    }
    /**
     * Returns `null` if the hover is not visible.
     */
    get position() {
        var _a, _b;
        return (_b = (_a = this._visibleData) === null || _a === void 0 ? void 0 : _a.showAtPosition) !== null && _b !== void 0 ? _b : null;
    }
    get isColorPickerVisible() {
        var _a;
        return Boolean((_a = this._visibleData) === null || _a === void 0 ? void 0 : _a.colorPicker);
    }
    dispose() {
        this._editor.removeContentWidget(this);
        if (this._visibleData) {
            this._visibleData.disposables.dispose();
        }
        super.dispose();
    }
    getId() {
        return ContentHoverWidget.ID;
    }
    getDomNode() {
        return this._hover.containerDomNode;
    }
    getPosition() {
        if (!this._visibleData) {
            return null;
        }
        let preferAbove = this._visibleData.preferAbove;
        if (!preferAbove && this._contextKeyService.getContextKeyValue(SuggestContext.Visible.key)) {
            // Prefer rendering above if the suggest widget is visible
            preferAbove = true;
        }
        return {
            position: this._visibleData.showAtPosition,
            range: this._visibleData.showAtRange,
            preference: (preferAbove
                ? [1 /* ContentWidgetPositionPreference.ABOVE */, 2 /* ContentWidgetPositionPreference.BELOW */]
                : [2 /* ContentWidgetPositionPreference.BELOW */, 1 /* ContentWidgetPositionPreference.ABOVE */]),
        };
    }
    _setVisibleData(visibleData) {
        if (this._visibleData) {
            this._visibleData.disposables.dispose();
        }
        this._visibleData = visibleData;
        this._hoverVisibleKey.set(!!this._visibleData);
        this._hover.containerDomNode.classList.toggle('hidden', !this._visibleData);
    }
    _layout() {
        const height = Math.max(this._editor.getLayoutInfo().height / 4, 250);
        const { fontSize, lineHeight } = this._editor.getOption(46 /* EditorOption.fontInfo */);
        this._hover.contentsDomNode.style.fontSize = `${fontSize}px`;
        this._hover.contentsDomNode.style.lineHeight = `${lineHeight / fontSize}`;
        this._hover.contentsDomNode.style.maxHeight = `${height}px`;
        this._hover.contentsDomNode.style.maxWidth = `${Math.max(this._editor.getLayoutInfo().width * 0.66, 500)}px`;
    }
    _updateFont() {
        const codeClasses = Array.prototype.slice.call(this._hover.contentsDomNode.getElementsByClassName('code'));
        codeClasses.forEach(node => this._editor.applyFontInfo(node));
    }
    showAt(node, visibleData) {
        this._setVisibleData(visibleData);
        this._hover.contentsDomNode.textContent = '';
        this._hover.contentsDomNode.appendChild(node);
        this._hover.contentsDomNode.style.paddingBottom = '';
        this._updateFont();
        this.onContentsChanged();
        // Simply force a synchronous render on the editor
        // such that the widget does not really render with left = '0px'
        this._editor.render();
        // See https://github.com/microsoft/vscode/issues/140339
        // TODO: Doing a second layout of the hover after force rendering the editor
        this.onContentsChanged();
        if (visibleData.stoleFocus) {
            this._hover.containerDomNode.focus();
        }
        if (visibleData.colorPicker) {
            visibleData.colorPicker.layout();
        }
    }
    hide() {
        if (this._visibleData) {
            const stoleFocus = this._visibleData.stoleFocus;
            this._setVisibleData(null);
            this._editor.layoutContentWidget(this);
            if (stoleFocus) {
                this._editor.focus();
            }
        }
    }
    onContentsChanged() {
        this._editor.layoutContentWidget(this);
        this._hover.onContentsChanged();
        const scrollDimensions = this._hover.scrollbar.getScrollDimensions();
        const hasHorizontalScrollbar = (scrollDimensions.scrollWidth > scrollDimensions.width);
        if (hasHorizontalScrollbar) {
            // There is just a horizontal scrollbar
            const extraBottomPadding = `${this._hover.scrollbar.options.horizontalScrollbarSize}px`;
            if (this._hover.contentsDomNode.style.paddingBottom !== extraBottomPadding) {
                this._hover.contentsDomNode.style.paddingBottom = extraBottomPadding;
                this._editor.layoutContentWidget(this);
                this._hover.onContentsChanged();
            }
        }
    }
    clear() {
        this._hover.contentsDomNode.textContent = '';
    }
};
ContentHoverWidget.ID = 'editor.contrib.contentHoverWidget';
ContentHoverWidget = __decorate([
    __param(1, IContextKeyService)
], ContentHoverWidget);
export { ContentHoverWidget };
let EditorHoverStatusBar = class EditorHoverStatusBar extends Disposable {
    constructor(_keybindingService) {
        super();
        this._keybindingService = _keybindingService;
        this._hasContent = false;
        this.hoverElement = $('div.hover-row.status-bar');
        this.actionsElement = dom.append(this.hoverElement, $('div.actions'));
    }
    get hasContent() {
        return this._hasContent;
    }
    addAction(actionOptions) {
        const keybinding = this._keybindingService.lookupKeybinding(actionOptions.commandId);
        const keybindingLabel = keybinding ? keybinding.getLabel() : null;
        this._hasContent = true;
        return this._register(HoverAction.render(this.actionsElement, actionOptions, keybindingLabel));
    }
    append(element) {
        const result = dom.append(this.actionsElement, element);
        this._hasContent = true;
        return result;
    }
};
EditorHoverStatusBar = __decorate([
    __param(0, IKeybindingService)
], EditorHoverStatusBar);
class ContentHoverComputer {
    constructor(_editor, _participants) {
        this._editor = _editor;
        this._participants = _participants;
        this._anchor = null;
        this._shouldFocus = false;
    }
    get anchor() { return this._anchor; }
    set anchor(value) { this._anchor = value; }
    get shouldFocus() { return this._shouldFocus; }
    set shouldFocus(value) { this._shouldFocus = value; }
    static _getLineDecorations(editor, anchor) {
        if (anchor.type !== 1 /* HoverAnchorType.Range */) {
            return [];
        }
        const model = editor.getModel();
        const lineNumber = anchor.range.startLineNumber;
        if (lineNumber > model.getLineCount()) {
            // invalid line
            return [];
        }
        const maxColumn = model.getLineMaxColumn(lineNumber);
        return editor.getLineDecorations(lineNumber).filter((d) => {
            if (d.options.isWholeLine) {
                return true;
            }
            const startColumn = (d.range.startLineNumber === lineNumber) ? d.range.startColumn : 1;
            const endColumn = (d.range.endLineNumber === lineNumber) ? d.range.endColumn : maxColumn;
            if (d.options.showIfCollapsed) {
                // Relax check around `showIfCollapsed` decorations to also include +/- 1 character
                if (startColumn > anchor.range.startColumn + 1 || anchor.range.endColumn - 1 > endColumn) {
                    return false;
                }
            }
            else {
                if (startColumn > anchor.range.startColumn || anchor.range.endColumn > endColumn) {
                    return false;
                }
            }
            return true;
        });
    }
    computeAsync(token) {
        const anchor = this._anchor;
        if (!this._editor.hasModel() || !anchor) {
            return AsyncIterableObject.EMPTY;
        }
        const lineDecorations = ContentHoverComputer._getLineDecorations(this._editor, anchor);
        return AsyncIterableObject.merge(this._participants.map((participant) => {
            if (!participant.computeAsync) {
                return AsyncIterableObject.EMPTY;
            }
            return participant.computeAsync(anchor, lineDecorations, token);
        }));
    }
    computeSync() {
        if (!this._editor.hasModel() || !this._anchor) {
            return [];
        }
        const lineDecorations = ContentHoverComputer._getLineDecorations(this._editor, this._anchor);
        let result = [];
        for (const participant of this._participants) {
            result = result.concat(participant.computeSync(this._anchor, lineDecorations));
        }
        return coalesce(result);
    }
}
