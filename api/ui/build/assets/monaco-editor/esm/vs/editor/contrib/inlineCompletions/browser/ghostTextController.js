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
import { Emitter } from '../../../../base/common/event.js';
import { Disposable, MutableDisposable, toDisposable } from '../../../../base/common/lifecycle.js';
import { firstNonWhitespaceIndex } from '../../../../base/common/strings.js';
import { EditorAction } from '../../../browser/editorExtensions.js';
import { CursorColumns } from '../../../common/core/cursorColumns.js';
import { EditorContextKeys } from '../../../common/editorContextKeys.js';
import { GhostTextModel } from './ghostTextModel.js';
import { GhostTextWidget } from './ghostTextWidget.js';
import * as nls from '../../../../nls.js';
import { ContextKeyExpr, IContextKeyService, RawContextKey } from '../../../../platform/contextkey/common/contextkey.js';
import { IInstantiationService } from '../../../../platform/instantiation/common/instantiation.js';
let GhostTextController = class GhostTextController extends Disposable {
    constructor(editor, instantiationService) {
        super();
        this.editor = editor;
        this.instantiationService = instantiationService;
        this.triggeredExplicitly = false;
        this.activeController = this._register(new MutableDisposable());
        this.activeModelDidChangeEmitter = this._register(new Emitter());
        this._register(this.editor.onDidChangeModel(() => {
            this.updateModelController();
        }));
        this._register(this.editor.onDidChangeConfiguration((e) => {
            if (e.hasChanged(108 /* EditorOption.suggest */)) {
                this.updateModelController();
            }
            if (e.hasChanged(57 /* EditorOption.inlineSuggest */)) {
                this.updateModelController();
            }
        }));
        this.updateModelController();
    }
    static get(editor) {
        return editor.getContribution(GhostTextController.ID);
    }
    get activeModel() {
        var _a;
        return (_a = this.activeController.value) === null || _a === void 0 ? void 0 : _a.model;
    }
    // Don't call this method when not necessary. It will recreate the activeController.
    updateModelController() {
        const suggestOptions = this.editor.getOption(108 /* EditorOption.suggest */);
        const inlineSuggestOptions = this.editor.getOption(57 /* EditorOption.inlineSuggest */);
        this.activeController.value = undefined;
        // ActiveGhostTextController is only created if one of those settings is set or if the inline completions are triggered explicitly.
        this.activeController.value =
            this.editor.hasModel() && (suggestOptions.preview || inlineSuggestOptions.enabled || this.triggeredExplicitly)
                ? this.instantiationService.createInstance(ActiveGhostTextController, this.editor)
                : undefined;
        this.activeModelDidChangeEmitter.fire();
    }
    shouldShowHoverAt(hoverRange) {
        var _a;
        return ((_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.shouldShowHoverAt(hoverRange)) || false;
    }
    shouldShowHoverAtViewZone(viewZoneId) {
        var _a, _b;
        return ((_b = (_a = this.activeController.value) === null || _a === void 0 ? void 0 : _a.widget) === null || _b === void 0 ? void 0 : _b.shouldShowHoverAtViewZone(viewZoneId)) || false;
    }
    trigger() {
        var _a;
        this.triggeredExplicitly = true;
        if (!this.activeController.value) {
            this.updateModelController();
        }
        (_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.triggerInlineCompletion();
    }
    commit() {
        var _a;
        (_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.commitInlineCompletion();
    }
    hide() {
        var _a;
        (_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.hideInlineCompletion();
    }
    showNextInlineCompletion() {
        var _a;
        (_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.showNextInlineCompletion();
    }
    showPreviousInlineCompletion() {
        var _a;
        (_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.showPreviousInlineCompletion();
    }
    hasMultipleInlineCompletions() {
        var _a;
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield ((_a = this.activeModel) === null || _a === void 0 ? void 0 : _a.hasMultipleInlineCompletions());
            return result !== undefined ? result : false;
        });
    }
};
GhostTextController.inlineSuggestionVisible = new RawContextKey('inlineSuggestionVisible', false, nls.localize('inlineSuggestionVisible', "Whether an inline suggestion is visible"));
GhostTextController.inlineSuggestionHasIndentation = new RawContextKey('inlineSuggestionHasIndentation', false, nls.localize('inlineSuggestionHasIndentation', "Whether the inline suggestion starts with whitespace"));
GhostTextController.inlineSuggestionHasIndentationLessThanTabSize = new RawContextKey('inlineSuggestionHasIndentationLessThanTabSize', true, nls.localize('inlineSuggestionHasIndentationLessThanTabSize', "Whether the inline suggestion starts with whitespace that is less than what would be inserted by tab"));
GhostTextController.ID = 'editor.contrib.ghostTextController';
GhostTextController = __decorate([
    __param(1, IInstantiationService)
], GhostTextController);
export { GhostTextController };
class GhostTextContextKeys {
    constructor(contextKeyService) {
        this.contextKeyService = contextKeyService;
        this.inlineCompletionVisible = GhostTextController.inlineSuggestionVisible.bindTo(this.contextKeyService);
        this.inlineCompletionSuggestsIndentation = GhostTextController.inlineSuggestionHasIndentation.bindTo(this.contextKeyService);
        this.inlineCompletionSuggestsIndentationLessThanTabSize = GhostTextController.inlineSuggestionHasIndentationLessThanTabSize.bindTo(this.contextKeyService);
    }
}
/**
 * The controller for a text editor with an initialized text model.
 * Must be disposed as soon as the model detaches from the editor.
*/
let ActiveGhostTextController = class ActiveGhostTextController extends Disposable {
    constructor(editor, instantiationService, contextKeyService) {
        super();
        this.editor = editor;
        this.instantiationService = instantiationService;
        this.contextKeyService = contextKeyService;
        this.contextKeys = new GhostTextContextKeys(this.contextKeyService);
        this.model = this._register(this.instantiationService.createInstance(GhostTextModel, this.editor));
        this.widget = this._register(this.instantiationService.createInstance(GhostTextWidget, this.editor, this.model));
        this._register(toDisposable(() => {
            this.contextKeys.inlineCompletionVisible.set(false);
            this.contextKeys.inlineCompletionSuggestsIndentation.set(false);
            this.contextKeys.inlineCompletionSuggestsIndentationLessThanTabSize.set(true);
        }));
        this._register(this.model.onDidChange(() => {
            this.updateContextKeys();
        }));
        this.updateContextKeys();
    }
    updateContextKeys() {
        var _a;
        this.contextKeys.inlineCompletionVisible.set(((_a = this.model.activeInlineCompletionsModel) === null || _a === void 0 ? void 0 : _a.ghostText) !== undefined);
        let startsWithIndentation = false;
        let startsWithIndentationLessThanTabSize = true;
        const ghostText = this.model.inlineCompletionsModel.ghostText;
        if (!!this.model.activeInlineCompletionsModel && ghostText && ghostText.parts.length > 0) {
            const { column, lines } = ghostText.parts[0];
            const firstLine = lines[0];
            const indentationEndColumn = this.editor.getModel().getLineIndentColumn(ghostText.lineNumber);
            const inIndentation = column <= indentationEndColumn;
            if (inIndentation) {
                let firstNonWsIdx = firstNonWhitespaceIndex(firstLine);
                if (firstNonWsIdx === -1) {
                    firstNonWsIdx = firstLine.length - 1;
                }
                startsWithIndentation = firstNonWsIdx > 0;
                const tabSize = this.editor.getModel().getOptions().tabSize;
                const visibleColumnIndentation = CursorColumns.visibleColumnFromColumn(firstLine, firstNonWsIdx + 1, tabSize);
                startsWithIndentationLessThanTabSize = visibleColumnIndentation < tabSize;
            }
        }
        this.contextKeys.inlineCompletionSuggestsIndentation.set(startsWithIndentation);
        this.contextKeys.inlineCompletionSuggestsIndentationLessThanTabSize.set(startsWithIndentationLessThanTabSize);
    }
};
ActiveGhostTextController = __decorate([
    __param(1, IInstantiationService),
    __param(2, IContextKeyService)
], ActiveGhostTextController);
export { ActiveGhostTextController };
export class ShowNextInlineSuggestionAction extends EditorAction {
    constructor() {
        super({
            id: ShowNextInlineSuggestionAction.ID,
            label: nls.localize('action.inlineSuggest.showNext', "Show Next Inline Suggestion"),
            alias: 'Show Next Inline Suggestion',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, GhostTextController.inlineSuggestionVisible),
            kbOpts: {
                weight: 100,
                primary: 512 /* KeyMod.Alt */ | 89 /* KeyCode.BracketRight */,
            },
        });
    }
    run(accessor, editor) {
        return __awaiter(this, void 0, void 0, function* () {
            const controller = GhostTextController.get(editor);
            if (controller) {
                controller.showNextInlineCompletion();
                editor.focus();
            }
        });
    }
}
ShowNextInlineSuggestionAction.ID = 'editor.action.inlineSuggest.showNext';
export class ShowPreviousInlineSuggestionAction extends EditorAction {
    constructor() {
        super({
            id: ShowPreviousInlineSuggestionAction.ID,
            label: nls.localize('action.inlineSuggest.showPrevious', "Show Previous Inline Suggestion"),
            alias: 'Show Previous Inline Suggestion',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, GhostTextController.inlineSuggestionVisible),
            kbOpts: {
                weight: 100,
                primary: 512 /* KeyMod.Alt */ | 87 /* KeyCode.BracketLeft */,
            },
        });
    }
    run(accessor, editor) {
        return __awaiter(this, void 0, void 0, function* () {
            const controller = GhostTextController.get(editor);
            if (controller) {
                controller.showPreviousInlineCompletion();
                editor.focus();
            }
        });
    }
}
ShowPreviousInlineSuggestionAction.ID = 'editor.action.inlineSuggest.showPrevious';
export class TriggerInlineSuggestionAction extends EditorAction {
    constructor() {
        super({
            id: 'editor.action.inlineSuggest.trigger',
            label: nls.localize('action.inlineSuggest.trigger', "Trigger Inline Suggestion"),
            alias: 'Trigger Inline Suggestion',
            precondition: EditorContextKeys.writable
        });
    }
    run(accessor, editor) {
        return __awaiter(this, void 0, void 0, function* () {
            const controller = GhostTextController.get(editor);
            if (controller) {
                controller.trigger();
            }
        });
    }
}
