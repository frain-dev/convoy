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
import * as dom from '../../../../base/browser/dom.js';
import { List } from '../../../../base/browser/ui/list/listWidget.js';
import { Action, Separator } from '../../../../base/common/actions.js';
import { canceled } from '../../../../base/common/errors.js';
import { Lazy } from '../../../../base/common/lazy.js';
import { Disposable, dispose, MutableDisposable, DisposableStore } from '../../../../base/common/lifecycle.js';
import './media/action.css';
import { Position } from '../../../common/core/position.js';
import { ILanguageFeaturesService } from '../../../common/services/languageFeatures.js';
import { codeActionCommandId, CodeActionItem, fixAllCommandId, organizeImportsCommandId, refactorCommandId, sourceActionCommandId } from './codeAction.js';
import { CodeActionCommandArgs, CodeActionKind, CodeActionTriggerSource } from './types.js';
import { localize } from '../../../../nls.js';
import { IConfigurationService } from '../../../../platform/configuration/common/configuration.js';
import { IContextKeyService, RawContextKey } from '../../../../platform/contextkey/common/contextkey.js';
import { IContextMenuService, IContextViewService } from '../../../../platform/contextview/browser/contextView.js';
import { IKeybindingService } from '../../../../platform/keybinding/common/keybinding.js';
import { ITelemetryService } from '../../../../platform/telemetry/common/telemetry.js';
import { IThemeService } from '../../../../platform/theme/common/themeService.js';
export const Context = {
    Visible: new RawContextKey('CodeActionMenuVisible', false, localize('CodeActionMenuVisible', "Whether the code action list widget is visible"))
};
class CodeActionAction extends Action {
    constructor(action, callback) {
        super(action.command ? action.command.id : action.title, stripNewlines(action.title), undefined, !action.disabled, callback);
        this.action = action;
    }
}
function stripNewlines(str) {
    return str.replace(/\r\n|\r|\n/g, ' ');
}
const TEMPLATE_ID = 'codeActionWidget';
const codeActionLineHeight = 26;
let CodeMenuRenderer = class CodeMenuRenderer {
    constructor(acceptKeybindings, keybindingService) {
        this.acceptKeybindings = acceptKeybindings;
        this.keybindingService = keybindingService;
    }
    get templateId() { return TEMPLATE_ID; }
    renderTemplate(container) {
        const data = Object.create(null);
        data.disposables = [];
        data.root = container;
        data.text = document.createElement('span');
        // data.detail = document.createElement('');
        container.append(data.text);
        // container.append(data.detail);
        return data;
    }
    renderElement(element, index, templateData) {
        const data = templateData;
        const text = element.title;
        // const detail = element.detail;
        const isEnabled = element.isEnabled;
        const isSeparator = element.isSeparator;
        const isDocumentation = element.isDocumentation;
        data.text.textContent = text;
        // data.detail.textContent = detail;
        if (!isEnabled) {
            data.root.classList.add('option-disabled');
            data.root.style.backgroundColor = 'transparent !important';
        }
        else {
            data.root.classList.remove('option-disabled');
        }
        if (isSeparator) {
            data.root.classList.add('separator');
            data.root.style.height = '10px';
        }
        if (!isDocumentation) {
            const updateLabel = () => {
                var _a, _b;
                const [accept, preview] = this.acceptKeybindings;
                data.root.title = localize({ key: 'label', comment: ['placeholders are keybindings, e.g "F2 to Refactor, Shift+F2 to Preview"'] }, "{0} to Refactor, {1} to Preview", (_a = this.keybindingService.lookupKeybinding(accept)) === null || _a === void 0 ? void 0 : _a.getLabel(), (_b = this.keybindingService.lookupKeybinding(preview)) === null || _b === void 0 ? void 0 : _b.getLabel());
                // data.root.title = this.keybindingService.lookupKeybinding(accept)?.getLabel() + ' to Refactor, ' + this.keybindingService.lookupKeybinding(preview)?.getLabel() + ' to Preview';
            };
            updateLabel();
        }
    }
    disposeTemplate(templateData) {
        templateData.disposables = dispose(templateData.disposables);
    }
};
CodeMenuRenderer = __decorate([
    __param(1, IKeybindingService)
], CodeMenuRenderer);
let CodeActionMenu = class CodeActionMenu extends Disposable {
    constructor(_editor, _delegate, _contextMenuService, keybindingService, _languageFeaturesService, _telemetryService, _themeService, _configurationService, _contextViewService, _contextKeyService) {
        super();
        this._editor = _editor;
        this._delegate = _delegate;
        this._contextMenuService = _contextMenuService;
        this._languageFeaturesService = _languageFeaturesService;
        this._telemetryService = _telemetryService;
        this._configurationService = _configurationService;
        this._contextViewService = _contextViewService;
        this._contextKeyService = _contextKeyService;
        this._showingActions = this._register(new MutableDisposable());
        this.codeActionList = this._register(new MutableDisposable());
        this.options = [];
        this._visible = false;
        this.viewItems = [];
        this.hasSeperator = false;
        this._keybindingResolver = new CodeActionKeybindingResolver({
            getKeybindings: () => keybindingService.getKeybindings()
        });
        this._ctxMenuWidgetVisible = Context.Visible.bindTo(this._contextKeyService);
        this.listRenderer = new CodeMenuRenderer([`onEnterSelectCodeAction`, `onEnterSelectCodeActionWithPreview`], keybindingService);
    }
    get isVisible() {
        return this._visible;
    }
    isCodeActionWidgetEnabled(model) {
        return this._configurationService.getValue('editor.experimental.useCustomCodeActionMenu', {
            resource: model.uri
        });
    }
    _onListSelection(e) {
        if (e.elements.length) {
            e.elements.forEach(element => {
                if (element.isEnabled) {
                    element.action.run();
                    this.hideCodeActionWidget();
                }
            });
        }
    }
    _onListHover(e) {
        var _a, _b, _c, _d;
        if (!e.element) {
            this.currSelectedItem = undefined;
            (_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.setFocus([]);
        }
        else {
            if ((_b = e.element) === null || _b === void 0 ? void 0 : _b.isEnabled) {
                (_c = this.codeActionList.value) === null || _c === void 0 ? void 0 : _c.setFocus([e.element.index]);
                this.focusedEnabledItem = this.viewItems.indexOf(e.element);
                this.currSelectedItem = e.element.index;
            }
            else {
                this.currSelectedItem = undefined;
                (_d = this.codeActionList.value) === null || _d === void 0 ? void 0 : _d.setFocus([e.element.index]);
            }
        }
    }
    renderCodeActionMenuList(element, inputArray) {
        var _a;
        const renderDisposables = new DisposableStore();
        const renderMenu = document.createElement('div');
        // Render invisible div to block mouse interaction in the rest of the UI
        const menuBlock = document.createElement('div');
        this.block = element.appendChild(menuBlock);
        this.block.classList.add('context-view-block');
        this.block.style.position = 'fixed';
        this.block.style.cursor = 'initial';
        this.block.style.left = '0';
        this.block.style.top = '0';
        this.block.style.width = '100%';
        this.block.style.height = '100%';
        this.block.style.zIndex = '-1';
        renderDisposables.add(dom.addDisposableListener(this.block, dom.EventType.MOUSE_DOWN, e => e.stopPropagation()));
        renderMenu.id = 'codeActionMenuWidget';
        renderMenu.classList.add('codeActionMenuWidget');
        element.appendChild(renderMenu);
        this.codeActionList.value = new List('codeActionWidget', renderMenu, {
            getHeight(element) {
                if (element.isSeparator) {
                    return 10;
                }
                return codeActionLineHeight;
            },
            getTemplateId(element) {
                return 'codeActionWidget';
            }
        }, [this.listRenderer], { keyboardSupport: false });
        renderDisposables.add(this.codeActionList.value.onMouseOver(e => this._onListHover(e)));
        renderDisposables.add(this.codeActionList.value.onDidChangeFocus(e => { var _a; return (_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.domFocus(); }));
        renderDisposables.add(this.codeActionList.value.onDidChangeSelection(e => this._onListSelection(e)));
        renderDisposables.add(this._editor.onDidLayoutChange(e => this.hideCodeActionWidget()));
        // Populating the list widget and tracking enabled options.
        inputArray.forEach((item, index) => {
            const currIsSeparator = item.class === 'separator';
            let isDocumentation = false;
            if (item instanceof CodeActionAction) {
                isDocumentation = item.action.kind === CodeActionMenu.documentationID;
            }
            if (currIsSeparator) {
                // set to true forever
                this.hasSeperator = true;
            }
            const menuItem = { title: item.label, detail: item.tooltip, action: inputArray[index], isEnabled: item.enabled, isSeparator: currIsSeparator, index, isDocumentation };
            if (item.enabled) {
                this.viewItems.push(menuItem);
            }
            this.options.push(menuItem);
        });
        this.codeActionList.value.splice(0, this.codeActionList.value.length, this.options);
        const height = this.hasSeperator ? (inputArray.length - 1) * codeActionLineHeight + 10 : inputArray.length * codeActionLineHeight;
        renderMenu.style.height = String(height) + 'px';
        this.codeActionList.value.layout(height);
        // For finding width dynamically (not using resize observer)
        const arr = [];
        this.options.forEach((item, index) => {
            var _a, _b;
            if (!this.codeActionList.value) {
                return;
            }
            const element = (_b = document.getElementById((_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.getElementID(index))) === null || _b === void 0 ? void 0 : _b.getElementsByTagName('span')[0].offsetWidth;
            arr.push(Number(element));
        });
        // resize observer - can be used in the future since list widget supports dynamic height but not width
        const maxWidth = Math.max(...arr);
        // 40 is the additional padding for the list widget (20 left, 20 right)
        renderMenu.style.width = maxWidth + 52 + 'px';
        (_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.layout(height, maxWidth);
        // List selection
        if (this.viewItems.length < 1 || this.viewItems.every(item => item.isDocumentation)) {
            this.currSelectedItem = undefined;
        }
        else {
            this.focusedEnabledItem = 0;
            this.currSelectedItem = this.viewItems[0].index;
            this.codeActionList.value.setFocus([this.currSelectedItem]);
        }
        // List Focus
        this.codeActionList.value.domFocus();
        const focusTracker = dom.trackFocus(element);
        const blurListener = focusTracker.onDidBlur(() => {
            this.hideCodeActionWidget();
            // this._contextViewService.hideContextView({ source: this });
        });
        renderDisposables.add(blurListener);
        renderDisposables.add(focusTracker);
        this._ctxMenuWidgetVisible.set(true);
        return renderDisposables;
    }
    focusPrevious() {
        var _a;
        if (typeof this.focusedEnabledItem === 'undefined') {
            this.focusedEnabledItem = this.viewItems[0].index;
        }
        else if (this.viewItems.length < 1) {
            return false;
        }
        const startIndex = this.focusedEnabledItem;
        let item;
        do {
            this.focusedEnabledItem = this.focusedEnabledItem - 1;
            if (this.focusedEnabledItem < 0) {
                this.focusedEnabledItem = this.viewItems.length - 1;
            }
            item = this.viewItems[this.focusedEnabledItem];
            (_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.setFocus([item.index]);
            this.currSelectedItem = item.index;
        } while (this.focusedEnabledItem !== startIndex && ((!item.isEnabled) || item.action.id === Separator.ID));
        return true;
    }
    focusNext() {
        var _a;
        if (typeof this.focusedEnabledItem === 'undefined') {
            this.focusedEnabledItem = this.viewItems.length - 1;
        }
        else if (this.viewItems.length < 1) {
            return false;
        }
        const startIndex = this.focusedEnabledItem;
        let item;
        do {
            this.focusedEnabledItem = (this.focusedEnabledItem + 1) % this.viewItems.length;
            item = this.viewItems[this.focusedEnabledItem];
            (_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.setFocus([item.index]);
            this.currSelectedItem = item.index;
        } while (this.focusedEnabledItem !== startIndex && ((!item.isEnabled) || item.action.id === Separator.ID));
        return true;
    }
    navigateListWithKeysUp() {
        this.focusPrevious();
    }
    navigateListWithKeysDown() {
        this.focusNext();
    }
    onEnterSet() {
        var _a;
        if (typeof this.currSelectedItem === 'number') {
            (_a = this.codeActionList.value) === null || _a === void 0 ? void 0 : _a.setSelection([this.currSelectedItem]);
        }
    }
    dispose() {
        super.dispose();
    }
    hideCodeActionWidget() {
        this._ctxMenuWidgetVisible.reset();
        this.options = [];
        this.viewItems = [];
        this.focusedEnabledItem = 0;
        this.currSelectedItem = undefined;
        this.hasSeperator = false;
        this._contextViewService.hideContextView({ source: this });
    }
    codeActionTelemetry(openedFromString, didCancel, CodeActions) {
        this._telemetryService.publicLog2('codeAction.applyCodeAction', {
            codeActionFrom: openedFromString,
            validCodeActions: CodeActions.validActions.length,
            cancelled: didCancel,
        });
    }
    show(trigger, codeActions, at, options) {
        return __awaiter(this, void 0, void 0, function* () {
            const model = this._editor.getModel();
            if (!model) {
                return;
            }
            const actionsToShow = options.includeDisabledActions ? codeActions.allActions : codeActions.validActions;
            if (!actionsToShow.length) {
                this._visible = false;
                return;
            }
            if (!this._editor.getDomNode()) {
                // cancel when editor went off-dom
                this._visible = false;
                throw canceled();
            }
            this._visible = true;
            this._showingActions.value = codeActions;
            const menuActions = this.getMenuActions(trigger, actionsToShow, codeActions.documentation);
            const anchor = Position.isIPosition(at) ? this._toCoords(at) : at || { x: 0, y: 0 };
            const resolver = this._keybindingResolver.getResolver();
            const useShadowDOM = this._editor.getOption(117 /* EditorOption.useShadowDOM */);
            if (this.isCodeActionWidgetEnabled(model)) {
                this._contextViewService.showContextView({
                    getAnchor: () => anchor,
                    render: (container) => this.renderCodeActionMenuList(container, menuActions),
                    onHide: (didCancel) => {
                        const openedFromString = (options.fromLightbulb) ? CodeActionTriggerSource.Lightbulb : trigger.triggerAction;
                        this.codeActionTelemetry(openedFromString, didCancel, codeActions);
                        this._visible = false;
                        this._editor.focus();
                    },
                }, this._editor.getDomNode(), false);
            }
            else {
                this._contextMenuService.showContextMenu({
                    domForShadowRoot: useShadowDOM ? this._editor.getDomNode() : undefined,
                    getAnchor: () => anchor,
                    getActions: () => menuActions,
                    onHide: (didCancel) => {
                        const openedFromString = (options.fromLightbulb) ? CodeActionTriggerSource.Lightbulb : trigger.triggerAction;
                        this.codeActionTelemetry(openedFromString, didCancel, codeActions);
                        this._visible = false;
                        this._editor.focus();
                    },
                    autoSelectFirstItem: true,
                    getKeyBinding: action => action instanceof CodeActionAction ? resolver(action.action) : undefined,
                });
            }
        });
    }
    getMenuActions(trigger, actionsToShow, documentation) {
        var _a, _b;
        const toCodeActionAction = (item) => new CodeActionAction(item.action, () => this._delegate.onSelectCodeAction(item, trigger));
        const result = actionsToShow
            .map(toCodeActionAction);
        const allDocumentation = [...documentation];
        const model = this._editor.getModel();
        if (model && result.length) {
            for (const provider of this._languageFeaturesService.codeActionProvider.all(model)) {
                if (provider._getAdditionalMenuItems) {
                    allDocumentation.push(...provider._getAdditionalMenuItems({ trigger: trigger.type, only: (_b = (_a = trigger.filter) === null || _a === void 0 ? void 0 : _a.include) === null || _b === void 0 ? void 0 : _b.value }, actionsToShow.map(item => item.action)));
                }
            }
        }
        if (allDocumentation.length) {
            result.push(new Separator(), ...allDocumentation.map(command => toCodeActionAction(new CodeActionItem({
                title: command.title,
                command: command,
                kind: CodeActionMenu.documentationID
            }, undefined))));
        }
        return result;
    }
    _toCoords(position) {
        if (!this._editor.hasModel()) {
            return { x: 0, y: 0 };
        }
        this._editor.revealPosition(position, 1 /* ScrollType.Immediate */);
        this._editor.render();
        // Translate to absolute editor position
        const cursorCoords = this._editor.getScrolledVisiblePosition(position);
        const editorCoords = dom.getDomNodePagePosition(this._editor.getDomNode());
        const x = editorCoords.left + cursorCoords.left;
        const y = editorCoords.top + cursorCoords.top + cursorCoords.height;
        return { x, y };
    }
};
CodeActionMenu.documentationID = '_documentation';
CodeActionMenu = __decorate([
    __param(2, IContextMenuService),
    __param(3, IKeybindingService),
    __param(4, ILanguageFeaturesService),
    __param(5, ITelemetryService),
    __param(6, IThemeService),
    __param(7, IConfigurationService),
    __param(8, IContextViewService),
    __param(9, IContextKeyService)
], CodeActionMenu);
export { CodeActionMenu };
export class CodeActionKeybindingResolver {
    constructor(_keybindingProvider) {
        this._keybindingProvider = _keybindingProvider;
    }
    getResolver() {
        // Lazy since we may not actually ever read the value
        const allCodeActionBindings = new Lazy(() => this._keybindingProvider.getKeybindings()
            .filter(item => CodeActionKeybindingResolver.codeActionCommands.indexOf(item.command) >= 0)
            .filter(item => item.resolvedKeybinding)
            .map((item) => {
            // Special case these commands since they come built-in with VS Code and don't use 'commandArgs'
            let commandArgs = item.commandArgs;
            if (item.command === organizeImportsCommandId) {
                commandArgs = { kind: CodeActionKind.SourceOrganizeImports.value };
            }
            else if (item.command === fixAllCommandId) {
                commandArgs = { kind: CodeActionKind.SourceFixAll.value };
            }
            return Object.assign({ resolvedKeybinding: item.resolvedKeybinding }, CodeActionCommandArgs.fromUser(commandArgs, {
                kind: CodeActionKind.None,
                apply: "never" /* CodeActionAutoApply.Never */
            }));
        }));
        return (action) => {
            if (action.kind) {
                const binding = this.bestKeybindingForCodeAction(action, allCodeActionBindings.getValue());
                return binding === null || binding === void 0 ? void 0 : binding.resolvedKeybinding;
            }
            return undefined;
        };
    }
    bestKeybindingForCodeAction(action, candidates) {
        if (!action.kind) {
            return undefined;
        }
        const kind = new CodeActionKind(action.kind);
        return candidates
            .filter(candidate => candidate.kind.contains(kind))
            .filter(candidate => {
            if (candidate.preferred) {
                // If the candidate keybinding only applies to preferred actions, the this action must also be preferred
                return action.isPreferred;
            }
            return true;
        })
            .reduceRight((currentBest, candidate) => {
            if (!currentBest) {
                return candidate;
            }
            // Select the more specific binding
            return currentBest.kind.contains(candidate.kind) ? candidate : currentBest;
        }, undefined);
    }
}
CodeActionKeybindingResolver.codeActionCommands = [
    refactorCommandId,
    codeActionCommandId,
    sourceActionCommandId,
    organizeImportsCommandId,
    fixAllCommandId
];
