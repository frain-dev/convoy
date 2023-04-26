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
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var _a;
import { Disposable, DisposableStore } from '../../../../base/common/lifecycle.js';
import { registerEditorContribution } from '../../../browser/editorExtensions.js';
import { ILanguageFeaturesService } from '../../../common/services/languageFeatures.js';
import { OutlineModel, OutlineElement } from '../../documentSymbols/browser/outlineModel.js';
import { CancellationTokenSource } from '../../../../base/common/cancellation.js';
import * as dom from '../../../../base/browser/dom.js';
import { createStringBuilder } from '../../../common/core/stringBuilder.js';
import { RenderLineInput, renderViewLine } from '../../../common/viewLayout/viewLineRenderer.js';
import { LineDecoration } from '../../../common/viewLayout/lineDecorations.js';
import { RunOnceScheduler } from '../../../../base/common/async.js';
import { Position } from '../../../common/core/position.js';
let StickyScrollController = class StickyScrollController extends Disposable {
    constructor(editor, _languageFeaturesService) {
        super();
        this._sessionStore = new DisposableStore();
        this._ranges = [];
        this._rangesVersionId = 0;
        this._editor = editor;
        this._languageFeaturesService = _languageFeaturesService;
        this.stickyScrollWidget = new StickyScrollWidget(this._editor);
        this._register(this._editor.onDidChangeConfiguration(e => {
            if (e.hasChanged(34 /* EditorOption.experimental */)) {
                this.onConfigurationChange();
            }
        }));
        this._updateSoon = this._register(new RunOnceScheduler(() => this._update(true), 50));
        this.onConfigurationChange();
    }
    onConfigurationChange() {
        const options = this._editor.getOption(34 /* EditorOption.experimental */);
        if (options.stickyScroll.enabled === false) {
            this.stickyScrollWidget.emptyRootNode();
            this._editor.removeOverlayWidget(this.stickyScrollWidget);
            this._sessionStore.clear();
            return;
        }
        else {
            this._editor.addOverlayWidget(this.stickyScrollWidget);
            this._sessionStore.add(this._editor.onDidChangeModel(() => this._update(true)));
            this._sessionStore.add(this._editor.onDidScrollChange(() => this._update(false)));
            this._sessionStore.add(this._editor.onDidChangeHiddenAreas(() => this._update(true)));
            this._sessionStore.add(this._editor.onDidChangeModelTokens((e) => this._onTokensChange(e)));
            this._sessionStore.add(this._editor.onDidChangeModelContent(() => this._updateSoon.schedule()));
            this._sessionStore.add(this._languageFeaturesService.documentSymbolProvider.onDidChange(() => this._update(true)));
            this._update(true);
        }
    }
    _needsUpdate(event) {
        const stickyLineNumbers = this.stickyScrollWidget.getCurrentLines();
        for (const stickyLineNumber of stickyLineNumbers) {
            for (const range of event.ranges) {
                if (stickyLineNumber >= range.fromLineNumber && stickyLineNumber <= range.toLineNumber) {
                    return true;
                }
            }
        }
        return false;
    }
    _onTokensChange(event) {
        if (this._needsUpdate(event)) {
            this._update(false);
        }
    }
    _update(updateOutline = false) {
        var _a, _b;
        return __awaiter(this, void 0, void 0, function* () {
            if (updateOutline) {
                (_a = this._cts) === null || _a === void 0 ? void 0 : _a.dispose(true);
                this._cts = new CancellationTokenSource();
                yield this._updateOutlineModel(this._cts.token);
            }
            const hiddenRanges = (_b = this._editor._getViewModel()) === null || _b === void 0 ? void 0 : _b.getHiddenAreas();
            if (hiddenRanges) {
                for (const hiddenRange of hiddenRanges) {
                    this._ranges = this._ranges.filter(range => { return !(range[0] >= hiddenRange.startLineNumber && range[1] <= hiddenRange.endLineNumber + 1); });
                }
            }
            this._renderStickyScroll();
        });
    }
    _findLineRanges(outlineElement, depth) {
        if (outlineElement === null || outlineElement === void 0 ? void 0 : outlineElement.children.size) {
            let didRecursion = false;
            for (const outline of outlineElement === null || outlineElement === void 0 ? void 0 : outlineElement.children.values()) {
                const kind = outline.symbol.kind;
                if (kind === 4 /* SymbolKind.Class */ || kind === 8 /* SymbolKind.Constructor */ || kind === 11 /* SymbolKind.Function */ || kind === 10 /* SymbolKind.Interface */ || kind === 5 /* SymbolKind.Method */ || kind === 1 /* SymbolKind.Module */) {
                    didRecursion = true;
                    this._findLineRanges(outline, depth + 1);
                }
            }
            if (!didRecursion) {
                this._addOutlineRanges(outlineElement, depth);
            }
        }
        else {
            this._addOutlineRanges(outlineElement, depth);
        }
    }
    _addOutlineRanges(outlineElement, depth) {
        let currentStartLine = 0;
        let currentEndLine = 0;
        while (outlineElement) {
            const kind = outlineElement.symbol.kind;
            if (kind === 4 /* SymbolKind.Class */ || kind === 8 /* SymbolKind.Constructor */ || kind === 11 /* SymbolKind.Function */ || kind === 10 /* SymbolKind.Interface */ || kind === 5 /* SymbolKind.Method */ || kind === 1 /* SymbolKind.Module */) {
                currentStartLine = outlineElement === null || outlineElement === void 0 ? void 0 : outlineElement.symbol.range.startLineNumber;
                currentEndLine = outlineElement === null || outlineElement === void 0 ? void 0 : outlineElement.symbol.range.endLineNumber;
                this._ranges.push([currentStartLine, currentEndLine, depth]);
                depth--;
            }
            if (outlineElement.parent instanceof OutlineElement) {
                outlineElement = outlineElement.parent;
            }
            else {
                break;
            }
        }
    }
    _updateOutlineModel(token) {
        return __awaiter(this, void 0, void 0, function* () {
            if (this._editor.hasModel()) {
                const model = this._editor.getModel();
                const modelVersionId = model.getVersionId();
                const outlineModel = yield OutlineModel.create(this._languageFeaturesService.documentSymbolProvider, model, token);
                if (token.isCancellationRequested) {
                    return;
                }
                this._ranges = [];
                this._rangesVersionId = modelVersionId;
                for (const outline of outlineModel.children.values()) {
                    if (outline instanceof OutlineElement) {
                        const kind = outline.symbol.kind;
                        if (kind === 4 /* SymbolKind.Class */ || kind === 8 /* SymbolKind.Constructor */ || kind === 11 /* SymbolKind.Function */ || kind === 10 /* SymbolKind.Interface */ || kind === 5 /* SymbolKind.Method */ || kind === 1 /* SymbolKind.Module */) {
                            this._findLineRanges(outline, 1);
                        }
                        else {
                            this._findLineRanges(outline, 0);
                        }
                    }
                    this._ranges = this._ranges.sort(function (a, b) {
                        if (a[0] !== b[0]) {
                            return a[0] - b[0];
                        }
                        else if (a[1] !== b[1]) {
                            return b[1] - a[1];
                        }
                        else {
                            return a[2] - b[2];
                        }
                    });
                    let previous = [];
                    for (const [index, arr] of this._ranges.entries()) {
                        const [start, end, _depth] = arr;
                        if (previous[0] === start && previous[1] === end) {
                            this._ranges.splice(index, 1);
                        }
                        else {
                            previous = arr;
                        }
                    }
                }
            }
        });
    }
    _renderStickyScroll() {
        if (!(this._editor.hasModel())) {
            return;
        }
        const lineHeight = this._editor.getOption(61 /* EditorOption.lineHeight */);
        const model = this._editor.getModel();
        if (this._rangesVersionId !== model.getVersionId()) {
            // Old _ranges not updated yet
            return;
        }
        const scrollTop = this._editor.getScrollTop();
        this.stickyScrollWidget.emptyRootNode();
        const beginningLinesConsidered = new Set();
        for (const [index, arr] of this._ranges.entries()) {
            const [start, end, depth] = arr;
            if (end - start > 0 && model.getLineContent(start) !== '') {
                const topOfElementAtDepth = (depth - 1) * lineHeight;
                const bottomOfElementAtDepth = depth * lineHeight;
                const bottomOfBeginningLine = this._editor.getBottomForLineNumber(start) - scrollTop;
                const topOfEndLine = this._editor.getTopForLineNumber(end) - scrollTop;
                const bottomOfEndLine = this._editor.getBottomForLineNumber(end) - scrollTop;
                if (!beginningLinesConsidered.has(start)) {
                    if (topOfElementAtDepth >= topOfEndLine - 1 && topOfElementAtDepth < bottomOfEndLine - 2) {
                        beginningLinesConsidered.add(start);
                        this.stickyScrollWidget.pushCodeLine(new StickyScrollCodeLine(start, depth, this._editor, -1, bottomOfEndLine - bottomOfElementAtDepth));
                        break;
                    }
                    else if (bottomOfElementAtDepth > bottomOfBeginningLine && bottomOfElementAtDepth < bottomOfEndLine - 1) {
                        beginningLinesConsidered.add(start);
                        this.stickyScrollWidget.pushCodeLine(new StickyScrollCodeLine(start, depth, this._editor, 0, 0));
                    }
                }
                else {
                    this._ranges.splice(index, 1);
                }
            }
        }
        this.stickyScrollWidget.updateRootNode();
    }
    dispose() {
        super.dispose();
        this._sessionStore.dispose();
    }
};
StickyScrollController.ID = 'store.contrib.stickyScrollController';
StickyScrollController = __decorate([
    __param(1, ILanguageFeaturesService)
], StickyScrollController);
const _ttPolicy = (_a = window.trustedTypes) === null || _a === void 0 ? void 0 : _a.createPolicy('stickyScrollViewLayer', { createHTML: value => value });
class StickyScrollCodeLine {
    constructor(_lineNumber, _depth, _editor, _zIndex, _relativePosition) {
        this._lineNumber = _lineNumber;
        this._depth = _depth;
        this._editor = _editor;
        this._zIndex = _zIndex;
        this._relativePosition = _relativePosition;
        this.effectiveLineHeight = 0;
        this.effectiveLineHeight = this._editor.getOption(61 /* EditorOption.lineHeight */) + this._relativePosition;
    }
    get lineNumber() {
        return this._lineNumber;
    }
    getDomNode() {
        const root = document.createElement('div');
        const viewModel = this._editor._getViewModel();
        const viewLineNumber = viewModel.coordinatesConverter.convertModelPositionToViewPosition(new Position(this._lineNumber, 1)).lineNumber;
        const lineRenderingData = viewModel.getViewLineRenderingData(viewLineNumber);
        let actualInlineDecorations;
        try {
            actualInlineDecorations = LineDecoration.filter(lineRenderingData.inlineDecorations, viewLineNumber, lineRenderingData.minColumn, lineRenderingData.maxColumn);
        }
        catch (err) {
            actualInlineDecorations = [];
        }
        const renderLineInput = new RenderLineInput(true, true, lineRenderingData.content, lineRenderingData.continuesWithWrappedLine, lineRenderingData.isBasicASCII, lineRenderingData.containsRTL, 0, lineRenderingData.tokens, actualInlineDecorations, lineRenderingData.tabSize, lineRenderingData.startVisibleColumn, 1, 1, 1, 100, 'none', true, true, null);
        const sb = createStringBuilder(400);
        renderViewLine(renderLineInput, sb);
        let newLine;
        if (_ttPolicy) {
            newLine = _ttPolicy.createHTML(sb.build());
        }
        else {
            newLine = sb.build();
        }
        const lineHTMLNode = document.createElement('span');
        lineHTMLNode.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
        lineHTMLNode.style.overflow = 'hidden';
        lineHTMLNode.style.whiteSpace = 'nowrap';
        lineHTMLNode.style.display = 'inline-block';
        lineHTMLNode.style.lineHeight = this._editor.getOption(61 /* EditorOption.lineHeight */).toString() + 'px';
        lineHTMLNode.innerHTML = newLine;
        const lineNumberHTMLNode = document.createElement('span');
        lineNumberHTMLNode.style.width = this._editor.getLayoutInfo().contentLeft.toString() + 'px';
        lineNumberHTMLNode.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
        lineNumberHTMLNode.style.color = 'var(--vscode-editorLineNumber-foreground)';
        lineNumberHTMLNode.style.display = 'inline-block';
        lineNumberHTMLNode.style.lineHeight = this._editor.getOption(61 /* EditorOption.lineHeight */).toString() + 'px';
        const innerLineNumberHTML = document.createElement('span');
        innerLineNumberHTML.innerText = this._lineNumber.toString();
        innerLineNumberHTML.style.paddingLeft = this._editor.getLayoutInfo().lineNumbersLeft.toString() + 'px';
        innerLineNumberHTML.style.width = this._editor.getLayoutInfo().lineNumbersWidth.toString() + 'px';
        innerLineNumberHTML.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
        innerLineNumberHTML.style.textAlign = 'right';
        innerLineNumberHTML.style.float = 'left';
        innerLineNumberHTML.style.lineHeight = this._editor.getOption(61 /* EditorOption.lineHeight */).toString() + 'px';
        lineNumberHTMLNode.appendChild(innerLineNumberHTML);
        root.onclick = e => {
            e.stopPropagation();
            e.preventDefault();
            this._editor.revealPosition({ lineNumber: this._lineNumber - this._depth + 1, column: 1 });
        };
        root.onmouseover = e => {
            innerLineNumberHTML.style.background = `var(--vscode-editorStickyScrollHover-background)`;
            lineHTMLNode.style.backgroundColor = `var(--vscode-editorStickyScrollHover-background)`;
            lineNumberHTMLNode.style.backgroundColor = `var(--vscode-editorStickyScrollHover-background)`;
            root.style.backgroundColor = `var(--vscode-editorStickyScrollHover-background)`;
            innerLineNumberHTML.style.cursor = `pointer`;
            lineHTMLNode.style.cursor = `pointer`;
            root.style.cursor = `pointer`;
            lineNumberHTMLNode.style.cursor = `pointer`;
        };
        root.onmouseleave = e => {
            innerLineNumberHTML.style.background = `var(--vscode-editorStickyScroll-background)`;
            lineHTMLNode.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
            lineNumberHTMLNode.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
            root.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
        };
        this._editor.applyFontInfo(lineHTMLNode);
        this._editor.applyFontInfo(innerLineNumberHTML);
        root.appendChild(lineNumberHTMLNode);
        root.appendChild(lineHTMLNode);
        root.style.zIndex = this._zIndex.toString();
        root.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
        root.style.overflow = 'hidden';
        root.style.whiteSpace = 'nowrap';
        root.style.width = '100%';
        root.style.lineHeight = this._editor.getOption(61 /* EditorOption.lineHeight */).toString() + 'px';
        root.style.height = this._editor.getOption(61 /* EditorOption.lineHeight */).toString() + 'px';
        // Special case for last line of sticky scroll
        if (this._relativePosition) {
            root.style.position = 'relative';
            root.style.top = this._relativePosition + 'px';
            root.style.width = '100%';
        }
        return root;
    }
}
class StickyScrollWidget {
    constructor(_editor) {
        this._editor = _editor;
        this.arrayOfCodeLines = [];
        this.rootDomNode = document.createElement('div');
        this.rootDomNode = document.createElement('div');
        this.rootDomNode.style.width = '100%';
        this.rootDomNode.style.boxShadow = `var(--vscode-scrollbar-shadow) 0 6px 6px -6px`;
    }
    getCurrentLines() {
        const widgetLineRange = [];
        for (const codeLine of this.arrayOfCodeLines) {
            widgetLineRange.push(codeLine.lineNumber);
        }
        return widgetLineRange;
    }
    pushCodeLine(codeLine) {
        this.arrayOfCodeLines.push(codeLine);
    }
    updateRootNode() {
        let widgetHeight = 0;
        for (const line of this.arrayOfCodeLines) {
            widgetHeight += line.effectiveLineHeight;
            this.rootDomNode.appendChild(line.getDomNode());
        }
        this.rootDomNode.style.height = widgetHeight.toString() + 'px';
    }
    emptyRootNode() {
        this.arrayOfCodeLines.length = 0;
        dom.clearNode(this.rootDomNode);
    }
    getId() {
        return 'editor.contrib.stickyScrollWidget';
    }
    getDomNode() {
        this.rootDomNode.style.zIndex = '2';
        this.rootDomNode.style.backgroundColor = `var(--vscode-editorStickyScroll-background)`;
        return this.rootDomNode;
    }
    getPosition() {
        return {
            preference: null
        };
    }
}
registerEditorContribution(StickyScrollController.ID, StickyScrollController);
