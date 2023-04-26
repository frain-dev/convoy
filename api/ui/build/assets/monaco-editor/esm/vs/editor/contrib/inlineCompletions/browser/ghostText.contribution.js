/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { EditorCommand, registerEditorAction, registerEditorCommand, registerEditorContribution } from '../../../browser/editorExtensions.js';
import { EditorContextKeys } from '../../../common/editorContextKeys.js';
import { HoverParticipantRegistry } from '../../hover/browser/hoverTypes.js';
import { inlineSuggestCommitId } from './consts.js';
import { GhostTextController, ShowNextInlineSuggestionAction, ShowPreviousInlineSuggestionAction, TriggerInlineSuggestionAction } from './ghostTextController.js';
import { InlineCompletionsHoverParticipant } from './ghostTextHoverParticipant.js';
import { ContextKeyExpr } from '../../../../platform/contextkey/common/contextkey.js';
import { KeybindingsRegistry } from '../../../../platform/keybinding/common/keybindingsRegistry.js';
registerEditorContribution(GhostTextController.ID, GhostTextController);
registerEditorAction(TriggerInlineSuggestionAction);
registerEditorAction(ShowNextInlineSuggestionAction);
registerEditorAction(ShowPreviousInlineSuggestionAction);
HoverParticipantRegistry.register(InlineCompletionsHoverParticipant);
const GhostTextCommand = EditorCommand.bindToContribution(GhostTextController.get);
export const commitInlineSuggestionAction = new GhostTextCommand({
    id: inlineSuggestCommitId,
    precondition: GhostTextController.inlineSuggestionVisible,
    handler(x) {
        x.commit();
        x.editor.focus();
    }
});
registerEditorCommand(commitInlineSuggestionAction);
KeybindingsRegistry.registerKeybindingRule({
    primary: 2 /* KeyCode.Tab */,
    weight: 200,
    id: commitInlineSuggestionAction.id,
    when: ContextKeyExpr.and(commitInlineSuggestionAction.precondition, EditorContextKeys.tabMovesFocus.toNegated(), GhostTextController.inlineSuggestionHasIndentationLessThanTabSize),
});
registerEditorCommand(new GhostTextCommand({
    id: 'editor.action.inlineSuggest.hide',
    precondition: GhostTextController.inlineSuggestionVisible,
    kbOpts: {
        weight: 100,
        primary: 9 /* KeyCode.Escape */,
    },
    handler(x) {
        x.hide();
    }
}));
