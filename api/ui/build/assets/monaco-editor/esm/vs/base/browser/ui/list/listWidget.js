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
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import { createStyleSheet } from '../../dom.js';
import { DomEmitter, stopEvent } from '../../event.js';
import { StandardKeyboardEvent } from '../../keyboardEvent.js';
import { Gesture } from '../../touch.js';
import { alert } from '../aria/aria.js';
import { CombinedSpliceable } from './splice.js';
import { binarySearch, firstOrDefault, range } from '../../../common/arrays.js';
import { timeout } from '../../../common/async.js';
import { Color } from '../../../common/color.js';
import { memoize } from '../../../common/decorators.js';
import { Emitter, Event, EventBufferer } from '../../../common/event.js';
import { matchesPrefix } from '../../../common/filters.js';
import { DisposableStore, dispose } from '../../../common/lifecycle.js';
import { clamp } from '../../../common/numbers.js';
import { mixin } from '../../../common/objects.js';
import * as platform from '../../../common/platform.js';
import { isNumber } from '../../../common/types.js';
import './list.css';
import { ListError } from './list.js';
import { ListView } from './listView.js';
class TraitRenderer {
    constructor(trait) {
        this.trait = trait;
        this.renderedElements = [];
    }
    get templateId() {
        return `template:${this.trait.name}`;
    }
    renderTemplate(container) {
        return container;
    }
    renderElement(element, index, templateData) {
        const renderedElementIndex = this.renderedElements.findIndex(el => el.templateData === templateData);
        if (renderedElementIndex >= 0) {
            const rendered = this.renderedElements[renderedElementIndex];
            this.trait.unrender(templateData);
            rendered.index = index;
        }
        else {
            const rendered = { index, templateData };
            this.renderedElements.push(rendered);
        }
        this.trait.renderIndex(index, templateData);
    }
    splice(start, deleteCount, insertCount) {
        const rendered = [];
        for (const renderedElement of this.renderedElements) {
            if (renderedElement.index < start) {
                rendered.push(renderedElement);
            }
            else if (renderedElement.index >= start + deleteCount) {
                rendered.push({
                    index: renderedElement.index + insertCount - deleteCount,
                    templateData: renderedElement.templateData
                });
            }
        }
        this.renderedElements = rendered;
    }
    renderIndexes(indexes) {
        for (const { index, templateData } of this.renderedElements) {
            if (indexes.indexOf(index) > -1) {
                this.trait.renderIndex(index, templateData);
            }
        }
    }
    disposeTemplate(templateData) {
        const index = this.renderedElements.findIndex(el => el.templateData === templateData);
        if (index < 0) {
            return;
        }
        this.renderedElements.splice(index, 1);
    }
}
class Trait {
    constructor(_trait) {
        this._trait = _trait;
        this.length = 0;
        this.indexes = [];
        this.sortedIndexes = [];
        this._onChange = new Emitter();
        this.onChange = this._onChange.event;
    }
    get name() { return this._trait; }
    get renderer() {
        return new TraitRenderer(this);
    }
    splice(start, deleteCount, elements) {
        var _a;
        deleteCount = Math.max(0, Math.min(deleteCount, this.length - start));
        const diff = elements.length - deleteCount;
        const end = start + deleteCount;
        const sortedIndexes = [
            ...this.sortedIndexes.filter(i => i < start),
            ...elements.map((hasTrait, i) => hasTrait ? i + start : -1).filter(i => i !== -1),
            ...this.sortedIndexes.filter(i => i >= end).map(i => i + diff)
        ];
        const length = this.length + diff;
        if (this.sortedIndexes.length > 0 && sortedIndexes.length === 0 && length > 0) {
            const first = (_a = this.sortedIndexes.find(index => index >= start)) !== null && _a !== void 0 ? _a : length - 1;
            sortedIndexes.push(Math.min(first, length - 1));
        }
        this.renderer.splice(start, deleteCount, elements.length);
        this._set(sortedIndexes, sortedIndexes);
        this.length = length;
    }
    renderIndex(index, container) {
        container.classList.toggle(this._trait, this.contains(index));
    }
    unrender(container) {
        container.classList.remove(this._trait);
    }
    /**
     * Sets the indexes which should have this trait.
     *
     * @param indexes Indexes which should have this trait.
     * @return The old indexes which had this trait.
     */
    set(indexes, browserEvent) {
        return this._set(indexes, [...indexes].sort(numericSort), browserEvent);
    }
    _set(indexes, sortedIndexes, browserEvent) {
        const result = this.indexes;
        const sortedResult = this.sortedIndexes;
        this.indexes = indexes;
        this.sortedIndexes = sortedIndexes;
        const toRender = disjunction(sortedResult, indexes);
        this.renderer.renderIndexes(toRender);
        this._onChange.fire({ indexes, browserEvent });
        return result;
    }
    get() {
        return this.indexes;
    }
    contains(index) {
        return binarySearch(this.sortedIndexes, index, numericSort) >= 0;
    }
    dispose() {
        dispose(this._onChange);
    }
}
__decorate([
    memoize
], Trait.prototype, "renderer", null);
class SelectionTrait extends Trait {
    constructor(setAriaSelected) {
        super('selected');
        this.setAriaSelected = setAriaSelected;
    }
    renderIndex(index, container) {
        super.renderIndex(index, container);
        if (this.setAriaSelected) {
            if (this.contains(index)) {
                container.setAttribute('aria-selected', 'true');
            }
            else {
                container.setAttribute('aria-selected', 'false');
            }
        }
    }
}
/**
 * The TraitSpliceable is used as a util class to be able
 * to preserve traits across splice calls, given an identity
 * provider.
 */
