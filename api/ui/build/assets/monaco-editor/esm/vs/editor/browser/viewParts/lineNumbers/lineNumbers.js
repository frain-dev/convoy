/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import './lineNumbers.css';
import * as platform from '../../../../base/common/platform.js';
import { DynamicViewOverlay } from '../../view/dynamicViewOverlay.js';
import { Position } from '../../../common/core/position.js';
import { editorActiveLineNumber, editorLineNumbers } from '../../../common/core/editorColorRegistry.js';
import { registerThemingParticipant } from '../../../../platform/theme/common/themeService.js';
export class LineNumbersOverlay extends DynamicViewOverlay {
    constructor(context) {
        super();
        this._context = context;
        this._readConfig();
        this._lastCursorModelPosition = new Position(1, 1);
        this._lastCursorViewPosition = new Position(1, 1);
        this._renderResult = null;
        this._activeLineNumber = 1;
        this._context.addEventHandler(this);
    }
    _readConfig() {
        const options = this._context.configuration.options;
        this._lineHeight = options.get(61 /* EditorOption.lineHeight */);
        const lineNumbers = options.get(62 /* EditorOption.lineNumbers */);
        this._renderLineNumbers = lineNumbers.renderType;
        this._renderCustomLineNumbers = lineNumbers.renderFn;
        this._renderFinalNewline = options.get(86 /* EditorOption.renderFinalNewline */);
        const layoutInfo = options.get(133 /* EditorOption.layoutInfo */);
        this._lineNumbersLeft = layoutInfo.lineNumbersLeft;
        this._lineNumbersWidth = layoutInfo.lineNumbersWidth;
    }
    dispose() {
        this._context.removeEventHandler(this);
        this._renderResult = null;
        super.dispose();
    }
    // --- begin event handlers
    onConfigurationChanged(e) {
        this._readConfig();
        return true;
    }
    onCursorStateChanged(e) {
        const primaryViewPosition = e.selections[0].getPosition();
        this._lastCursorViewPosition = primaryViewPosition;
        this._lastCursorModelPosition = this._context.viewModel.coordinatesConverter.convertViewPositionToModelPosition(primaryViewPosition);
        let shouldRender = false;
        if (this._activeLineNumber !== primaryViewPosition.lineNumber) {
            this._activeLineNumber = primaryViewPosition.lineNumber;
            shouldRender = true;
        }
        if (this._renderLineNumbers === 2 /* RenderLineNumbersType.Relative */ || this._renderLineNumbers === 3 /* RenderLineNumbersType.Interval */) {
            shouldRender = true;
        }
        return shouldRender;
    }
    onFlushed(e) {
        return true;
    }
    onLinesChanged(e) {
        return true;
    }
    onLinesDeleted(e) {
        return true;
    }
    onLinesInserted(e) {
        return true;
    }
    onScrollChanged(e) {
        return e.scrollTopChanged;
    }
    onZonesChanged(e) {
        return true;
    }
    // --- end event handlers
    _getLineRenderLineNumber(viewLineNumber) {
        const modelPosition = this._context.viewModel.coordinatesConverter.convertViewPositionToModelPosition(new Position(viewLineNumber, 1));
        if (modelPosition.column !== 1) {
            return '';
        }
        const modelLineNumber = modelPosition.lineNumber;
        if (this._renderCustomLineNumbers) {
            return this._renderCustomLineNumbers(modelLineNumber);
        }
        if (this._renderLineNumbers === 3 /* RenderLineNumbersType.Interval */) {
            if (this._lastCursorModelPosition.lineNumber === modelLineNumber) {
                return String(modelLineNumber);
            }
            if (modelLineNumber % 10 === 0) {
                return String(modelLineNumber);
            }
            return '';
        }
        return String(modelLineNumber);
    }
    prepareRender(ctx) {
        if (this._renderLineNumbers === 0 /* RenderLineNumbersType.Off */) {
            this._renderResult = null;
            return;
        }
        const lineHeightClassName = (platform.isLinux ? (this._lineHeight % 2 === 0 ? ' lh-even' : ' lh-odd') : '');
        const visibleStartLineNumber = ctx.visibleRange.startLineNumber;
        const visibleEndLineNumber = ctx.visibleRange.endLineNumber;
        const common = '<div class="' + LineNumbersOverlay.CLASS_NAME + lineHeightClassName + '" style="left:' + this._lineNumbersLeft + 'px;width:' + this._lineNumbersWidth + 'px;">';
        let relativeLineNumbers = null;
        if (this._renderLineNumbers === 2 /* RenderLineNumbersType.Relative */) {
            relativeLineNumbers = new Array(visibleEndLineNumber - visibleStartLineNumber + 1);
            if (this._lastCursorViewPosition.lineNumber >= visibleStartLineNumber && this._lastCursorViewPosition.lineNumber <= visibleEndLineNumber) {
                relativeLineNumbers[this._lastCursorViewPosition.lineNumber - visibleStartLineNumber] = this._lastCursorModelPosition.lineNumber;
            }
            // Iterate up to compute relative line numbers
            {
                let value = 0;
                for (let lineNumber = this._lastCursorViewPosition.lineNumber + 1; lineNumber <= visibleEndLineNumber; lineNumber++) {
                    const modelPosition = this._context.viewModel.coordinatesConverter.convertViewPositionToModelPosition(new Position(lineNumber, 1));
                    const isWrappedLine = (modelPosition.column !== 1);
                    if (!isWrappedLine) {
                        value++;
                    }
                    if (lineNumber >= visibleStartLineNumber) {
                        relativeLineNumbers[lineNumber - visibleStartLineNumber] = isWrappedLine ? 0 : value;
                    }
                }
            }
            // Iterate down to compute relative line numbers
            {
                let value = 0;
                for (let lineNumber = this._lastCursorViewPosition.lineNumber - 1; lineNumber >= visibleStartLineNumber; lineNumber--) {
                    const modelPosition = this._context.viewModel.coordinatesConverter.convertViewPositionToModelPosition(new Position(lineNumber, 1));
                    const isWrappedLine = (modelPosition.column !== 1);
                    if (!isWrappedLine) {
                        value++;
                    }
                    if (lineNumber <= visibleEndLineNumber) {
                        relativeLineNumbers[lineNumber - visibleStartLineNumber] = isWrappedLine ? 0 : value;
                    }
                }
            }
        }
        const lineCount = this._context.viewModel.getLineCount();
        const output = [];
        for (let lineNumber = visibleStartLineNumber; lineNumber <= visibleEndLineNumber; lineNumber++) {
            const lineIndex = lineNumber - visibleStartLineNumber;
            if (!this._renderFinalNewline) {
                if (lineNumber === lineCount && this._context.viewModel.getLineLength(lineNumber) === 0) {
                    // Do not render last (empty) line
                    output[lineIndex] = '';
                    continue;
                }
            }
            let renderLineNumber;
            if (relativeLineNumbers) {
                const relativeLineNumber = relativeLineNumbers[lineIndex];
                if (this._lastCursorViewPosition.lineNumber === lineNumber) {
                    // current line!
                    renderLineNumber = `<span class="relative-current-line-number">${relativeLineNumber}</span>`;
                }
                else if (relativeLineNumber) {
                    renderLineNumber = String(relativeLineNumber);
                }
                else {
                    renderLineNumber = '';
                }
            }
            else {
                renderLineNumber = this._getLineRenderLineNumber(lineNumber);
            }
            if (renderLineNumber) {
                if (lineNumber === this._activeLineNumber) {
                    output[lineIndex] = ('<div class="active-line-number ' + LineNumbersOverlay.CLASS_NAME + lineHeightClassName + '" style="left:' + this._lineNumbersLeft + 'px;width:' + this._lineNumbersWidth + 'px;">'
                        + renderLineNumber
                        + '</div>');
                }
                else {
                    output[lineIndex] = (common
                        + renderLineNumber
                        + '</div>');
                }
            }
            else {
                output[lineIndex] = '';
            }
        }
        this._renderResult = output;
    }
    render(startLineNumber, lineNumber) {
        if (!this._renderResult) {
            return '';
        }
        const lineIndex = lineNumber - startLineNumber;
        if (lineIndex < 0 || lineIndex >= this._renderResult.length) {
            return '';
        }
        return this._renderResult[lineIndex];
    }
}
LineNumbersOverlay.CLASS_NAME = 'line-numbers';
// theming
registerThemingParticipant((theme, collector) => {
    const lineNumbers = theme.getColor(editorLineNumbers);
    if (lineNumbers) {
        collector.addRule(`.monaco-editor .line-numbers { color: ${lineNumbers}; }`);
    }
    const activeLineNumber = theme.getColor(editorActiveLineNumber);
    if (activeLineNumber) {
        collector.addRule(`.monaco-editor .line-numbers.active-line-number { color: ${activeLineNumber}; }`);
    }
});
