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
import { alert } from '../../../../base/browser/ui/aria/aria.js';
import { TimeoutTimer } from '../../../../base/common/async.js';
import { DisposableStore, MutableDisposable } from '../../../../base/common/lifecycle.js';
import './messageController.css';
import { EditorCommand, registerEditorCommand, registerEditorContribution } from '../../../browser/editorExtensions.js';
import { Range } from '../../../common/core/range.js';
import * as nls from '../../../../nls.js';
import { IContextKeyService, RawContextKey } from '../../../../platform/contextkey/common/contextkey.js';
let MessageController = class MessageController {
    constructor(editor, contextKeyService) {
        this._messageWidget = new MutableDisposable();
        this._messageListeners = new DisposableStore();
        this._editor = editor;
        this._visible = MessageController.MESSAGE_VISIBLE.bindTo(contextKeyService);
    }
    static get(editor) {
        return editor.getContribution(MessageController.ID);
    }
    dispose() {
        this._messageListeners.dispose();
        this._messageWidget.dispose();
        this._visible.reset();
    }
    showMessage(message, position) {
        alert(message);
        this._visible.set(true);
        this._messageWidget.clear();
        this._messageListeners.clear();
        this._messageWidget.value = new MessageWidget(this._editor, position, message);
        // close on blur, cursor, model change, dispose
        this._messageListeners.add(this._editor.onDidBlurEditorText(() => this.closeMessage()));
        this._messageListeners.add(this._editor.onDidChangeCursorPosition(() => this.closeMessage()));
        this._messageListeners.add(this._editor.onDidDispose(() => this.closeMessage()));
        this._messageListeners.add(this._editor.onDidChangeModel(() => this.closeMessage()));
        // 3sec
        this._messageListeners.add(new TimeoutTimer(() => this.closeMessage(), 3000));
        // close on mouse move
        let bounds;
        this._messageListeners.add(this._editor.onMouseMove(e => {
            // outside the text area
            if (!e.target.position) {
                return;
            }
            if (!bounds) {
                // define bounding box around position and first mouse occurance
                bounds = new Range(position.lineNumber - 3, 1, e.target.position.lineNumber + 3, 1);
            }
            else if (!bounds.containsPosition(e.target.position)) {
                // check if position is still in bounds
                this.closeMessage();
            }
        }));
    }
    closeMessage() {
        this._visible.reset();
        this._messageListeners.clear();
        if (this._messageWidget.value) {
            this._messageListeners.add(MessageWidget.fadeOut(this._messageWidget.value));
        }
    }
};
MessageController.ID = 'editor.contrib.messageController';
MessageController.MESSAGE_VISIBLE = new RawContextKey('messageVisible', false, nls.localize('messageVisible', 'Whether the editor is currently showing an inline message'));
MessageController = __decorate([
    __param(1, IContextKeyService)
], MessageController);
export { MessageController };
const MessageCommand = EditorCommand.bindToContribution(MessageController.get);
registerEditorCommand(new MessageCommand({
    id: 'leaveEditorMessage',
    precondition: MessageController.MESSAGE_VISIBLE,
    handler: c => c.closeMessage(),
    kbOpts: {
        weight: 100 /* KeybindingWeight.EditorContrib */ + 30,
        primary: 9 /* KeyCode.Escape */
    }
}));
class MessageWidget {
    constructor(editor, { lineNumber, column }, text) {
        // Editor.IContentWidget.allowEditorOverflow
        this.allowEditorOverflow = true;
        this.suppressMouseDown = false;
        this._editor = editor;
        this._editor.revealLinesInCenterIfOutsideViewport(lineNumber, lineNumber, 0 /* ScrollType.Smooth */);
        this._position = { lineNumber, column };
        this._domNode = document.createElement('div');
        this._domNode.classList.add('monaco-editor-overlaymessage');
        this._domNode.style.marginLeft = '-6px';
        const anchorTop = document.createElement('div');
        anchorTop.classList.add('anchor', 'top');
        this._domNode.appendChild(anchorTop);
        const message = document.createElement('div');
        message.classList.add('message');
        message.textContent = text;
        this._domNode.appendChild(message);
        const anchorBottom = document.createElement('div');
        anchorBottom.classList.add('anchor', 'below');
        this._domNode.appendChild(anchorBottom);
        this._editor.addContentWidget(this);
        this._domNode.classList.add('fadeIn');
    }
    static fadeOut(messageWidget) {
        const dispose = () => {
            messageWidget.dispose();
            clearTimeout(handle);
            messageWidget.getDomNode().removeEventListener('animationend', dispose);
        };
        const handle = setTimeout(dispose, 110);
        messageWidget.getDomNode().addEventListener('animationend', dispose);
        messageWidget.getDomNode().classList.add('fadeOut');
        return { dispose };
    }
    dispose() {
        this._editor.removeContentWidget(this);
    }
    getId() {
        return 'messageoverlay';
    }
    getDomNode() {
        return this._domNode;
    }
    getPosition() {
        return {
            position: this._position,
            preference: [
                1 /* ContentWidgetPositionPreference.ABOVE */,
                2 /* ContentWidgetPositionPreference.BELOW */,
            ],
            positionAffinity: 1 /* PositionAffinity.Right */,
        };
    }
    afterRender(position) {
        this._domNode.classList.toggle('below', position === 2 /* ContentWidgetPositionPreference.BELOW */);
    }
}
registerEditorContribution(MessageController.ID, MessageController);