class TraitSpliceable {
    constructor(trait, view, identityProvider) {
        this.trait = trait;
        this.view = view;
        this.identityProvider = identityProvider;
    }
    splice(start, deleteCount, elements) {
        if (!this.identityProvider) {
            return this.trait.splice(start, deleteCount, elements.map(() => false));
        }
        const pastElementsWithTrait = this.trait.get().map(i => this.identityProvider.getId(this.view.element(i)).toString());
        const elementsWithTrait = elements.map(e => pastElementsWithTrait.indexOf(this.identityProvider.getId(e).toString()) > -1);
        this.trait.splice(start, deleteCount, elementsWithTrait);
    }
}
export function isInputElement(e) {
    return e.tagName === 'INPUT' || e.tagName === 'TEXTAREA';
}
export function isMonacoEditor(e) {
    if (e.classList.contains('monaco-editor')) {
        return true;
    }
    if (e.classList.contains('monaco-list')) {
        return false;
    }
    if (!e.parentElement) {
        return false;
    }
    return isMonacoEditor(e.parentElement);
}
export function isButton(e) {
    if ((e.tagName === 'A' && e.classList.contains('monaco-button')) ||
        (e.tagName === 'DIV' && e.classList.contains('monaco-button-dropdown'))) {
        return true;
    }
    if (e.classList.contains('monaco-list')) {
        return false;
    }
    if (!e.parentElement) {
        return false;
    }
    return isButton(e.parentElement);
}
class KeyboardController {
    constructor(list, view, options) {
        this.list = list;
        this.view = view;
        this.disposables = new DisposableStore();
        this.multipleSelectionDisposables = new DisposableStore();
        this.onKeyDown.filter(e => e.keyCode === 3 /* KeyCode.Enter */).on(this.onEnter, this, this.disposables);
        this.onKeyDown.filter(e => e.keyCode === 16 /* KeyCode.UpArrow */).on(this.onUpArrow, this, this.disposables);
        this.onKeyDown.filter(e => e.keyCode === 18 /* KeyCode.DownArrow */).on(this.onDownArrow, this, this.disposables);
        this.onKeyDown.filter(e => e.keyCode === 11 /* KeyCode.PageUp */).on(this.onPageUpArrow, this, this.disposables);
        this.onKeyDown.filter(e => e.keyCode === 12 /* KeyCode.PageDown */).on(this.onPageDownArrow, this, this.disposables);
        this.onKeyDown.filter(e => e.keyCode === 9 /* KeyCode.Escape */).on(this.onEscape, this, this.disposables);
        if (options.multipleSelectionSupport !== false) {
            this.onKeyDown.filter(e => (platform.isMacintosh ? e.metaKey : e.ctrlKey) && e.keyCode === 31 /* KeyCode.KeyA */).on(this.onCtrlA, this, this.multipleSelectionDisposables);
        }
    }
    get onKeyDown() {
        return this.disposables.add(Event.chain(this.disposables.add(new DomEmitter(this.view.domNode, 'keydown')).event)
            .filter(e => !isInputElement(e.target))
            .map(e => new StandardKeyboardEvent(e)));
    }
    updateOptions(optionsUpdate) {
        if (optionsUpdate.multipleSelectionSupport !== undefined) {
            this.multipleSelectionDisposables.clear();
            if (optionsUpdate.multipleSelectionSupport) {
                this.onKeyDown.filter(e => (platform.isMacintosh ? e.metaKey : e.ctrlKey) && e.keyCode === 31 /* KeyCode.KeyA */).on(this.onCtrlA, this, this.multipleSelectionDisposables);
            }
        }
    }
    onEnter(e) {
        e.preventDefault();
        e.stopPropagation();
        this.list.setSelection(this.list.getFocus(), e.browserEvent);
    }
    onUpArrow(e) {
        e.preventDefault();
        e.stopPropagation();
        this.list.focusPrevious(1, false, e.browserEvent);
        const el = this.list.getFocus()[0];
        this.list.setAnchor(el);
        this.list.reveal(el);
        this.view.domNode.focus();
    }
    onDownArrow(e) {
        e.preventDefault();
        e.stopPropagation();
        this.list.focusNext(1, false, e.browserEvent);
        const el = this.list.getFocus()[0];
        this.list.setAnchor(el);
        this.list.reveal(el);
        this.view.domNode.focus();
    }
    onPageUpArrow(e) {
        e.preventDefault();
        e.stopPropagation();
        this.list.focusPreviousPage(e.browserEvent);
        const el = this.list.getFocus()[0];
        this.list.setAnchor(el);
        this.list.reveal(el);
        this.view.domNode.focus();
    }
    onPageDownArrow(e) {
        e.preventDefault();
        e.stopPropagation();
        this.list.focusNextPage(e.browserEvent);
        const el = this.list.getFocus()[0];
        this.list.setAnchor(el);
        this.list.reveal(el);
        this.view.domNode.focus();
    }
    onCtrlA(e) {
        e.preventDefault();
        e.stopPropagation();
        this.list.setSelection(range(this.list.length), e.browserEvent);
        this.list.setAnchor(undefined);
        this.view.domNode.focus();
    }
    onEscape(e) {
        if (this.list.getSelection().length) {
            e.preventDefault();
            e.stopPropagation();
            this.list.setSelection([], e.browserEvent);
            this.list.setAnchor(undefined);
            this.view.domNode.focus();
        }
    }
    dispose() {
        this.disposables.dispose();
        this.multipleSelectionDisposables.dispose();
    }
}
__decorate([
    memoize
], KeyboardController.prototype, "onKeyDown", null);
export var TypeNavigationMode;
(function (TypeNavigationMode) {
    TypeNavigationMode[TypeNavigationMode["Automatic"] = 0] = "Automatic";
    TypeNavigationMode[TypeNavigationMode["Trigger"] = 1] = "Trigger";
})(TypeNavigationMode || (TypeNavigationMode = {}));
var TypeNavigationControllerState;
(function (TypeNavigationControllerState) {
    TypeNavigationControllerState[TypeNavigationControllerState["Idle"] = 0] = "Idle";
    TypeNavigationControllerState[TypeNavigationControllerState["Typing"] = 1] = "Typing";
})(TypeNavigationControllerState || (TypeNavigationControllerState = {}));
export const DefaultKeyboardNavigationDelegate = new class {
    mightProducePrintableCharacter(event) {
        if (event.ctrlKey || event.metaKey || event.altKey) {
            return false;
        }
        return (event.keyCode >= 31 /* KeyCode.KeyA */ && event.keyCode <= 56 /* KeyCode.KeyZ */)
            || (event.keyCode >= 21 /* KeyCode.Digit0 */ && event.keyCode <= 30 /* KeyCode.Digit9 */)
            || (event.keyCode >= 93 /* KeyCode.Numpad0 */ && event.keyCode <= 102 /* KeyCode.Numpad9 */)
            || (event.keyCode >= 80 /* KeyCode.Semicolon */ && event.keyCode <= 90 /* KeyCode.Quote */);
    }
};
class TypeNavigationController {
    constructor(list, view, keyboardNavigationLabelProvider, keyboardNavigationEventFilter, delegate) {
        this.list = list;
        this.view = view;
        this.keyboardNavigationLabelProvider = keyboardNavigationLabelProvider;
        this.keyboardNavigationEventFilter = keyboardNavigationEventFilter;
        this.delegate = delegate;
        this.enabled = false;
        this.state = TypeNavigationControllerState.Idle;
        this.mode = TypeNavigationMode.Automatic;
        this.triggered = false;
        this.previouslyFocused = -1;
        this.enabledDisposables = new DisposableStore();
        this.disposables = new DisposableStore();
        this.updateOptions(list.options);
    }
    updateOptions(options) {
        var _a, _b;
        if ((_a = options.typeNavigationEnabled) !== null && _a !== void 0 ? _a : true) {
            this.enable();
        }
        else {
            this.disable();
        }
        this.mode = (_b = options.typeNavigationMode) !== null && _b !== void 0 ? _b : TypeNavigationMode.Automatic;
    }
    enable() {
        if (this.enabled) {
            return;
        }
        let typing = false;
        const onChar = this.enabledDisposables.add(Event.chain(this.enabledDisposables.add(new DomEmitter(this.view.domNode, 'keydown')).event))
            .filter(e => !isInputElement(e.target))
            .filter(() => this.mode === TypeNavigationMode.Automatic || this.triggered)
            .map(event => new StandardKeyboardEvent(event))
            .filter(e => typing || this.keyboardNavigationEventFilter(e))
            .filter(e => this.delegate.mightProducePrintableCharacter(e))
            .forEach(stopEvent)
            .map(event => event.browserEvent.key)
            .event;
        const onClear = Event.debounce(onChar, () => null, 800, undefined, undefined, this.enabledDisposables);
        const onInput = Event.reduce(Event.any(onChar, onClear), (r, i) => i === null ? null : ((r || '') + i), undefined, this.enabledDisposables);
        onInput(this.onInput, this, this.enabledDisposables);
        onClear(this.onClear, this, this.enabledDisposables);
        onChar(() => typing = true, undefined, this.enabledDisposables);
        onClear(() => typing = false, undefined, this.enabledDisposables);
        this.enabled = true;
        this.triggered = false;
    }
    disable() {
        if (!this.enabled) {
            return;
        }
        this.enabledDisposables.clear();
        this.enabled = false;
        this.triggered = false;
    }
    onClear() {
        var _a;
        const focus = this.list.getFocus();
        if (focus.length > 0 && focus[0] === this.previouslyFocused) {
            // List: re-announce element on typing end since typed keys will interrupt aria label of focused element
            // Do not announce if there was a focus change at the end to prevent duplication https://github.com/microsoft/vscode/issues/95961
            const ariaLabel = (_a = this.list.options.accessibilityProvider) === null || _a === void 0 ? void 0 : _a.getAriaLabel(this.list.element(focus[0]));
            if (ariaLabel) {
                alert(ariaLabel);
            }
        }
        this.previouslyFocused = -1;
    }
    onInput(word) {
        if (!word) {
            this.state = TypeNavigationControllerState.Idle;
            this.triggered = false;
            return;
        }
        const focus = this.list.getFocus();
        const start = focus.length > 0 ? focus[0] : 0;
        const delta = this.state === TypeNavigationControllerState.Idle ? 1 : 0;
        this.state = TypeNavigationControllerState.Typing;
        for (let i = 0; i < this.list.length; i++) {
            const index = (start + i + delta) % this.list.length;
            const label = this.keyboardNavigationLabelProvider.getKeyboardNavigationLabel(this.view.element(index));
            const labelStr = label && label.toString();
            if (typeof labelStr === 'undefined' || matchesPrefix(word, labelStr)) {
                this.previouslyFocused = start;
                this.list.setFocus([index]);
                this.list.reveal(index);
                return;
            }
        }
    }
    dispose() {
        this.disable();
        this.enabledDisposables.dispose();
        this.disposables.dispose();
    }
}
class DOMFocusController {
    constructor(list, view) {
        this.list = list;
        this.view = view;
        this.disposables = new DisposableStore();
        const onKeyDown = this.disposables.add(Event.chain(this.disposables.add(new DomEmitter(view.domNode, 'keydown')).event))
            .filter(e => !isInputElement(e.target))
            .map(e => new StandardKeyboardEvent(e));
        onKeyDown.filter(e => e.keyCode === 2 /* KeyCode.Tab */ && !e.ctrlKey && !e.metaKey && !e.shiftKey && !e.altKey)
            .on(this.onTab, this, this.disposables);
    }
    onTab(e) {
        if (e.target !== this.view.domNode) {
            return;
        }
        const focus = this.list.getFocus();
        if (focus.length === 0) {
            return;
        }
        const focusedDomElement = this.view.domElement(focus[0]);
        if (!focusedDomElement) {
            return;
        }
        const tabIndexElement = focusedDomElement.querySelector('[tabIndex]');
        if (!tabIndexElement || !(tabIndexElement instanceof HTMLElement) || tabIndexElement.tabIndex === -1) {
            return;
        }
        const style = window.getComputedStyle(tabIndexElement);
        if (style.visibility === 'hidden' || style.display === 'none') {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        tabIndexElement.focus();
    }
    dispose() {
        this.disposables.dispose();
    }
}
export function isSelectionSingleChangeEvent(event) {
    return platform.isMacintosh ? event.browserEvent.metaKey : event.browserEvent.ctrlKey;
}
export function isSelectionRangeChangeEvent(event) {
    return event.browserEvent.shiftKey;
}
function isMouseRightClick(event) {
    return event instanceof MouseEvent && event.button === 2;
}
const DefaultMultipleSelectionController = {
    isSelectionSingleChangeEvent,
    isSelectionRangeChangeEvent
};
export class MouseController {
    constructor(list) {
        this.list = list;
        this.disposables = new DisposableStore();
        this._onPointer = new Emitter();
        this.onPointer = this._onPointer.event;
        if (list.options.multipleSelectionSupport !== false) {
            this.multipleSelectionController = this.list.options.multipleSelectionController || DefaultMultipleSelectionController;
        }
        this.mouseSupport = typeof list.options.mouseSupport === 'undefined' || !!list.options.mouseSupport;
        if (this.mouseSupport) {
            list.onMouseDown(this.onMouseDown, this, this.disposables);
            list.onContextMenu(this.onContextMenu, this, this.disposables);
            list.onMouseDblClick(this.onDoubleClick, this, this.disposables);
            list.onTouchStart(this.onMouseDown, this, this.disposables);
            this.disposables.add(Gesture.addTarget(list.getHTMLElement()));
        }
        Event.any(list.onMouseClick, list.onMouseMiddleClick, list.onTap)(this.onViewPointer, this, this.disposables);
    }
    updateOptions(optionsUpdate) {
        if (optionsUpdate.multipleSelectionSupport !== undefined) {
            this.multipleSelectionController = undefined;
            if (optionsUpdate.multipleSelectionSupport) {
                this.multipleSelectionController = this.list.options.multipleSelectionController || DefaultMultipleSelectionController;
            }
        }
    }
    isSelectionSingleChangeEvent(event) {
        if (!this.multipleSelectionController) {
            return false;
        }
        return this.multipleSelectionController.isSelectionSingleChangeEvent(event);
    }
    isSelectionRangeChangeEvent(event) {
        if (!this.multipleSelectionController) {
            return false;
        }
        return this.multipleSelectionController.isSelectionRangeChangeEvent(event);
    }
    isSelectionChangeEvent(event) {
        return this.isSelectionSingleChangeEvent(event) || this.isSelectionRangeChangeEvent(event);
    }
    onMouseDown(e) {
        if (isMonacoEditor(e.browserEvent.target)) {
            return;
        }
        if (document.activeElement !== e.browserEvent.target) {
            this.list.domFocus();
        }
    }
    onContextMenu(e) {
        if (isMonacoEditor(e.browserEvent.target)) {
            return;
        }
        const focus = typeof e.index === 'undefined' ? [] : [e.index];
        this.list.setFocus(focus, e.browserEvent);
    }
    onViewPointer(e) {
        if (!this.mouseSupport) {
            return;
        }
        if (isInputElement(e.browserEvent.target) || isMonacoEditor(e.browserEvent.target)) {
            return;
        }
        const focus = e.index;
        if (typeof focus === 'undefined') {
            this.list.setFocus([], e.browserEvent);
            this.list.setSelection([], e.browserEvent);
            this.list.setAnchor(undefined);
            return;
        }
        if (this.isSelectionRangeChangeEvent(e)) {
            return this.changeSelection(e);
        }
        if (this.isSelectionChangeEvent(e)) {
            return this.changeSelection(e);
        }
        this.list.setFocus([focus], e.browserEvent);
        this.list.setAnchor(focus);
        if (!isMouseRightClick(e.browserEvent)) {
            this.list.setSelection([focus], e.browserEvent);
        }
        this._onPointer.fire(e);
    }
    onDoubleClick(e) {
        if (isInputElement(e.browserEvent.target) || isMonacoEditor(e.browserEvent.target)) {
            return;
        }
        if (this.isSelectionChangeEvent(e)) {
            return;
        }
        const focus = this.list.getFocus();
        this.list.setSelection(focus, e.browserEvent);
    }
    changeSelection(e) {
        const focus = e.index;
        let anchor = this.list.getAnchor();
        if (this.isSelectionRangeChangeEvent(e)) {
            if (typeof anchor === 'undefined') {
                const currentFocus = this.list.getFocus()[0];
                anchor = currentFocus !== null && currentFocus !== void 0 ? currentFocus : focus;
                this.list.setAnchor(anchor);
            }
            const min = Math.min(anchor, focus);
            const max = Math.max(anchor, focus);
            const rangeSelection = range(min, max + 1);
            const selection = this.list.getSelection();
            const contiguousRange = getContiguousRangeContaining(disjunction(selection, [anchor]), anchor);
            if (contiguousRange.length === 0) {
                return;
            }
            const newSelection = disjunction(rangeSelection, relativeComplement(selection, contiguousRange));
            this.list.setSelection(newSelection, e.browserEvent);
            this.list.setFocus([focus], e.browserEvent);
        }
        else if (this.isSelectionSingleChangeEvent(e)) {
            const selection = this.list.getSelection();
            const newSelection = selection.filter(i => i !== focus);
            this.list.setFocus([focus]);
            this.list.setAnchor(focus);
            if (selection.length === newSelection.length) {
                this.list.setSelection([...newSelection, focus], e.browserEvent);
            }
            else {
                this.list.setSelection(newSelection, e.browserEvent);
            }
        }
    }
    dispose() {
        this.disposables.dispose();
    }
}
export class DefaultStyleController {
    constructor(styleElement, selectorSuffix) {
        this.styleElement = styleElement;
        this.selectorSuffix = selectorSuffix;
    }
    style(styles) {
        const suffix = this.selectorSuffix && `.${this.selectorSuffix}`;
        const content = [];
        if (styles.listBackground) {
            if (styles.listBackground.isOpaque()) {
                content.push(`.monaco-list${suffix} .monaco-list-rows { background: ${styles.listBackground}; }`);
            }
            else if (!platform.isMacintosh) { // subpixel AA doesn't exist in macOS
                console.warn(`List with id '${this.selectorSuffix}' was styled with a non-opaque background color. This will break sub-pixel antialiasing.`);
            }
        }
        if (styles.listFocusBackground) {
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.focused { background-color: ${styles.listFocusBackground}; }`);
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.focused:hover { background-color: ${styles.listFocusBackground}; }`); // overwrite :hover style in this case!
        }
        if (styles.listFocusForeground) {
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.focused { color: ${styles.listFocusForeground}; }`);
        }
        if (styles.listActiveSelectionBackground) {
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.selected { background-color: ${styles.listActiveSelectionBackground}; }`);
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.selected:hover { background-color: ${styles.listActiveSelectionBackground}; }`); // overwrite :hover style in this case!
        }
        if (styles.listActiveSelectionForeground) {
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.selected { color: ${styles.listActiveSelectionForeground}; }`);
        }
        if (styles.listActiveSelectionIconForeground) {
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.selected .codicon { color: ${styles.listActiveSelectionIconForeground}; }`);
        }
        if (styles.listFocusAndSelectionOutline) {
            content.push(`.monaco-list${suffix}:focus .monaco-list-row.selected { outline-color: ${styles.listFocusAndSelectionOutline} !important; }`);
        }
        if (styles.listFocusAndSelectionBackground) {
            content.push(`
				.monaco-drag-image,
				.monaco-list${suffix}:focus .monaco-list-row.selected.focused { background-color: ${styles.listFocusAndSelectionBackground}; }
			`);
        }
        if (styles.listFocusAndSelectionForeground) {
            content.push(`
				.monaco-drag-image,
				.monaco-list${suffix}:focus .monaco-list-row.selected.focused { color: ${styles.listFocusAndSelectionForeground}; }
			`);
        }
        if (styles.listInactiveFocusForeground) {
            content.push(`.monaco-list${suffix} .monaco-list-row.focused { color:  ${styles.listInactiveFocusForeground}; }`);
            content.push(`.monaco-list${suffix} .monaco-list-row.focused:hover { color:  ${styles.listInactiveFocusForeground}; }`); // overwrite :hover style in this case!
        }
        if (styles.listInactiveSelectionIconForeground) {
            content.push(`.monaco-list${suffix} .monaco-list-row.focused .codicon { color:  ${styles.listInactiveSelectionIconForeground}; }`);
        }
        if (styles.listInactiveFocusBackground) {
            content.push(`.monaco-list${suffix} .monaco-list-row.focused { background-color:  ${styles.listInactiveFocusBackground}; }`);
            content.push(`.monaco-list${suffix} .monaco-list-row.focused:hover { background-color:  ${styles.listInactiveFocusBackground}; }`); // overwrite :hover style in this case!
        }
        if (styles.listInactiveSelectionBackground) {
            content.push(`.monaco-list${suffix} .monaco-list-row.selected { background-color:  ${styles.listInactiveSelectionBackground}; }`);
            content.push(`.monaco-list${suffix} .monaco-list-row.selected:hover { background-color:  ${styles.listInactiveSelectionBackground}; }`); // overwrite :hover style in this case!
        }
        if (styles.listInactiveSelectionForeground) {
            content.push(`.monaco-list${suffix} .monaco-list-row.selected { color: ${styles.listInactiveSelectionForeground}; }`);
        }
        if (styles.listHoverBackground) {
            content.push(`.monaco-list${suffix}:not(.drop-target) .monaco-list-row:hover:not(.selected):not(.focused) { background-color: ${styles.listHoverBackground}; }`);
        }
        if (styles.listHoverForeground) {
            content.push(`.monaco-list${suffix} .monaco-list-row:hover:not(.selected):not(.focused) { color:  ${styles.listHoverForeground}; }`);
        }
        if (styles.listSelectionOutline) {
            content.push(`.monaco-list${suffix} .monaco-list-row.selected { outline: 1px dotted ${styles.listSelectionOutline}; outline-offset: -1px; }`);
        }
        if (styles.listFocusOutline) {
            content.push(`
				.monaco-drag-image,
				.monaco-list${suffix}:focus .monaco-list-row.focused { outline: 1px solid ${styles.listFocusOutline}; outline-offset: -1px; }
				.monaco-workbench.context-menu-visible .monaco-list${suffix}.last-focused .monaco-list-row.focused { outline: 1px solid ${styles.listFocusOutline}; outline-offset: -1px; }
			`);
        }
        if (styles.listInactiveFocusOutline) {
            content.push(`.monaco-list${suffix} .monaco-list-row.focused { outline: 1px dotted ${styles.listInactiveFocusOutline}; outline-offset: -1px; }`);
        }
        if (styles.listHoverOutline) {
            content.push(`.monaco-list${suffix} .monaco-list-row:hover { outline: 1px dashed ${styles.listHoverOutline}; outline-offset: -1px; }`);
        }
        if (styles.listDropBackground) {
            content.push(`
				.monaco-list${suffix}.drop-target,
				.monaco-list${suffix} .monaco-list-rows.drop-target,
				.monaco-list${suffix} .monaco-list-row.drop-target { background-color: ${styles.listDropBackground} !important; color: inherit !important; }
			`);
        }
        if (styles.tableColumnsBorder) {
            content.push(`
				.monaco-table:hover > .monaco-split-view2,
				.monaco-table:hover > .monaco-split-view2 .monaco-sash.vertical::before {
					border-color: ${styles.tableColumnsBorder};
			}`);
        }
        if (styles.tableOddRowsBackgroundColor) {
            content.push(`
				.monaco-table .monaco-list-row[data-parity=odd]:not(.focused):not(.selected):not(:hover) .monaco-table-tr,
				.monaco-table .monaco-list:not(:focus) .monaco-list-row[data-parity=odd].focused:not(.selected):not(:hover) .monaco-table-tr,
				.monaco-table .monaco-list:not(.focused) .monaco-list-row[data-parity=odd].focused:not(.selected):not(:hover) .monaco-table-tr {
					background-color: ${styles.tableOddRowsBackgroundColor};
				}
			`);
        }
        this.styleElement.textContent = content.join('\n');
    }
}
const defaultStyles = {
    listFocusBackground: Color.fromHex('#7FB0D0'),
    listActiveSelectionBackground: Color.fromHex('#0E639C'),
    listActiveSelectionForeground: Color.fromHex('#FFFFFF'),
    listActiveSelectionIconForeground: Color.fromHex('#FFFFFF'),
    listFocusAndSelectionOutline: Color.fromHex('#90C2F9'),
    listFocusAndSelectionBackground: Color.fromHex('#094771'),
    listFocusAndSelectionForeground: Color.fromHex('#FFFFFF'),
    listInactiveSelectionBackground: Color.fromHex('#3F3F46'),
    listInactiveSelectionIconForeground: Color.fromHex('#FFFFFF'),
    listHoverBackground: Color.fromHex('#2A2D2E'),
    listDropBackground: Color.fromHex('#383B3D'),
    treeIndentGuidesStroke: Color.fromHex('#a9a9a9'),
    tableColumnsBorder: Color.fromHex('#cccccc').transparent(0.2),
    tableOddRowsBackgroundColor: Color.fromHex('#cccccc').transparent(0.04)
};
const DefaultOptions = {
    keyboardSupport: true,
    mouseSupport: true,
    multipleSelectionSupport: true,
    dnd: {
        getDragURI() { return null; },
        onDragStart() { },
        onDragOver() { return false; },
        drop() { }
    }
};
// TODO@Joao: move these utils into a SortedArray class
function getContiguousRangeContaining(range, value) {
    const index = range.indexOf(value);
    if (index === -1) {
        return [];
    }
    const result = [];
    let i = index - 1;
    while (i >= 0 && range[i] === value - (index - i)) {
        result.push(range[i--]);
    }
    result.reverse();
    i = index;
    while (i < range.length && range[i] === value + (i - index)) {
        result.push(range[i++]);
    }
    return result;
}
/**
 * Given two sorted collections of numbers, returns the intersection
 * between them (OR).
 */
function disjunction(one, other) {
    const result = [];
    let i = 0, j = 0;
    while (i < one.length || j < other.length) {
        if (i >= one.length) {
            result.push(other[j++]);
        }
        else if (j >= other.length) {
            result.push(one[i++]);
        }
        else if (one[i] === other[j]) {
            result.push(one[i]);
            i++;
            j++;
            continue;
        }
        else if (one[i] < other[j]) {
            result.push(one[i++]);
        }
        else {
            result.push(other[j++]);
        }
    }
    return result;
}
/**
 * Given two sorted collections of numbers, returns the relative
 * complement between them (XOR).
 */
function relativeComplement(one, other) {
    const result = [];
    let i = 0, j = 0;
    while (i < one.length || j < other.length) {
        if (i >= one.length) {
            result.push(other[j++]);
        }
        else if (j >= other.length) {
            result.push(one[i++]);
        }
        else if (one[i] === other[j]) {
            i++;
            j++;
            continue;
        }
        else if (one[i] < other[j]) {
            result.push(one[i++]);
        }
        else {
            j++;
        }
    }
    return result;
}
const numericSort = (a, b) => a - b;
class PipelineRenderer {
    constructor(_templateId, renderers) {
        this._templateId = _templateId;
        this.renderers = renderers;
    }
    get templateId() {
        return this._templateId;
    }
    renderTemplate(container) {
        return this.renderers.map(r => r.renderTemplate(container));
    }
    renderElement(element, index, templateData, height) {
        let i = 0;
        for (const renderer of this.renderers) {
            renderer.renderElement(element, index, templateData[i++], height);
        }
    }
    disposeElement(element, index, templateData, height) {
        var _a;
        let i = 0;
        for (const renderer of this.renderers) {
            (_a = renderer.disposeElement) === null || _a === void 0 ? void 0 : _a.call(renderer, element, index, templateData[i], height);
            i += 1;
        }
    }
    disposeTemplate(templateData) {
        let i = 0;
        for (const renderer of this.renderers) {
            renderer.disposeTemplate(templateData[i++]);
        }
    }
}
class AccessibiltyRenderer {
    constructor(accessibilityProvider) {
        this.accessibilityProvider = accessibilityProvider;
        this.templateId = 'a18n';
    }
    renderTemplate(container) {
        return container;
    }
    renderElement(element, index, container) {
        const ariaLabel = this.accessibilityProvider.getAriaLabel(element);
        if (ariaLabel) {
            container.setAttribute('aria-label', ariaLabel);
        }
        else {
            container.removeAttribute('aria-label');
        }
        const ariaLevel = this.accessibilityProvider.getAriaLevel && this.accessibilityProvider.getAriaLevel(element);
        if (typeof ariaLevel === 'number') {
            container.setAttribute('aria-level', `${ariaLevel}`);
        }
        else {
            container.removeAttribute('aria-level');
        }
    }
    disposeTemplate(templateData) {
        // noop
    }
}
class ListViewDragAndDrop {
    constructor(list, dnd) {
        this.list = list;
        this.dnd = dnd;
    }
    getDragElements(element) {
        const selection = this.list.getSelectedElements();
        const elements = selection.indexOf(element) > -1 ? selection : [element];
        return elements;
    }
    getDragURI(element) {
        return this.dnd.getDragURI(element);
    }
    getDragLabel(elements, originalEvent) {
        if (this.dnd.getDragLabel) {
            return this.dnd.getDragLabel(elements, originalEvent);
        }
        return undefined;
    }
    onDragStart(data, originalEvent) {
        var _a, _b;
        (_b = (_a = this.dnd).onDragStart) === null || _b === void 0 ? void 0 : _b.call(_a, data, originalEvent);
    }
    onDragOver(data, targetElement, targetIndex, originalEvent) {
        return this.dnd.onDragOver(data, targetElement, targetIndex, originalEvent);
    }
    onDragLeave(data, targetElement, targetIndex, originalEvent) {
        var _a, _b;
        (_b = (_a = this.dnd).onDragLeave) === null || _b === void 0 ? void 0 : _b.call(_a, data, targetElement, targetIndex, originalEvent);
    }
    onDragEnd(originalEvent) {
        var _a, _b;
        (_b = (_a = this.dnd).onDragEnd) === null || _b === void 0 ? void 0 : _b.call(_a, originalEvent);
    }
    drop(data, targetElement, targetIndex, originalEvent) {
        this.dnd.drop(data, targetElement, targetIndex, originalEvent);
    }
}
/**
 * The {@link List} is a virtual scrolling widget, built on top of the {@link ListView}
 * widget.
 *
 * Features:
 * - Customizable keyboard and mouse support
 * - Element traits: focus, selection, achor
 * - Accessibility support
 * - Touch support
 * - Performant template-based rendering
 * - Horizontal scrolling
 * - Variable element height support
 * - Dynamic element height support
 * - Drag-and-drop support
 */
export class List {
    constructor(user, container, virtualDelegate, renderers, _options = DefaultOptions) {
        var _a, _b, _c, _d;
        this.user = user;
        this._options = _options;
        this.focus = new Trait('focused');
        this.anchor = new Trait('anchor');
        this.eventBufferer = new EventBufferer();
        this._ariaLabel = '';
        this.disposables = new DisposableStore();
        this._onDidDispose = new Emitter();
        this.onDidDispose = this._onDidDispose.event;
        const role = this._options.accessibilityProvider && this._options.accessibilityProvider.getWidgetRole ? (_a = this._options.accessibilityProvider) === null || _a === void 0 ? void 0 : _a.getWidgetRole() : 'list';
        this.selection = new SelectionTrait(role !== 'listbox');
        mixin(_options, defaultStyles, false);
        const baseRenderers = [this.focus.renderer, this.selection.renderer];
        this.accessibilityProvider = _options.accessibilityProvider;
        if (this.accessibilityProvider) {
            baseRenderers.push(new AccessibiltyRenderer(this.accessibilityProvider));
            (_c = (_b = this.accessibilityProvider).onDidChangeActiveDescendant) === null || _c === void 0 ? void 0 : _c.call(_b, this.onDidChangeActiveDescendant, this, this.disposables);
        }
        renderers = renderers.map(r => new PipelineRenderer(r.templateId, [...baseRenderers, r]));
        const viewOptions = Object.assign(Object.assign({}, _options), { dnd: _options.dnd && new ListViewDragAndDrop(this, _options.dnd) });
        this.view = new ListView(container, virtualDelegate, renderers, viewOptions);
        this.view.domNode.setAttribute('role', role);
        if (_options.styleController) {
            this.styleController = _options.styleController(this.view.domId);
        }
        else {
            const styleElement = createStyleSheet(this.view.domNode);
            this.styleController = new DefaultStyleController(styleElement, this.view.domId);
        }
        this.spliceable = new CombinedSpliceable([
            new TraitSpliceable(this.focus, this.view, _options.identityProvider),
            new TraitSpliceable(this.selection, this.view, _options.identityProvider),
            new TraitSpliceable(this.anchor, this.view, _options.identityProvider),
            this.view
        ]);
        this.disposables.add(this.focus);
        this.disposables.add(this.selection);
        this.disposables.add(this.anchor);
        this.disposables.add(this.view);
        this.disposables.add(this._onDidDispose);
        this.disposables.add(new DOMFocusController(this, this.view));
        if (typeof _options.keyboardSupport !== 'boolean' || _options.keyboardSupport) {
            this.keyboardController = new KeyboardController(this, this.view, _options);
            this.disposables.add(this.keyboardController);
        }
        if (_options.keyboardNavigationLabelProvider) {
            const delegate = _options.keyboardNavigationDelegate || DefaultKeyboardNavigationDelegate;
            this.typeNavigationController = new TypeNavigationController(this, this.view, _options.keyboardNavigationLabelProvider, (_d = _options.keyboardNavigationEventFilter) !== null && _d !== void 0 ? _d : (() => true), delegate);
            this.disposables.add(this.typeNavigationController);
        }
        this.mouseController = this.createMouseController(_options);
        this.disposables.add(this.mouseController);
        this.onDidChangeFocus(this._onFocusChange, this, this.disposables);
        this.onDidChangeSelection(this._onSelectionChange, this, this.disposables);
        if (this.accessibilityProvider) {
            this.ariaLabel = this.accessibilityProvider.getWidgetAriaLabel();
        }
        if (this._options.multipleSelectionSupport !== false) {
            this.view.domNode.setAttribute('aria-multiselectable', 'true');
        }
    }
    get onDidChangeFocus() {
        return Event.map(this.eventBufferer.wrapEvent(this.focus.onChange), e => this.toListEvent(e), this.disposables);
    }
    get onDidChangeSelection() {
        return Event.map(this.eventBufferer.wrapEvent(this.selection.onChange), e => this.toListEvent(e), this.disposables);
    }
    get domId() { return this.view.domId; }
    get onMouseClick() { return this.view.onMouseClick; }
    get onMouseDblClick() { return this.view.onMouseDblClick; }
    get onMouseMiddleClick() { return this.view.onMouseMiddleClick; }
    get onPointer() { return this.mouseController.onPointer; }
    get onMouseDown() { return this.view.onMouseDown; }
    get onMouseOver() { return this.view.onMouseOver; }
    get onTouchStart() { return this.view.onTouchStart; }
    get onTap() { return this.view.onTap; }
    /**
     * Possible context menu trigger events:
     * - ContextMenu key
     * - Shift F10
     * - Ctrl Option Shift M (macOS with VoiceOver)
     * - Mouse right click
     */
    get onContextMenu() {
        let didJustPressContextMenuKey = false;
        const fromKeyDown = this.disposables.add(Event.chain(this.disposables.add(new DomEmitter(this.view.domNode, 'keydown')).event))
            .map(e => new StandardKeyboardEvent(e))
            .filter(e => didJustPressContextMenuKey = e.keyCode === 58 /* KeyCode.ContextMenu */ || (e.shiftKey && e.keyCode === 68 /* KeyCode.F10 */))
            .map(stopEvent)
            .filter(() => false)
            .event;
        const fromKeyUp = this.disposables.add(Event.chain(this.disposables.add(new DomEmitter(this.view.domNode, 'keyup')).event))
            .forEach(() => didJustPressContextMenuKey = false)
            .map(e => new StandardKeyboardEvent(e))
            .filter(e => e.keyCode === 58 /* KeyCode.ContextMenu */ || (e.shiftKey && e.keyCode === 68 /* KeyCode.F10 */))
            .map(stopEvent)
            .map(({ browserEvent }) => {
            const focus = this.getFocus();
            const index = focus.length ? focus[0] : undefined;
            const element = typeof index !== 'undefined' ? this.view.element(index) : undefined;
            const anchor = typeof index !== 'undefined' ? this.view.domElement(index) : this.view.domNode;
            return { index, element, anchor, browserEvent };
        })
            .event;
        const fromMouse = this.disposables.add(Event.chain(this.view.onContextMenu))
            .filter(_ => !didJustPressContextMenuKey)
            .map(({ element, index, browserEvent }) => ({ element, index, anchor: { x: browserEvent.pageX + 1, y: browserEvent.pageY }, browserEvent }))
            .event;
        return Event.any(fromKeyDown, fromKeyUp, fromMouse);
    }
    get onKeyDown() { return this.disposables.add(new DomEmitter(this.view.domNode, 'keydown')).event; }
    get onDidFocus() { return Event.signal(this.disposables.add(new DomEmitter(this.view.domNode, 'focus', true)).event); }
    createMouseController(options) {
        return new MouseController(this);
    }
    updateOptions(optionsUpdate = {}) {
        var _a, _b;
        this._options = Object.assign(Object.assign({}, this._options), optionsUpdate);
        (_a = this.typeNavigationController) === null || _a === void 0 ? void 0 : _a.updateOptions(this._options);
        if (this._options.multipleSelectionController !== undefined) {
            if (this._options.multipleSelectionSupport) {
                this.view.domNode.setAttribute('aria-multiselectable', 'true');
            }
            else {
                this.view.domNode.removeAttribute('aria-multiselectable');
            }
        }
        this.mouseController.updateOptions(optionsUpdate);
        (_b = this.keyboardController) === null || _b === void 0 ? void 0 : _b.updateOptions(optionsUpdate);
        this.view.updateOptions(optionsUpdate);
    }
    get options() {
        return this._options;
    }
    splice(start, deleteCount, elements = []) {
        if (start < 0 || start > this.view.length) {
            throw new ListError(this.user, `Invalid start index: ${start}`);
        }
        if (deleteCount < 0) {
            throw new ListError(this.user, `Invalid delete count: ${deleteCount}`);
        }
        if (deleteCount === 0 && elements.length === 0) {
            return;
        }
        this.eventBufferer.bufferEvents(() => this.spliceable.splice(start, deleteCount, elements));
    }
    rerender() {
        this.view.rerender();
    }
    element(index) {
        return this.view.element(index);
    }
    get length() {
        return this.view.length;
    }
    get contentHeight() {
        return this.view.contentHeight;
    }
    get scrollTop() {
        return this.view.getScrollTop();
    }
    set scrollTop(scrollTop) {
        this.view.setScrollTop(scrollTop);
    }
    get ariaLabel() {
        return this._ariaLabel;
    }
    set ariaLabel(value) {
        this._ariaLabel = value;
        this.view.domNode.setAttribute('aria-label', value);
    }
    domFocus() {
        this.view.domNode.focus({ preventScroll: true });
    }
    layout(height, width) {
        this.view.layout(height, width);
    }
    setSelection(indexes, browserEvent) {
        for (const index of indexes) {
            if (index < 0 || index >= this.length) {
                throw new ListError(this.user, `Invalid index ${index}`);
            }
        }
        this.selection.set(indexes, browserEvent);
    }
    getSelection() {
        return this.selection.get();
    }
    getSelectedElements() {
        return this.getSelection().map(i => this.view.element(i));
    }
    setAnchor(index) {
        if (typeof index === 'undefined') {
            this.anchor.set([]);
            return;
        }
        if (index < 0 || index >= this.length) {
            throw new ListError(this.user, `Invalid index ${index}`);
        }
        this.anchor.set([index]);
    }
    getAnchor() {
        return firstOrDefault(this.anchor.get(), undefined);
    }
    getAnchorElement() {
        const anchor = this.getAnchor();
        return typeof anchor === 'undefined' ? undefined : this.element(anchor);
    }
    setFocus(indexes, browserEvent) {
        for (const index of indexes) {
            if (index < 0 || index >= this.length) {
                throw new ListError(this.user, `Invalid index ${index}`);
            }
        }
        this.focus.set(indexes, browserEvent);
    }
    focusNext(n = 1, loop = false, browserEvent, filter) {
        if (this.length === 0) {
            return;
        }
        const focus = this.focus.get();
        const index = this.findNextIndex(focus.length > 0 ? focus[0] + n : 0, loop, filter);
        if (index > -1) {
            this.setFocus([index], browserEvent);
        }
    }
    focusPrevious(n = 1, loop = false, browserEvent, filter) {
        if (this.length === 0) {
            return;
        }
        const focus = this.focus.get();
        const index = this.findPreviousIndex(focus.length > 0 ? focus[0] - n : 0, loop, filter);
        if (index > -1) {
            this.setFocus([index], browserEvent);
        }
    }
    focusNextPage(browserEvent, filter) {
        return __awaiter(this, void 0, void 0, function* () {
            let lastPageIndex = this.view.indexAt(this.view.getScrollTop() + this.view.renderHeight);
            lastPageIndex = lastPageIndex === 0 ? 0 : lastPageIndex - 1;
            const currentlyFocusedElementIndex = this.getFocus()[0];
            if (currentlyFocusedElementIndex !== lastPageIndex && (currentlyFocusedElementIndex === undefined || lastPageIndex > currentlyFocusedElementIndex)) {
                const lastGoodPageIndex = this.findPreviousIndex(lastPageIndex, false, filter);
                if (lastGoodPageIndex > -1 && currentlyFocusedElementIndex !== lastGoodPageIndex) {
                    this.setFocus([lastGoodPageIndex], browserEvent);
                }
                else {
                    this.setFocus([lastPageIndex], browserEvent);
                }
            }
            else {
                const previousScrollTop = this.view.getScrollTop();
                let nextpageScrollTop = previousScrollTop + this.view.renderHeight;
                if (lastPageIndex > currentlyFocusedElementIndex) {
                    // scroll last page element to the top only if the last page element is below the focused element
                    nextpageScrollTop -= this.view.elementHeight(lastPageIndex);
                }
                this.view.setScrollTop(nextpageScrollTop);
                if (this.view.getScrollTop() !== previousScrollTop) {
                    this.setFocus([]);
                    // Let the scroll event listener run
                    yield timeout(0);
                    yield this.focusNextPage(browserEvent, filter);
                }
            }
        });
    }
    focusPreviousPage(browserEvent, filter) {
        return __awaiter(this, void 0, void 0, function* () {
            let firstPageIndex;
            const scrollTop = this.view.getScrollTop();
            if (scrollTop === 0) {
                firstPageIndex = this.view.indexAt(scrollTop);
            }
            else {
                firstPageIndex = this.view.indexAfter(scrollTop - 1);
            }
            const currentlyFocusedElementIndex = this.getFocus()[0];
            if (currentlyFocusedElementIndex !== firstPageIndex && (currentlyFocusedElementIndex === undefined || currentlyFocusedElementIndex >= firstPageIndex)) {
                const firstGoodPageIndex = this.findNextIndex(firstPageIndex, false, filter);
                if (firstGoodPageIndex > -1 && currentlyFocusedElementIndex !== firstGoodPageIndex) {
                    this.setFocus([firstGoodPageIndex], browserEvent);
                }
                else {
                    this.setFocus([firstPageIndex], browserEvent);
                }
            }
            else {
                const previousScrollTop = scrollTop;
                this.view.setScrollTop(scrollTop - this.view.renderHeight);
                if (this.view.getScrollTop() !== previousScrollTop) {
                    this.setFocus([]);
                    // Let the scroll event listener run
                    yield timeout(0);
                    yield this.focusPreviousPage(browserEvent, filter);
                }
            }
        });
    }
    focusLast(browserEvent, filter) {
        if (this.length === 0) {
            return;
        }
        const index = this.findPreviousIndex(this.length - 1, false, filter);
        if (index > -1) {
            this.setFocus([index], browserEvent);
        }
    }
    focusFirst(browserEvent, filter) {
        this.focusNth(0, browserEvent, filter);
    }
    focusNth(n, browserEvent, filter) {
        if (this.length === 0) {
            return;
        }
        const index = this.findNextIndex(n, false, filter);
        if (index > -1) {
            this.setFocus([index], browserEvent);
        }
    }
    findNextIndex(index, loop = false, filter) {
        for (let i = 0; i < this.length; i++) {
            if (index >= this.length && !loop) {
                return -1;
            }
            index = index % this.length;
            if (!filter || filter(this.element(index))) {
                return index;
            }
            index++;
        }
        return -1;
    }
    findPreviousIndex(index, loop = false, filter) {
        for (let i = 0; i < this.length; i++) {
            if (index < 0 && !loop) {
                return -1;
            }
            index = (this.length + (index % this.length)) % this.length;
            if (!filter || filter(this.element(index))) {
                return index;
            }
            index--;
        }
        return -1;
    }
    getFocus() {
        return this.focus.get();
    }
    getFocusedElements() {
        return this.getFocus().map(i => this.view.element(i));
    }
    reveal(index, relativeTop) {
        if (index < 0 || index >= this.length) {
            throw new ListError(this.user, `Invalid index ${index}`);
        }
        const scrollTop = this.view.getScrollTop();
        const elementTop = this.view.elementTop(index);
        const elementHeight = this.view.elementHeight(index);
        if (isNumber(relativeTop)) {
            // y = mx + b
            const m = elementHeight - this.view.renderHeight;
            this.view.setScrollTop(m * clamp(relativeTop, 0, 1) + elementTop);
        }
        else {
            const viewItemBottom = elementTop + elementHeight;
            const scrollBottom = scrollTop + this.view.renderHeight;
            if (elementTop < scrollTop && viewItemBottom >= scrollBottom) {
                // The element is already overflowing the viewport, no-op
            }
            else if (elementTop < scrollTop || (viewItemBottom >= scrollBottom && elementHeight >= this.view.renderHeight)) {
                this.view.setScrollTop(elementTop);
            }
            else if (viewItemBottom >= scrollBottom) {
                this.view.setScrollTop(viewItemBottom - this.view.renderHeight);
            }
        }
    }
    getHTMLElement() {
        return this.view.domNode;
    }
    getElementID(index) {
        return this.view.getElementDomId(index);
    }
    style(styles) {
        this.styleController.style(styles);
    }
    toListEvent({ indexes, browserEvent }) {
        return { indexes, elements: indexes.map(i => this.view.element(i)), browserEvent };
    }
    _onFocusChange() {
        const focus = this.focus.get();
        this.view.domNode.classList.toggle('element-focused', focus.length > 0);
        this.onDidChangeActiveDescendant();
    }
    onDidChangeActiveDescendant() {
        var _a;
        const focus = this.focus.get();
        if (focus.length > 0) {
            let id;
            if ((_a = this.accessibilityProvider) === null || _a === void 0 ? void 0 : _a.getActiveDescendantId) {
                id = this.accessibilityProvider.getActiveDescendantId(this.view.element(focus[0]));
            }
            this.view.domNode.setAttribute('aria-activedescendant', id || this.view.getElementDomId(focus[0]));
        }
        else {
            this.view.domNode.removeAttribute('aria-activedescendant');
        }
    }
    _onSelectionChange() {
        const selection = this.selection.get();
        this.view.domNode.classList.toggle('selection-none', selection.length === 0);
        this.view.domNode.classList.toggle('selection-single', selection.length === 1);
        this.view.domNode.classList.toggle('selection-multiple', selection.length > 1);
    }
    dispose() {
        this._onDidDispose.fire();
        this.disposables.dispose();
        this._onDidDispose.dispose();
    }
}
__decorate([
    memoize
], List.prototype, "onDidChangeFocus", null);
__decorate([
    memoize
], List.prototype, "onDidChangeSelection", null);
__decorate([
    memoize
], List.prototype, "onContextMenu", null);
__decorate([
    memoize
], List.prototype, "onKeyDown", null);
__decorate([
    memoize
], List.prototype, "onDidFocus", null);
