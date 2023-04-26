/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import { coalesce, equals, isNonEmptyArray } from '../../../../base/common/arrays.js';
import { CancellationToken } from '../../../../base/common/cancellation.js';
import { illegalArgument, isCancellationError, onUnexpectedExternalError } from '../../../../base/common/errors.js';
import { Disposable, DisposableStore } from '../../../../base/common/lifecycle.js';
import { URI } from '../../../../base/common/uri.js';
import { TextModelCancellationTokenSource } from '../../editorState/browser/editorState.js';
import { Range } from '../../../common/core/range.js';
import { Selection } from '../../../common/core/selection.js';
import { IModelService } from '../../../common/services/model.js';
import { CommandsRegistry } from '../../../../platform/commands/common/commands.js';
import { Progress } from '../../../../platform/progress/common/progress.js';
import { CodeActionKind, CodeActionTriggerSource, filtersAction, mayIncludeActionsOfKind } from './types.js';
import { ILanguageFeaturesService } from '../../../common/services/languageFeatures.js';
export const codeActionCommandId = 'editor.action.codeAction';
export const refactorCommandId = 'editor.action.refactor';
export const refactorPreviewCommandId = 'editor.action.refactor.preview';
export const sourceActionCommandId = 'editor.action.sourceAction';
export const organizeImportsCommandId = 'editor.action.organizeImports';
export const fixAllCommandId = 'editor.action.fixAll';
export class CodeActionItem {
    constructor(action, provider) {
        this.action = action;
        this.provider = provider;
    }
    resolve(token) {
        var _a;
        return __awaiter(this, void 0, void 0, function* () {
            if (((_a = this.provider) === null || _a === void 0 ? void 0 : _a.resolveCodeAction) && !this.action.edit) {
                let action;
                try {
                    action = yield this.provider.resolveCodeAction(this.action, token);
                }
                catch (err) {
                    onUnexpectedExternalError(err);
                }
                if (action) {
                    this.action.edit = action.edit;
                }
            }
            return this;
        });
    }
}
class ManagedCodeActionSet extends Disposable {
    constructor(actions, documentation, disposables) {
        super();
        this.documentation = documentation;
        this._register(disposables);
        this.allActions = [...actions].sort(ManagedCodeActionSet.codeActionsComparator);
        this.validActions = this.allActions.filter(({ action }) => !action.disabled);
    }
    static codeActionsComparator({ action: a }, { action: b }) {
        if (a.isPreferred && !b.isPreferred) {
            return -1;
        }
        else if (!a.isPreferred && b.isPreferred) {
            return 1;
        }
        if (isNonEmptyArray(a.diagnostics)) {
            if (isNonEmptyArray(b.diagnostics)) {
                return a.diagnostics[0].message.localeCompare(b.diagnostics[0].message);
            }
            else {
                return -1;
            }
        }
        else if (isNonEmptyArray(b.diagnostics)) {
            return 1;
        }
        else {
            return 0; // both have no diagnostics
        }
    }
    get hasAutoFix() {
        return this.validActions.some(({ action: fix }) => !!fix.kind && CodeActionKind.QuickFix.contains(new CodeActionKind(fix.kind)) && !!fix.isPreferred);
    }
}
const emptyCodeActionsResponse = { actions: [], documentation: undefined };
export function getCodeActions(registry, model, rangeOrSelection, trigger, progress, token) {
    var _a;
    const filter = trigger.filter || {};
    const codeActionContext = {
        only: (_a = filter.include) === null || _a === void 0 ? void 0 : _a.value,
        trigger: trigger.type,
    };
    const cts = new TextModelCancellationTokenSource(model, token);
    const providers = getCodeActionProviders(registry, model, filter);
    const disposables = new DisposableStore();
    const promises = providers.map((provider) => __awaiter(this, void 0, void 0, function* () {
        try {
            progress.report(provider);
            const providedCodeActions = yield provider.provideCodeActions(model, rangeOrSelection, codeActionContext, cts.token);
            if (providedCodeActions) {
                disposables.add(providedCodeActions);
            }
            if (cts.token.isCancellationRequested) {
                return emptyCodeActionsResponse;
            }
            const filteredActions = ((providedCodeActions === null || providedCodeActions === void 0 ? void 0 : providedCodeActions.actions) || []).filter(action => action && filtersAction(filter, action));
            const documentation = getDocumentation(provider, filteredActions, filter.include);
            return {
                actions: filteredActions.map(action => new CodeActionItem(action, provider)),
                documentation
            };
        }
        catch (err) {
            if (isCancellationError(err)) {
                throw err;
            }
            onUnexpectedExternalError(err);
            return emptyCodeActionsResponse;
        }
    }));
    const listener = registry.onDidChange(() => {
        const newProviders = registry.all(model);
        if (!equals(newProviders, providers)) {
            cts.cancel();
        }
    });
    return Promise.all(promises).then(actions => {
        const allActions = actions.map(x => x.actions).flat();
        const allDocumentation = coalesce(actions.map(x => x.documentation));
        return new ManagedCodeActionSet(allActions, allDocumentation, disposables);
    })
        .finally(() => {
        listener.dispose();
        cts.dispose();
    });
}
function getCodeActionProviders(registry, model, filter) {
    return registry.all(model)
        // Don't include providers that we know will not return code actions of interest
        .filter(provider => {
        if (!provider.providedCodeActionKinds) {
            // We don't know what type of actions this provider will return.
            return true;
        }
        return provider.providedCodeActionKinds.some(kind => mayIncludeActionsOfKind(filter, new CodeActionKind(kind)));
    });
}
function getDocumentation(provider, providedCodeActions, only) {
    if (!provider.documentation) {
        return undefined;
    }
    const documentation = provider.documentation.map(entry => ({ kind: new CodeActionKind(entry.kind), command: entry.command }));
    if (only) {
        let currentBest;
        for (const entry of documentation) {
            if (entry.kind.contains(only)) {
                if (!currentBest) {
                    currentBest = entry;
                }
                else {
                    // Take best match
                    if (currentBest.kind.contains(entry.kind)) {
                        currentBest = entry;
                    }
                }
            }
        }
        if (currentBest) {
            return currentBest === null || currentBest === void 0 ? void 0 : currentBest.command;
        }
    }
    // Otherwise, check to see if any of the provided actions match.
    for (const action of providedCodeActions) {
        if (!action.kind) {
            continue;
        }
        for (const entry of documentation) {
            if (entry.kind.contains(new CodeActionKind(action.kind))) {
                return entry.command;
            }
        }
    }
    return undefined;
}
CommandsRegistry.registerCommand('_executeCodeActionProvider', function (accessor, resource, rangeOrSelection, kind, itemResolveCount) {
    return __awaiter(this, void 0, void 0, function* () {
        if (!(resource instanceof URI)) {
            throw illegalArgument();
        }
        const { codeActionProvider } = accessor.get(ILanguageFeaturesService);
        const model = accessor.get(IModelService).getModel(resource);
        if (!model) {
            throw illegalArgument();
        }
        const validatedRangeOrSelection = Selection.isISelection(rangeOrSelection)
            ? Selection.liftSelection(rangeOrSelection)
            : Range.isIRange(rangeOrSelection)
                ? model.validateRange(rangeOrSelection)
                : undefined;
        if (!validatedRangeOrSelection) {
            throw illegalArgument();
        }
        const include = typeof kind === 'string' ? new CodeActionKind(kind) : undefined;
        const codeActionSet = yield getCodeActions(codeActionProvider, model, validatedRangeOrSelection, { type: 1 /* languages.CodeActionTriggerType.Invoke */, triggerAction: CodeActionTriggerSource.Default, filter: { includeSourceActions: true, include } }, Progress.None, CancellationToken.None);
        const resolving = [];
        const resolveCount = Math.min(codeActionSet.validActions.length, typeof itemResolveCount === 'number' ? itemResolveCount : 0);
        for (let i = 0; i < resolveCount; i++) {
            resolving.push(codeActionSet.validActions[i].resolve(CancellationToken.None));
        }
        try {
            yield Promise.all(resolving);
            return codeActionSet.validActions.map(item => item.action);
        }
        finally {
            setTimeout(() => codeActionSet.dispose(), 100);
        }
    });
});
