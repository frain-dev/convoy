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
import { MarkdownString } from '../../../../base/common/htmlContent.js';
import { DisposableStore } from '../../../../base/common/lifecycle.js';
import { MarkdownRenderer } from '../../markdownRenderer/browser/markdownRenderer.js';
import { Range } from '../../../common/core/range.js';
import { ILanguageService } from '../../../common/languages/language.js';
import { HoverForeignElementAnchor } from '../../hover/browser/hoverTypes.js';
import { GhostTextController, ShowNextInlineSuggestionAction, ShowPreviousInlineSuggestionAction } from './ghostTextController.js';
import * as nls from '../../../../nls.js';
import { IAccessibilityService } from '../../../../platform/accessibility/common/accessibility.js';
import { IMenuService, MenuId, MenuItemAction } from '../../../../platform/actions/common/actions.js';
import { ICommandService } from '../../../../platform/commands/common/commands.js';
import { IContextKeyService } from '../../../../platform/contextkey/common/contextkey.js';
import { IOpenerService } from '../../../../platform/opener/common/opener.js';
import { inlineSuggestCommitId } from './consts.js';
export class InlineCompletionsHover {
    constructor(owner, range, controller) {
        this.owner = owner;
        this.range = range;
        this.controller = controller;
    }
    isValidForHoverAnchor(anchor) {
        return (anchor.type === 1 /* HoverAnchorType.Range */
            && this.range.startColumn <= anchor.range.startColumn
            && this.range.endColumn >= anchor.range.endColumn);
    }
    hasMultipleSuggestions() {
        return this.controller.hasMultipleInlineCompletions();
    }
    get commands() {
        var _a, _b, _c;
        return ((_c = (_b = (_a = this.controller.activeModel) === null || _a === void 0 ? void 0 : _a.activeInlineCompletionsModel) === null || _b === void 0 ? void 0 : _b.completionSession.value) === null || _c === void 0 ? void 0 : _c.commands) || [];
    }
}
let InlineCompletionsHoverParticipant = class InlineCompletionsHoverParticipant {
    constructor(_editor, _commandService, _menuService, _contextKeyService, _languageService, _openerService, accessibilityService) {
        this._editor = _editor;
        this._commandService = _commandService;
        this._menuService = _menuService;
        this._contextKeyService = _contextKeyService;
        this._languageService = _languageService;
        this._openerService = _openerService;
        this.accessibilityService = accessibilityService;
        this.hoverOrdinal = 3;
    }
    suggestHoverAnchor(mouseEvent) {
        const controller = GhostTextController.get(this._editor);
        if (!controller) {
            return null;
        }
        const target = mouseEvent.target;
        if (target.type === 8 /* MouseTargetType.CONTENT_VIEW_ZONE */) {
            // handle the case where the mouse is over the view zone
            const viewZoneData = target.detail;
            if (controller.shouldShowHoverAtViewZone(viewZoneData.viewZoneId)) {
                return new HoverForeignElementAnchor(1000, this, Range.fromPositions(viewZoneData.positionBefore || viewZoneData.position, viewZoneData.positionBefore || viewZoneData.position));
            }
        }
        if (target.type === 7 /* MouseTargetType.CONTENT_EMPTY */) {
            // handle the case where the mouse is over the empty portion of a line following ghost text
            if (controller.shouldShowHoverAt(target.range)) {
                return new HoverForeignElementAnchor(1000, this, target.range);
            }
        }
        if (target.type === 6 /* MouseTargetType.CONTENT_TEXT */) {
            // handle the case where the mouse is directly over ghost text
            const mightBeForeignElement = target.detail.mightBeForeignElement;
            if (mightBeForeignElement && controller.shouldShowHoverAt(target.range)) {
                return new HoverForeignElementAnchor(1000, this, target.range);
            }
        }
        return null;
    }
    computeSync(anchor, lineDecorations) {
        const controller = GhostTextController.get(this._editor);
        if (controller && controller.shouldShowHoverAt(anchor.range)) {
            return [new InlineCompletionsHover(this, anchor.range, controller)];
        }
        return [];
    }
    renderHoverParts(context, hoverParts) {
        const disposableStore = new DisposableStore();
        const part = hoverParts[0];
        if (this.accessibilityService.isScreenReaderOptimized()) {
            this.renderScreenReaderText(context, part, disposableStore);
        }
        // TODO@hediet: deprecate MenuId.InlineCompletionsActions
        const menu = disposableStore.add(this._menuService.createMenu(MenuId.InlineCompletionsActions, this._contextKeyService));
        const previousAction = context.statusBar.addAction({
            label: nls.localize('showNextInlineSuggestion', "Next"),
            commandId: ShowNextInlineSuggestionAction.ID,
            run: () => this._commandService.executeCommand(ShowNextInlineSuggestionAction.ID)
        });
        const nextAction = context.statusBar.addAction({
            label: nls.localize('showPreviousInlineSuggestion', "Previous"),
            commandId: ShowPreviousInlineSuggestionAction.ID,
            run: () => this._commandService.executeCommand(ShowPreviousInlineSuggestionAction.ID)
        });
        context.statusBar.addAction({
            label: nls.localize('acceptInlineSuggestion', "Accept"),
            commandId: inlineSuggestCommitId,
            run: () => this._commandService.executeCommand(inlineSuggestCommitId)
        });
        const actions = [previousAction, nextAction];
        for (const action of actions) {
            action.setEnabled(false);
        }
        part.hasMultipleSuggestions().then(hasMore => {
            for (const action of actions) {
                action.setEnabled(hasMore);
            }
        });
        for (const command of part.commands) {
            context.statusBar.addAction({
                label: command.title,
                commandId: command.id,
                run: () => this._commandService.executeCommand(command.id, ...(command.arguments || []))
            });
        }
        for (const [_, group] of menu.getActions()) {
            for (const action of group) {
                if (action instanceof MenuItemAction) {
                    context.statusBar.addAction({
                        label: action.label,
                        commandId: action.item.id,
                        run: () => this._commandService.executeCommand(action.item.id)
                    });
                }
            }
        }
        return disposableStore;
    }
    renderScreenReaderText(context, part, disposableStore) {
        var _a, _b;
        const $ = dom.$;
        const markdownHoverElement = $('div.hover-row.markdown-hover');
        const hoverContentsElement = dom.append(markdownHoverElement, $('div.hover-contents'));
        const renderer = disposableStore.add(new MarkdownRenderer({ editor: this._editor }, this._languageService, this._openerService));
        const render = (code) => {
            disposableStore.add(renderer.onDidRenderAsync(() => {
                hoverContentsElement.className = 'hover-contents code-hover-contents';
                context.onContentsChanged();
            }));
            const inlineSuggestionAvailable = nls.localize('inlineSuggestionFollows', "Suggestion:");
            const renderedContents = disposableStore.add(renderer.render(new MarkdownString().appendText(inlineSuggestionAvailable).appendCodeblock('text', code)));
            hoverContentsElement.replaceChildren(renderedContents.element);
        };
        const ghostText = (_b = (_a = part.controller.activeModel) === null || _a === void 0 ? void 0 : _a.inlineCompletionsModel) === null || _b === void 0 ? void 0 : _b.ghostText;
        if (ghostText) {
            const lineText = this._editor.getModel().getLineContent(ghostText.lineNumber);
            render(ghostText.renderForScreenReader(lineText));
        }
        context.fragment.appendChild(markdownHoverElement);
    }
};
InlineCompletionsHoverParticipant = __decorate([
    __param(1, ICommandService),
    __param(2, IMenuService),
    __param(3, IContextKeyService),
    __param(4, ILanguageService),
    __param(5, IOpenerService),
    __param(6, IAccessibilityService)
], InlineCompletionsHoverParticipant);
export { InlineCompletionsHoverParticipant };
