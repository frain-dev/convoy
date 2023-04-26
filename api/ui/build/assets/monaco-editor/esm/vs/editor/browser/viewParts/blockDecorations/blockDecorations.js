/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { createFastDomNode } from '../../../../base/browser/fastDomNode.js';
import './blockDecorations.css';
import { ViewPart } from '../../view/viewPart.js';
export class BlockDecorations extends ViewPart {
    constructor(context) {
        super(context);
        this.blocks = [];
        this.contentWidth = -1;
        this.domNode = createFastDomNode(document.createElement('div'));
        this.domNode.setAttribute('role', 'presentation');
        this.domNode.setAttribute('aria-hidden', 'true');
        this.domNode.setClassName('blockDecorations-container');
        this.update();
    }
    update() {
        let didChange = false;
        const options = this._context.configuration.options;
        const layoutInfo = options.get(133 /* EditorOption.layoutInfo */);
        const newContentWidth = layoutInfo.contentWidth - layoutInfo.verticalScrollbarWidth;
        if (this.contentWidth !== newContentWidth) {
            this.contentWidth = newContentWidth;
            didChange = true;
        }
        return didChange;
    }
    dispose() {
        super.dispose();
    }
    // --- begin event handlers
    onConfigurationChanged(e) {
        return this.update();
    }
    onScrollChanged(e) {
        return e.scrollTopChanged || e.scrollLeftChanged;
    }
    onDecorationsChanged(e) {
        return true;
    }
    onZonesChanged(e) {
        return true;
    }
    // --- end event handlers
    prepareRender(ctx) {
        // Nothing to read
    }
    render(ctx) {
        let count = 0;
        const decorations = ctx.getDecorationsInViewport();
        for (const decoration of decorations) {
            if (!decoration.options.blockClassName) {
                continue;
            }
            let block = this.blocks[count];
            if (!block) {
                block = this.blocks[count] = createFastDomNode(document.createElement('div'));
                this.domNode.appendChild(block);
            }
            const top = ctx.getVerticalOffsetForLineNumber(decoration.range.startLineNumber);
            // See https://github.com/microsoft/vscode/pull/152740#discussion_r902661546
            const bottom = ctx.getVerticalOffsetForLineNumber(decoration.range.endLineNumber + 1);
            block.setClassName('blockDecorations-block ' + decoration.options.blockClassName);
            block.setLeft(ctx.scrollLeft);
            block.setWidth(this.contentWidth);
            block.setTop(top);
            block.setHeight(bottom - top);
            count++;
        }
        for (let i = count; i < this.blocks.length; i++) {
            this.blocks[i].domNode.remove();
        }
        this.blocks.length = count;
    }
}
