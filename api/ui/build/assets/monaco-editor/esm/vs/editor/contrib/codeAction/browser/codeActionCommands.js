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
import { CancellationToken } from '../../../../base/common/cancellation.js';
import { Lazy } from '../../../../base/common/lazy.js';
import { Disposable } from '../../../../base/common/lifecycle.js';
import { escapeRegExpCharacters } from '../../../../base/common/strings.js';
import { EditorAction, EditorCommand, registerEditorCommand } from '../../../browser/editorExtensions.js';
import { IBulkEditService, ResourceEdit } from '../../../browser/services/bulkEditService.js';
import { EditorContextKeys } from '../../../common/editorContextKeys.js';
import { ILanguageFeaturesService } from '../../../common/services/languageFeatures.js';
import { codeActionCommandId, fixAllCommandId, organizeImportsCommandId, refactorCommandId, refactorPreviewCommandId, sourceActionCommandId } from './codeAction.js';
import { CodeActionUi } from './codeActionUi.js';
import { MessageController } from '../../message/browser/messageController.js';
import * as nls from '../../../../nls.js';
import { ICommandService } from '../../../../platform/commands/common/commands.js';
import { ContextKeyExpr, IContextKeyService } from '../../../../platform/contextkey/common/contextkey.js';
import { IInstantiationService } from '../../../../platform/instantiation/common/instantiation.js';
import { IMarkerService } from '../../../../platform/markers/common/markers.js';
import { IEditorProgressService } from '../../../../platform/progress/common/progress.js';
import { INotificationService } from '../../../../platform/notification/common/notification.js';
import { ITelemetryService } from '../../../../platform/telemetry/common/telemetry.js';
import { CodeActionModel, SUPPORTED_CODE_ACTIONS } from './codeActionModel.js';
import { CodeActionCommandArgs, CodeActionKind, CodeActionTriggerSource } from './types.js';
import { Context } from './codeActionMenu.js';
function contextKeyForSupportedActions(kind) {
    return ContextKeyExpr.regex(SUPPORTED_CODE_ACTIONS.keys()[0], new RegExp('(\\s|^)' + escapeRegExpCharacters(kind.value) + '\\b'));
}
function refactorTrigger(editor, userArgs, preview, codeActionFrom) {
    const args = CodeActionCommandArgs.fromUser(userArgs, {
        kind: CodeActionKind.Refactor,
        apply: "never" /* CodeActionAutoApply.Never */
    });
    return triggerCodeActionsForEditorSelection(editor, typeof (userArgs === null || userArgs === void 0 ? void 0 : userArgs.kind) === 'string'
        ? args.preferred
            ? nls.localize('editor.action.refactor.noneMessage.preferred.kind', "No preferred refactorings for '{0}' available", userArgs.kind)
            : nls.localize('editor.action.refactor.noneMessage.kind', "No refactorings for '{0}' available", userArgs.kind)
        : args.preferred
            ? nls.localize('editor.action.refactor.noneMessage.preferred', "No preferred refactorings available")
            : nls.localize('editor.action.refactor.noneMessage', "No refactorings available"), {
        include: CodeActionKind.Refactor.contains(args.kind) ? args.kind : CodeActionKind.None,
        onlyIncludePreferredActions: args.preferred
    }, args.apply, preview, codeActionFrom);
}
const argsSchema = {
    type: 'object',
    defaultSnippets: [{ body: { kind: '' } }],
    properties: {
        'kind': {
            type: 'string',
            description: nls.localize('args.schema.kind', "Kind of the code action to run."),
        },
        'apply': {
            type: 'string',
            description: nls.localize('args.schema.apply', "Controls when the returned actions are applied."),
            default: "ifSingle" /* CodeActionAutoApply.IfSingle */,
            enum: ["first" /* CodeActionAutoApply.First */, "ifSingle" /* CodeActionAutoApply.IfSingle */, "never" /* CodeActionAutoApply.Never */],
            enumDescriptions: [
                nls.localize('args.schema.apply.first', "Always apply the first returned code action."),
                nls.localize('args.schema.apply.ifSingle', "Apply the first returned code action if it is the only one."),
                nls.localize('args.schema.apply.never', "Do not apply the returned code actions."),
            ]
        },
        'preferred': {
            type: 'boolean',
            default: false,
            description: nls.localize('args.schema.preferred', "Controls if only preferred code actions should be returned."),
        }
    }
};
let QuickFixController = class QuickFixController extends Disposable {
    constructor(editor, markerService, contextKeyService, progressService, _instantiationService, languageFeaturesService) {
        super();
        this._instantiationService = _instantiationService;
        this._editor = editor;
        this._model = this._register(new CodeActionModel(this._editor, languageFeaturesService.codeActionProvider, markerService, contextKeyService, progressService));
        this._register(this._model.onDidChangeState(newState => this.update(newState)));
        this._ui = new Lazy(() => this._register(new CodeActionUi(editor, QuickFixAction.Id, AutoFixAction.Id, {
            applyCodeAction: (action, retrigger, preview) => __awaiter(this, void 0, void 0, function* () {
                try {
                    yield this._applyCodeAction(action, preview);
                }
                finally {
                    if (retrigger) {
                        this._trigger({ type: 2 /* CodeActionTriggerType.Auto */, triggerAction: CodeActionTriggerSource.QuickFix, filter: {} });
                    }
                }
            })
        }, this._instantiationService)));
    }
    static get(editor) {
        return editor.getContribution(QuickFixController.ID);
    }
    update(newState) {
        this._ui.getValue().update(newState);
    }
    hideCodeActionMenu() {
        if (this._ui.hasValue()) {
            this._ui.getValue().hideCodeActionWidget();
        }
    }
    navigateCodeActionList(navUp) {
        if (this._ui.hasValue()) {
            this._ui.getValue().navigateList(navUp);
        }
    }
    selectedOption() {
        if (this._ui.hasValue()) {
            this._ui.getValue().onEnter();
        }
    }
    selectedOptionWithPreview() {
        if (this._ui.hasValue()) {
            this._ui.getValue().onPreviewEnter();
        }
    }
    showCodeActions(trigger, actions, at) {
        return this._ui.getValue().showCodeActionList(trigger, actions, at, { includeDisabledActions: false, fromLightbulb: false });
    }
    manualTriggerAtCurrentPosition(notAvailableMessage, triggerAction, filter, autoApply, preview) {
        var _a;
        if (!this._editor.hasModel()) {
            return;
        }
        (_a = MessageController.get(this._editor)) === null || _a === void 0 ? void 0 : _a.closeMessage();
        const triggerPosition = this._editor.getPosition();
        this._trigger({ type: 1 /* CodeActionTriggerType.Invoke */, triggerAction, filter, autoApply, context: { notAvailableMessage, position: triggerPosition }, preview });
    }
    _trigger(trigger) {
        return this._model.trigger(trigger);
    }
    _applyCodeAction(action, preview) {
        return this._instantiationService.invokeFunction(applyCodeAction, action, ApplyCodeActionReason.FromCodeActions, { preview, editor: this._editor });
    }
};
QuickFixController.ID = 'editor.contrib.quickFixController';
QuickFixController = __decorate([
    __param(1, IMarkerService),
    __param(2, IContextKeyService),
    __param(3, IEditorProgressService),
    __param(4, IInstantiationService),
    __param(5, ILanguageFeaturesService)
], QuickFixController);
export { QuickFixController };
export var ApplyCodeActionReason;
(function (ApplyCodeActionReason) {
    ApplyCodeActionReason["OnSave"] = "onSave";
    ApplyCodeActionReason["FromProblemsView"] = "fromProblemsView";
    ApplyCodeActionReason["FromCodeActions"] = "fromCodeActions";
})(ApplyCodeActionReason || (ApplyCodeActionReason = {}));
export function applyCodeAction(accessor, item, codeActionReason, options) {
    return __awaiter(this, void 0, void 0, function* () {
        const bulkEditService = accessor.get(IBulkEditService);
        const commandService = accessor.get(ICommandService);
        const telemetryService = accessor.get(ITelemetryService);
        const notificationService = accessor.get(INotificationService);
        telemetryService.publicLog2('codeAction.applyCodeAction', {
            codeActionTitle: item.action.title,
            codeActionKind: item.action.kind,
            codeActionIsPreferred: !!item.action.isPreferred,
            reason: codeActionReason,
        });
        yield item.resolve(CancellationToken.None);
        if (item.action.edit) {
            yield bulkEditService.apply(ResourceEdit.convert(item.action.edit), {
                editor: options === null || options === void 0 ? void 0 : options.editor,
                label: item.action.title,
                quotableLabel: item.action.title,
                code: 'undoredo.codeAction',
                respectAutoSaveConfig: true,
                showPreview: options === null || options === void 0 ? void 0 : options.preview,
            });
        }
        if (item.action.command) {
            try {
                yield commandService.executeCommand(item.action.command.id, ...(item.action.command.arguments || []));
            }
            catch (err) {
                const message = asMessage(err);
                notificationService.error(typeof message === 'string'
                    ? message
                    : nls.localize('applyCodeActionFailed', "An unknown error occurred while applying the code action"));
            }
        }
    });
}
function asMessage(err) {
    if (typeof err === 'string') {
        return err;
    }
    else if (err instanceof Error && typeof err.message === 'string') {
        return err.message;
    }
    else {
        return undefined;
    }
}
function triggerCodeActionsForEditorSelection(editor, notAvailableMessage, filter, autoApply, preview = false, triggerAction = CodeActionTriggerSource.Default) {
    if (editor.hasModel()) {
        const controller = QuickFixController.get(editor);
        controller === null || controller === void 0 ? void 0 : controller.manualTriggerAtCurrentPosition(notAvailableMessage, triggerAction, filter, autoApply, preview);
    }
}
export class QuickFixAction extends EditorAction {
    constructor() {
        super({
            id: QuickFixAction.Id,
            label: nls.localize('quickfix.trigger.label', "Quick Fix..."),
            alias: 'Quick Fix...',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, EditorContextKeys.hasCodeActionsProvider),
            kbOpts: {
                kbExpr: EditorContextKeys.editorTextFocus,
                primary: 2048 /* KeyMod.CtrlCmd */ | 84 /* KeyCode.Period */,
                weight: 100 /* KeybindingWeight.EditorContrib */
            }
        });
    }
    run(_accessor, editor) {
        return triggerCodeActionsForEditorSelection(editor, nls.localize('editor.action.quickFix.noneMessage', "No code actions available"), undefined, undefined, false, CodeActionTriggerSource.QuickFix);
    }
}
QuickFixAction.Id = 'editor.action.quickFix';
export class CodeActionCommand extends EditorCommand {
    constructor() {
        super({
            id: codeActionCommandId,
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, EditorContextKeys.hasCodeActionsProvider),
            description: {
                description: 'Trigger a code action',
                args: [{ name: 'args', schema: argsSchema, }]
            }
        });
    }
    runEditorCommand(_accessor, editor, userArgs) {
        const args = CodeActionCommandArgs.fromUser(userArgs, {
            kind: CodeActionKind.Empty,
            apply: "ifSingle" /* CodeActionAutoApply.IfSingle */,
        });
        return triggerCodeActionsForEditorSelection(editor, typeof (userArgs === null || userArgs === void 0 ? void 0 : userArgs.kind) === 'string'
            ? args.preferred
                ? nls.localize('editor.action.codeAction.noneMessage.preferred.kind', "No preferred code actions for '{0}' available", userArgs.kind)
                : nls.localize('editor.action.codeAction.noneMessage.kind', "No code actions for '{0}' available", userArgs.kind)
            : args.preferred
                ? nls.localize('editor.action.codeAction.noneMessage.preferred', "No preferred code actions available")
                : nls.localize('editor.action.codeAction.noneMessage', "No code actions available"), {
            include: args.kind,
            includeSourceActions: true,
            onlyIncludePreferredActions: args.preferred,
        }, args.apply);
    }
}
export class RefactorAction extends EditorAction {
    constructor() {
        super({
            id: refactorCommandId,
            label: nls.localize('refactor.label', "Refactor..."),
            alias: 'Refactor...',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, EditorContextKeys.hasCodeActionsProvider),
            kbOpts: {
                kbExpr: EditorContextKeys.editorTextFocus,
                primary: 2048 /* KeyMod.CtrlCmd */ | 1024 /* KeyMod.Shift */ | 48 /* KeyCode.KeyR */,
                mac: {
                    primary: 256 /* KeyMod.WinCtrl */ | 1024 /* KeyMod.Shift */ | 48 /* KeyCode.KeyR */
                },
                weight: 100 /* KeybindingWeight.EditorContrib */
            },
            contextMenuOpts: {
                group: '1_modification',
                order: 2,
                when: ContextKeyExpr.and(EditorContextKeys.writable, contextKeyForSupportedActions(CodeActionKind.Refactor)),
            },
            description: {
                description: 'Refactor...',
                args: [{ name: 'args', schema: argsSchema }]
            }
        });
    }
    run(_accessor, editor, userArgs) {
        return refactorTrigger(editor, userArgs, false, CodeActionTriggerSource.Refactor);
    }
}
export class RefactorPreview extends EditorAction {
    constructor() {
        super({
            id: refactorPreviewCommandId,
            label: nls.localize('refactor.preview.label', "Refactor with Preview..."),
            alias: 'Refactor Preview...',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, EditorContextKeys.hasCodeActionsProvider),
            description: {
                description: 'Refactor Preview...',
                args: [{ name: 'args', schema: argsSchema }]
            }
        });
    }
    run(_accessor, editor, userArgs) {
        return refactorTrigger(editor, userArgs, true, CodeActionTriggerSource.RefactorPreview);
    }
}
export class SourceAction extends EditorAction {
    constructor() {
        super({
            id: sourceActionCommandId,
            label: nls.localize('source.label', "Source Action..."),
            alias: 'Source Action...',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, EditorContextKeys.hasCodeActionsProvider),
            contextMenuOpts: {
                group: '1_modification',
                order: 2.1,
                when: ContextKeyExpr.and(EditorContextKeys.writable, contextKeyForSupportedActions(CodeActionKind.Source)),
            },
            description: {
                description: 'Source Action...',
                args: [{ name: 'args', schema: argsSchema }]
            }
        });
    }
    run(_accessor, editor, userArgs) {
        const args = CodeActionCommandArgs.fromUser(userArgs, {
            kind: CodeActionKind.Source,
            apply: "never" /* CodeActionAutoApply.Never */
        });
        return triggerCodeActionsForEditorSelection(editor, typeof (userArgs === null || userArgs === void 0 ? void 0 : userArgs.kind) === 'string'
            ? args.preferred
                ? nls.localize('editor.action.source.noneMessage.preferred.kind', "No preferred source actions for '{0}' available", userArgs.kind)
                : nls.localize('editor.action.source.noneMessage.kind', "No source actions for '{0}' available", userArgs.kind)
            : args.preferred
                ? nls.localize('editor.action.source.noneMessage.preferred', "No preferred source actions available")
                : nls.localize('editor.action.source.noneMessage', "No source actions available"), {
            include: CodeActionKind.Source.contains(args.kind) ? args.kind : CodeActionKind.None,
            includeSourceActions: true,
            onlyIncludePreferredActions: args.preferred,
        }, args.apply, undefined, CodeActionTriggerSource.SourceAction);
    }
}
export class OrganizeImportsAction extends EditorAction {
    constructor() {
        super({
            id: organizeImportsCommandId,
            label: nls.localize('organizeImports.label', "Organize Imports"),
            alias: 'Organize Imports',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, contextKeyForSupportedActions(CodeActionKind.SourceOrganizeImports)),
            kbOpts: {
                kbExpr: EditorContextKeys.editorTextFocus,
                primary: 1024 /* KeyMod.Shift */ | 512 /* KeyMod.Alt */ | 45 /* KeyCode.KeyO */,
                weight: 100 /* KeybindingWeight.EditorContrib */
            },
        });
    }
    run(_accessor, editor) {
        return triggerCodeActionsForEditorSelection(editor, nls.localize('editor.action.organize.noneMessage', "No organize imports action available"), { include: CodeActionKind.SourceOrganizeImports, includeSourceActions: true }, "ifSingle" /* CodeActionAutoApply.IfSingle */, undefined, CodeActionTriggerSource.OrganizeImports);
    }
}
export class FixAllAction extends EditorAction {
    constructor() {
        super({
            id: fixAllCommandId,
            label: nls.localize('fixAll.label', "Fix All"),
            alias: 'Fix All',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, contextKeyForSupportedActions(CodeActionKind.SourceFixAll))
        });
    }
    run(_accessor, editor) {
        return triggerCodeActionsForEditorSelection(editor, nls.localize('fixAll.noneMessage', "No fix all action available"), { include: CodeActionKind.SourceFixAll, includeSourceActions: true }, "ifSingle" /* CodeActionAutoApply.IfSingle */, undefined, CodeActionTriggerSource.FixAll);
    }
}
export class AutoFixAction extends EditorAction {
    constructor() {
        super({
            id: AutoFixAction.Id,
            label: nls.localize('autoFix.label', "Auto Fix..."),
            alias: 'Auto Fix...',
            precondition: ContextKeyExpr.and(EditorContextKeys.writable, contextKeyForSupportedActions(CodeActionKind.QuickFix)),
            kbOpts: {
                kbExpr: EditorContextKeys.editorTextFocus,
                primary: 512 /* KeyMod.Alt */ | 1024 /* KeyMod.Shift */ | 84 /* KeyCode.Period */,
                mac: {
                    primary: 2048 /* KeyMod.CtrlCmd */ | 512 /* KeyMod.Alt */ | 84 /* KeyCode.Period */
                },
                weight: 100 /* KeybindingWeight.EditorContrib */
            }
        });
    }
    run(_accessor, editor) {
        return triggerCodeActionsForEditorSelection(editor, nls.localize('editor.action.autoFix.noneMessage', "No auto fixes available"), {
            include: CodeActionKind.QuickFix,
            onlyIncludePreferredActions: true
        }, "ifSingle" /* CodeActionAutoApply.IfSingle */, undefined, CodeActionTriggerSource.AutoFix);
    }
}
AutoFixAction.Id = 'editor.action.autoFix';
const CodeActionContribution = EditorCommand.bindToContribution(QuickFixController.get);
const weight = 100 /* KeybindingWeight.EditorContrib */ + 90;
registerEditorCommand(new CodeActionContribution({
    id: 'hideCodeActionMenuWidget',
    precondition: Context.Visible,
    handler(x) {
        x.hideCodeActionMenu();
    },
    kbOpts: {
        weight: weight,
        primary: 9 /* KeyCode.Escape */,
        secondary: [1024 /* KeyMod.Shift */ | 9 /* KeyCode.Escape */]
    }
}));
registerEditorCommand(new CodeActionContribution({
    id: 'focusPreviousCodeAction',
    precondition: Context.Visible,
    handler(x) {
        x.navigateCodeActionList(true);
    },
    kbOpts: {
        weight: weight + 100000,
        primary: 16 /* KeyCode.UpArrow */,
        secondary: [2048 /* KeyMod.CtrlCmd */ | 16 /* KeyCode.UpArrow */],
    }
}));
registerEditorCommand(new CodeActionContribution({
    id: 'focusNextCodeAction',
    precondition: Context.Visible,
    handler(x) {
        x.navigateCodeActionList(false);
    },
    kbOpts: {
        weight: weight + 100000,
        primary: 18 /* KeyCode.DownArrow */,
        secondary: [2048 /* KeyMod.CtrlCmd */ | 18 /* KeyCode.DownArrow */],
    }
}));
registerEditorCommand(new CodeActionContribution({
    id: 'onEnterSelectCodeAction',
    precondition: Context.Visible,
    handler(x) {
        x.selectedOption();
    },
    kbOpts: {
        weight: weight + 100000,
        primary: 3 /* KeyCode.Enter */,
        secondary: [1024 /* KeyMod.Shift */ | 2 /* KeyCode.Tab */],
    }
}));
registerEditorCommand(new CodeActionContribution({
    id: 'onEnterSelectCodeActionWithPreview',
    precondition: Context.Visible,
    handler(x) {
        x.selectedOptionWithPreview();
    },
    kbOpts: {
        weight: weight + 100000,
        primary: 2048 /* KeyMod.CtrlCmd */ | 3 /* KeyCode.Enter */,
    }
}));
