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
import { Separator, SubmenuAction } from '../../../base/common/actions.js';
import { CSSIcon } from '../../../base/common/codicons.js';
import { Emitter } from '../../../base/common/event.js';
import { Iterable } from '../../../base/common/iterator.js';
import { toDisposable } from '../../../base/common/lifecycle.js';
import { LinkedList } from '../../../base/common/linkedList.js';
import { ICommandService } from '../../commands/common/commands.js';
import { IContextKeyService } from '../../contextkey/common/contextkey.js';
import { createDecorator } from '../../instantiation/common/instantiation.js';
import { ThemeIcon } from '../../theme/common/themeService.js';
export function isIMenuItem(item) {
    return item.command !== undefined;
}
export class MenuId {
    /**
     * Create a new `MenuId` with the unique identifier. Will throw if a menu
     * with the identifier already exists, use `MenuId.for(ident)` or a unique
     * identifier
     */
    constructor(identifier) {
        if (MenuId._instances.has(identifier)) {
            throw new TypeError(`MenuId with identifier '${identifier}' already exists. Use MenuId.for(ident) or a unique identifier`);
        }
        MenuId._instances.set(identifier, this);
        this.id = identifier;
    }
}
MenuId._instances = new Map();
MenuId.CommandPalette = new MenuId('CommandPalette');
MenuId.DebugBreakpointsContext = new MenuId('DebugBreakpointsContext');
MenuId.DebugCallStackContext = new MenuId('DebugCallStackContext');
MenuId.DebugConsoleContext = new MenuId('DebugConsoleContext');
MenuId.DebugVariablesContext = new MenuId('DebugVariablesContext');
MenuId.DebugWatchContext = new MenuId('DebugWatchContext');
MenuId.DebugToolBar = new MenuId('DebugToolBar');
MenuId.DebugToolBarStop = new MenuId('DebugToolBarStop');
MenuId.EditorContext = new MenuId('EditorContext');
MenuId.SimpleEditorContext = new MenuId('SimpleEditorContext');
MenuId.EditorContextCopy = new MenuId('EditorContextCopy');
MenuId.EditorContextPeek = new MenuId('EditorContextPeek');
MenuId.EditorContextShare = new MenuId('EditorContextShare');
MenuId.EditorTitle = new MenuId('EditorTitle');
MenuId.EditorTitleRun = new MenuId('EditorTitleRun');
MenuId.EditorTitleContext = new MenuId('EditorTitleContext');
MenuId.EmptyEditorGroup = new MenuId('EmptyEditorGroup');
MenuId.EmptyEditorGroupContext = new MenuId('EmptyEditorGroupContext');
MenuId.ExplorerContext = new MenuId('ExplorerContext');
MenuId.ExtensionContext = new MenuId('ExtensionContext');
MenuId.GlobalActivity = new MenuId('GlobalActivity');
MenuId.CommandCenter = new MenuId('CommandCenter');
MenuId.LayoutControlMenuSubmenu = new MenuId('LayoutControlMenuSubmenu');
MenuId.LayoutControlMenu = new MenuId('LayoutControlMenu');
MenuId.MenubarMainMenu = new MenuId('MenubarMainMenu');
MenuId.MenubarAppearanceMenu = new MenuId('MenubarAppearanceMenu');
MenuId.MenubarDebugMenu = new MenuId('MenubarDebugMenu');
MenuId.MenubarEditMenu = new MenuId('MenubarEditMenu');
MenuId.MenubarCopy = new MenuId('MenubarCopy');
MenuId.MenubarFileMenu = new MenuId('MenubarFileMenu');
MenuId.MenubarGoMenu = new MenuId('MenubarGoMenu');
MenuId.MenubarHelpMenu = new MenuId('MenubarHelpMenu');
MenuId.MenubarLayoutMenu = new MenuId('MenubarLayoutMenu');
MenuId.MenubarNewBreakpointMenu = new MenuId('MenubarNewBreakpointMenu');
MenuId.MenubarPanelAlignmentMenu = new MenuId('MenubarPanelAlignmentMenu');
MenuId.MenubarPanelPositionMenu = new MenuId('MenubarPanelPositionMenu');
MenuId.MenubarPreferencesMenu = new MenuId('MenubarPreferencesMenu');
MenuId.MenubarRecentMenu = new MenuId('MenubarRecentMenu');
MenuId.MenubarSelectionMenu = new MenuId('MenubarSelectionMenu');
MenuId.MenubarShare = new MenuId('MenubarShare');
MenuId.MenubarSwitchEditorMenu = new MenuId('MenubarSwitchEditorMenu');
MenuId.MenubarSwitchGroupMenu = new MenuId('MenubarSwitchGroupMenu');
MenuId.MenubarTerminalMenu = new MenuId('MenubarTerminalMenu');
MenuId.MenubarViewMenu = new MenuId('MenubarViewMenu');
MenuId.MenubarHomeMenu = new MenuId('MenubarHomeMenu');
MenuId.OpenEditorsContext = new MenuId('OpenEditorsContext');
MenuId.ProblemsPanelContext = new MenuId('ProblemsPanelContext');
MenuId.SCMChangeContext = new MenuId('SCMChangeContext');
MenuId.SCMResourceContext = new MenuId('SCMResourceContext');
MenuId.SCMResourceFolderContext = new MenuId('SCMResourceFolderContext');
MenuId.SCMResourceGroupContext = new MenuId('SCMResourceGroupContext');
MenuId.SCMSourceControl = new MenuId('SCMSourceControl');
MenuId.SCMTitle = new MenuId('SCMTitle');
MenuId.SearchContext = new MenuId('SearchContext');
MenuId.StatusBarWindowIndicatorMenu = new MenuId('StatusBarWindowIndicatorMenu');
MenuId.StatusBarRemoteIndicatorMenu = new MenuId('StatusBarRemoteIndicatorMenu');
MenuId.TestItem = new MenuId('TestItem');
MenuId.TestItemGutter = new MenuId('TestItemGutter');
MenuId.TestPeekElement = new MenuId('TestPeekElement');
MenuId.TestPeekTitle = new MenuId('TestPeekTitle');
MenuId.TouchBarContext = new MenuId('TouchBarContext');
MenuId.TitleBarContext = new MenuId('TitleBarContext');
MenuId.TitleBarTitleContext = new MenuId('TitleBarTitleContext');
MenuId.TunnelContext = new MenuId('TunnelContext');
MenuId.TunnelPrivacy = new MenuId('TunnelPrivacy');
MenuId.TunnelProtocol = new MenuId('TunnelProtocol');
MenuId.TunnelPortInline = new MenuId('TunnelInline');
MenuId.TunnelTitle = new MenuId('TunnelTitle');
MenuId.TunnelLocalAddressInline = new MenuId('TunnelLocalAddressInline');
MenuId.TunnelOriginInline = new MenuId('TunnelOriginInline');
MenuId.ViewItemContext = new MenuId('ViewItemContext');
MenuId.ViewContainerTitle = new MenuId('ViewContainerTitle');
MenuId.ViewContainerTitleContext = new MenuId('ViewContainerTitleContext');
MenuId.ViewTitle = new MenuId('ViewTitle');
MenuId.ViewTitleContext = new MenuId('ViewTitleContext');
MenuId.CommentThreadTitle = new MenuId('CommentThreadTitle');
MenuId.CommentThreadActions = new MenuId('CommentThreadActions');
MenuId.CommentTitle = new MenuId('CommentTitle');
MenuId.CommentActions = new MenuId('CommentActions');
MenuId.InteractiveToolbar = new MenuId('InteractiveToolbar');
MenuId.InteractiveCellTitle = new MenuId('InteractiveCellTitle');
MenuId.InteractiveCellDelete = new MenuId('InteractiveCellDelete');
MenuId.InteractiveCellExecute = new MenuId('InteractiveCellExecute');
MenuId.InteractiveInputExecute = new MenuId('InteractiveInputExecute');
MenuId.NotebookToolbar = new MenuId('NotebookToolbar');
MenuId.NotebookCellTitle = new MenuId('NotebookCellTitle');
MenuId.NotebookCellDelete = new MenuId('NotebookCellDelete');
MenuId.NotebookCellInsert = new MenuId('NotebookCellInsert');
MenuId.NotebookCellBetween = new MenuId('NotebookCellBetween');
MenuId.NotebookCellListTop = new MenuId('NotebookCellTop');
MenuId.NotebookCellExecute = new MenuId('NotebookCellExecute');
MenuId.NotebookCellExecutePrimary = new MenuId('NotebookCellExecutePrimary');
MenuId.NotebookDiffCellInputTitle = new MenuId('NotebookDiffCellInputTitle');
MenuId.NotebookDiffCellMetadataTitle = new MenuId('NotebookDiffCellMetadataTitle');
MenuId.NotebookDiffCellOutputsTitle = new MenuId('NotebookDiffCellOutputsTitle');
MenuId.NotebookOutputToolbar = new MenuId('NotebookOutputToolbar');
MenuId.NotebookEditorLayoutConfigure = new MenuId('NotebookEditorLayoutConfigure');
MenuId.NotebookKernelSource = new MenuId('NotebookKernelSource');
MenuId.BulkEditTitle = new MenuId('BulkEditTitle');
MenuId.BulkEditContext = new MenuId('BulkEditContext');
MenuId.TimelineItemContext = new MenuId('TimelineItemContext');
MenuId.TimelineTitle = new MenuId('TimelineTitle');
MenuId.TimelineTitleContext = new MenuId('TimelineTitleContext');
MenuId.TimelineFilterSubMenu = new MenuId('TimelineFilterSubMenu');
MenuId.AccountsContext = new MenuId('AccountsContext');
MenuId.PanelTitle = new MenuId('PanelTitle');
MenuId.AuxiliaryBarTitle = new MenuId('AuxiliaryBarTitle');
MenuId.TerminalInstanceContext = new MenuId('TerminalInstanceContext');
MenuId.TerminalEditorInstanceContext = new MenuId('TerminalEditorInstanceContext');
MenuId.TerminalNewDropdownContext = new MenuId('TerminalNewDropdownContext');
MenuId.TerminalTabContext = new MenuId('TerminalTabContext');
MenuId.TerminalTabEmptyAreaContext = new MenuId('TerminalTabEmptyAreaContext');
MenuId.TerminalInlineTabContext = new MenuId('TerminalInlineTabContext');
MenuId.WebviewContext = new MenuId('WebviewContext');
MenuId.InlineCompletionsActions = new MenuId('InlineCompletionsActions');
MenuId.NewFile = new MenuId('NewFile');
MenuId.MergeToolbar = new MenuId('MergeToolbar');
MenuId.MergeInput1Toolbar = new MenuId('MergeToolbar1Toolbar');
MenuId.MergeInput2Toolbar = new MenuId('MergeToolbar2Toolbar');
export const IMenuService = createDecorator('menuService');
export const MenuRegistry = new class {
    constructor() {
        this._commands = new Map();
        this._menuItems = new Map();
        this._onDidChangeMenu = new Emitter();
        this.onDidChangeMenu = this._onDidChangeMenu.event;
        this._commandPaletteChangeEvent = {
            has: id => id === MenuId.CommandPalette
        };
    }
    addCommand(command) {
        return this.addCommands(Iterable.single(command));
    }
    addCommands(commands) {
        for (const command of commands) {
            this._commands.set(command.id, command);
        }
        this._onDidChangeMenu.fire(this._commandPaletteChangeEvent);
        return toDisposable(() => {
            let didChange = false;
            for (const command of commands) {
                didChange = this._commands.delete(command.id) || didChange;
            }
            if (didChange) {
                this._onDidChangeMenu.fire(this._commandPaletteChangeEvent);
            }
        });
    }
    getCommand(id) {
        return this._commands.get(id);
    }
    getCommands() {
        const map = new Map();
        this._commands.forEach((value, key) => map.set(key, value));
        return map;
    }
    appendMenuItem(id, item) {
        return this.appendMenuItems(Iterable.single({ id, item }));
    }
    appendMenuItems(items) {
        const changedIds = new Set();
        const toRemove = new LinkedList();
        for (const { id, item } of items) {
            let list = this._menuItems.get(id);
            if (!list) {
                list = new LinkedList();
                this._menuItems.set(id, list);
            }
            toRemove.push(list.push(item));
            changedIds.add(id);
        }
        this._onDidChangeMenu.fire(changedIds);
        return toDisposable(() => {
            if (toRemove.size > 0) {
                for (const fn of toRemove) {
                    fn();
                }
                this._onDidChangeMenu.fire(changedIds);
                toRemove.clear();
            }
        });
    }
    getMenuItems(id) {
        let result;
        if (this._menuItems.has(id)) {
            result = [...this._menuItems.get(id)];
        }
        else {
            result = [];
        }
        if (id === MenuId.CommandPalette) {
            // CommandPalette is special because it shows
            // all commands by default
            this._appendImplicitItems(result);
        }
        return result;
    }
    _appendImplicitItems(result) {
        const set = new Set();
        for (const item of result) {
            if (isIMenuItem(item)) {
                set.add(item.command.id);
                if (item.alt) {
                    set.add(item.alt.id);
                }
            }
        }
        this._commands.forEach((command, id) => {
            if (!set.has(id)) {
                result.push({ command });
            }
        });
    }
};
export class SubmenuItemAction extends SubmenuAction {
    constructor(item, _menuService, _contextKeyService, _options) {
        super(`submenuitem.${item.submenu.id}`, typeof item.title === 'string' ? item.title : item.title.value, [], 'submenu');
        this.item = item;
        this._menuService = _menuService;
        this._contextKeyService = _contextKeyService;
        this._options = _options;
    }
    get actions() {
        const result = [];
        const menu = this._menuService.createMenu(this.item.submenu, this._contextKeyService);
        const groups = menu.getActions(this._options);
        menu.dispose();
        for (const [, actions] of groups) {
            if (actions.length > 0) {
                result.push(...actions);
                result.push(new Separator());
            }
        }
        if (result.length) {
            result.pop(); // remove last separator
        }
        return result;
    }
}
// implements IAction, does NOT extend Action, so that no one
// subscribes to events of Action or modified properties
let MenuItemAction = class MenuItemAction {
    constructor(item, alt, options, hideActions, contextKeyService, _commandService) {
        var _a, _b;
        this.hideActions = hideActions;
        this._commandService = _commandService;
        this.id = item.id;
        this.label = (options === null || options === void 0 ? void 0 : options.renderShortTitle) && item.shortTitle
            ? (typeof item.shortTitle === 'string' ? item.shortTitle : item.shortTitle.value)
            : (typeof item.title === 'string' ? item.title : item.title.value);
        this.tooltip = (_b = (typeof item.tooltip === 'string' ? item.tooltip : (_a = item.tooltip) === null || _a === void 0 ? void 0 : _a.value)) !== null && _b !== void 0 ? _b : '';
        this.enabled = !item.precondition || contextKeyService.contextMatchesRules(item.precondition);
        this.checked = undefined;
        if (item.toggled) {
            const toggled = (item.toggled.condition ? item.toggled : { condition: item.toggled });
            this.checked = contextKeyService.contextMatchesRules(toggled.condition);
            if (this.checked && toggled.tooltip) {
                this.tooltip = typeof toggled.tooltip === 'string' ? toggled.tooltip : toggled.tooltip.value;
            }
            if (toggled.title) {
                this.label = typeof toggled.title === 'string' ? toggled.title : toggled.title.value;
            }
        }
        this.item = item;
        this.alt = alt ? new MenuItemAction(alt, undefined, options, hideActions, contextKeyService, _commandService) : undefined;
        this._options = options;
        if (ThemeIcon.isThemeIcon(item.icon)) {
            this.class = CSSIcon.asClassName(item.icon);
        }
    }
    dispose() {
        // there is NOTHING to dispose and the MenuItemAction should
        // never have anything to dispose as it is a convenience type
        // to bridge into the rendering world.
    }
    run(...args) {
        var _a, _b;
        let runArgs = [];
        if ((_a = this._options) === null || _a === void 0 ? void 0 : _a.arg) {
            runArgs = [...runArgs, this._options.arg];
        }
        if ((_b = this._options) === null || _b === void 0 ? void 0 : _b.shouldForwardArgs) {
            runArgs = [...runArgs, ...args];
        }
        return this._commandService.executeCommand(this.id, ...runArgs);
    }
};
MenuItemAction = __decorate([
    __param(4, IContextKeyService),
    __param(5, ICommandService)
], MenuItemAction);
export { MenuItemAction };
//#endregion
