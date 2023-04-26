/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import * as dom from '../../dom.js';
import { CaseSensitiveToggle, RegexToggle, WholeWordsToggle } from './findInputToggles.js';
import { HistoryInputBox } from '../inputbox/inputBox.js';
import { Widget } from '../widget.js';
import { Emitter } from '../../../common/event.js';
import './findInput.css';
import * as nls from '../../../../nls.js';
const NLS_DEFAULT_LABEL = nls.localize('defaultLabel', "input");
export class FindInput extends Widget {
    constructor(parent, contextViewProvider, _showOptionButtons, options) {
        var _a;
        super();
        this._showOptionButtons = _showOptionButtons;
        this.fixFocusOnOptionClickEnabled = true;
        this.imeSessionInProgress = false;
        this.additionalToggles = [];
        this._onDidOptionChange = this._register(new Emitter());
        this.onDidOptionChange = this._onDidOptionChange.event;
        this._onKeyDown = this._register(new Emitter());
        this.onKeyDown = this._onKeyDown.event;
        this._onMouseDown = this._register(new Emitter());
        this.onMouseDown = this._onMouseDown.event;
        this._onInput = this._register(new Emitter());
        this._onKeyUp = this._register(new Emitter());
        this._onCaseSensitiveKeyDown = this._register(new Emitter());
        this.onCaseSensitiveKeyDown = this._onCaseSensitiveKeyDown.event;
        this._onRegexKeyDown = this._register(new Emitter());
        this.onRegexKeyDown = this._onRegexKeyDown.event;
        this._lastHighlightFindOptions = 0;
        this.contextViewProvider = contextViewProvider;
        this.placeholder = options.placeholder || '';
        this.validation = options.validation;
        this.label = options.label || NLS_DEFAULT_LABEL;
        this.inputActiveOptionBorder = options.inputActiveOptionBorder;
        this.inputActiveOptionForeground = options.inputActiveOptionForeground;
        this.inputActiveOptionBackground = options.inputActiveOptionBackground;
        this.inputBackground = options.inputBackground;
        this.inputForeground = options.inputForeground;
        this.inputBorder = options.inputBorder;
        this.inputValidationInfoBorder = options.inputValidationInfoBorder;
        this.inputValidationInfoBackground = options.inputValidationInfoBackground;
        this.inputValidationInfoForeground = options.inputValidationInfoForeground;
        this.inputValidationWarningBorder = options.inputValidationWarningBorder;
        this.inputValidationWarningBackground = options.inputValidationWarningBackground;
        this.inputValidationWarningForeground = options.inputValidationWarningForeground;
        this.inputValidationErrorBorder = options.inputValidationErrorBorder;
        this.inputValidationErrorBackground = options.inputValidationErrorBackground;
        this.inputValidationErrorForeground = options.inputValidationErrorForeground;
        const appendCaseSensitiveLabel = options.appendCaseSensitiveLabel || '';
        const appendWholeWordsLabel = options.appendWholeWordsLabel || '';
        const appendRegexLabel = options.appendRegexLabel || '';
        const history = options.history || [];
        const flexibleHeight = !!options.flexibleHeight;
        const flexibleWidth = !!options.flexibleWidth;
        const flexibleMaxHeight = options.flexibleMaxHeight;
        this.domNode = document.createElement('div');
        this.domNode.classList.add('monaco-findInput');
        this.inputBox = this._register(new HistoryInputBox(this.domNode, this.contextViewProvider, {
            placeholder: this.placeholder || '',
            ariaLabel: this.label || '',
            validationOptions: {
                validation: this.validation
            },
            inputBackground: this.inputBackground,
            inputForeground: this.inputForeground,
            inputBorder: this.inputBorder,
            inputValidationInfoBackground: this.inputValidationInfoBackground,
            inputValidationInfoForeground: this.inputValidationInfoForeground,
            inputValidationInfoBorder: this.inputValidationInfoBorder,
            inputValidationWarningBackground: this.inputValidationWarningBackground,
            inputValidationWarningForeground: this.inputValidationWarningForeground,
            inputValidationWarningBorder: this.inputValidationWarningBorder,
            inputValidationErrorBackground: this.inputValidationErrorBackground,
            inputValidationErrorForeground: this.inputValidationErrorForeground,
            inputValidationErrorBorder: this.inputValidationErrorBorder,
            history,
            showHistoryHint: options.showHistoryHint,
            flexibleHeight,
            flexibleWidth,
            flexibleMaxHeight
        }));
        this.regex = this._register(new RegexToggle({
            appendTitle: appendRegexLabel,
            isChecked: false,
            inputActiveOptionBorder: this.inputActiveOptionBorder,
            inputActiveOptionForeground: this.inputActiveOptionForeground,
            inputActiveOptionBackground: this.inputActiveOptionBackground
        }));
        this._register(this.regex.onChange(viaKeyboard => {
            this._onDidOptionChange.fire(viaKeyboard);
            if (!viaKeyboard && this.fixFocusOnOptionClickEnabled) {
                this.inputBox.focus();
            }
            this.validate();
        }));
        this._register(this.regex.onKeyDown(e => {
            this._onRegexKeyDown.fire(e);
        }));
        this.wholeWords = this._register(new WholeWordsToggle({
            appendTitle: appendWholeWordsLabel,
            isChecked: false,
            inputActiveOptionBorder: this.inputActiveOptionBorder,
            inputActiveOptionForeground: this.inputActiveOptionForeground,
            inputActiveOptionBackground: this.inputActiveOptionBackground
        }));
        this._register(this.wholeWords.onChange(viaKeyboard => {
            this._onDidOptionChange.fire(viaKeyboard);
            if (!viaKeyboard && this.fixFocusOnOptionClickEnabled) {
                this.inputBox.focus();
            }
            this.validate();
        }));
        this.caseSensitive = this._register(new CaseSensitiveToggle({
            appendTitle: appendCaseSensitiveLabel,
            isChecked: false,
            inputActiveOptionBorder: this.inputActiveOptionBorder,
            inputActiveOptionForeground: this.inputActiveOptionForeground,
            inputActiveOptionBackground: this.inputActiveOptionBackground
        }));
        this._register(this.caseSensitive.onChange(viaKeyboard => {
            this._onDidOptionChange.fire(viaKeyboard);
            if (!viaKeyboard && this.fixFocusOnOptionClickEnabled) {
                this.inputBox.focus();
            }
            this.validate();
        }));
        this._register(this.caseSensitive.onKeyDown(e => {
            this._onCaseSensitiveKeyDown.fire(e);
        }));
        // Arrow-Key support to navigate between options
        const indexes = [this.caseSensitive.domNode, this.wholeWords.domNode, this.regex.domNode];
        this.onkeydown(this.domNode, (event) => {
            if (event.equals(15 /* KeyCode.LeftArrow */) || event.equals(17 /* KeyCode.RightArrow */) || event.equals(9 /* KeyCode.Escape */)) {
                const index = indexes.indexOf(document.activeElement);
                if (index >= 0) {
                    let newIndex = -1;
                    if (event.equals(17 /* KeyCode.RightArrow */)) {
                        newIndex = (index + 1) % indexes.length;
                    }
                    else if (event.equals(15 /* KeyCode.LeftArrow */)) {
                        if (index === 0) {
                            newIndex = indexes.length - 1;
                        }
                        else {
                            newIndex = index - 1;
                        }
                    }
                    if (event.equals(9 /* KeyCode.Escape */)) {
                        indexes[index].blur();
                        this.inputBox.focus();
                    }
                    else if (newIndex >= 0) {
                        indexes[newIndex].focus();
                    }
                    dom.EventHelper.stop(event, true);
                }
            }
        });
        this.controls = document.createElement('div');
        this.controls.className = 'controls';
        this.controls.style.display = this._showOptionButtons ? 'block' : 'none';
        this.controls.appendChild(this.caseSensitive.domNode);
        this.controls.appendChild(this.wholeWords.domNode);
        this.controls.appendChild(this.regex.domNode);
        if (!this._showOptionButtons) {
            this.caseSensitive.domNode.style.display = 'none';
            this.wholeWords.domNode.style.display = 'none';
            this.regex.domNode.style.display = 'none';
        }
        for (const toggle of (_a = options === null || options === void 0 ? void 0 : options.additionalToggles) !== null && _a !== void 0 ? _a : []) {
            this._register(toggle);
            this.controls.appendChild(toggle.domNode);
            this._register(toggle.onChange(viaKeyboard => {
                this._onDidOptionChange.fire(viaKeyboard);
                if (!viaKeyboard && this.fixFocusOnOptionClickEnabled) {
                    this.inputBox.focus();
                }
            }));
            this.additionalToggles.push(toggle);
        }
        if (this.additionalToggles.length > 0) {
            this.controls.style.display = 'block';
        }
        this.inputBox.paddingRight =
            (this._showOptionButtons ? this.caseSensitive.width() + this.wholeWords.width() + this.regex.width() : 0)
                + this.additionalToggles.reduce((r, t) => r + t.width(), 0);
        this.domNode.appendChild(this.controls);
        parent === null || parent === void 0 ? void 0 : parent.appendChild(this.domNode);
        this._register(dom.addDisposableListener(this.inputBox.inputElement, 'compositionstart', (e) => {
            this.imeSessionInProgress = true;
        }));
        this._register(dom.addDisposableListener(this.inputBox.inputElement, 'compositionend', (e) => {
            this.imeSessionInProgress = false;
            this._onInput.fire();
        }));
        this.onkeydown(this.inputBox.inputElement, (e) => this._onKeyDown.fire(e));
        this.onkeyup(this.inputBox.inputElement, (e) => this._onKeyUp.fire(e));
        this.oninput(this.inputBox.inputElement, (e) => this._onInput.fire());
        this.onmousedown(this.inputBox.inputElement, (e) => this._onMouseDown.fire(e));
    }
    get onDidChange() {
        return this.inputBox.onDidChange;
    }
    enable() {
        this.domNode.classList.remove('disabled');
        this.inputBox.enable();
        this.regex.enable();
        this.wholeWords.enable();
        this.caseSensitive.enable();
        for (const toggle of this.additionalToggles) {
            toggle.enable();
        }
    }
    disable() {
        this.domNode.classList.add('disabled');
        this.inputBox.disable();
        this.regex.disable();
        this.wholeWords.disable();
        this.caseSensitive.disable();
        for (const toggle of this.additionalToggles) {
            toggle.disable();
        }
    }
    setFocusInputOnOptionClick(value) {
        this.fixFocusOnOptionClickEnabled = value;
    }
    setEnabled(enabled) {
        if (enabled) {
            this.enable();
        }
        else {
            this.disable();
        }
    }
    getValue() {
        return this.inputBox.value;
    }
    setValue(value) {
        if (this.inputBox.value !== value) {
            this.inputBox.value = value;
        }
    }
    style(styles) {
        this.inputActiveOptionBorder = styles.inputActiveOptionBorder;
        this.inputActiveOptionForeground = styles.inputActiveOptionForeground;
        this.inputActiveOptionBackground = styles.inputActiveOptionBackground;
        this.inputBackground = styles.inputBackground;
        this.inputForeground = styles.inputForeground;
        this.inputBorder = styles.inputBorder;
        this.inputValidationInfoBackground = styles.inputValidationInfoBackground;
        this.inputValidationInfoForeground = styles.inputValidationInfoForeground;
        this.inputValidationInfoBorder = styles.inputValidationInfoBorder;
        this.inputValidationWarningBackground = styles.inputValidationWarningBackground;
        this.inputValidationWarningForeground = styles.inputValidationWarningForeground;
        this.inputValidationWarningBorder = styles.inputValidationWarningBorder;
        this.inputValidationErrorBackground = styles.inputValidationErrorBackground;
        this.inputValidationErrorForeground = styles.inputValidationErrorForeground;
        this.inputValidationErrorBorder = styles.inputValidationErrorBorder;
        this.applyStyles();
    }
    applyStyles() {
        if (this.domNode) {
            const toggleStyles = {
                inputActiveOptionBorder: this.inputActiveOptionBorder,
                inputActiveOptionForeground: this.inputActiveOptionForeground,
                inputActiveOptionBackground: this.inputActiveOptionBackground,
            };
            this.regex.style(toggleStyles);
            this.wholeWords.style(toggleStyles);
            this.caseSensitive.style(toggleStyles);
            for (const toggle of this.additionalToggles) {
                toggle.style(toggleStyles);
            }
            const inputBoxStyles = {
                inputBackground: this.inputBackground,
                inputForeground: this.inputForeground,
                inputBorder: this.inputBorder,
                inputValidationInfoBackground: this.inputValidationInfoBackground,
                inputValidationInfoForeground: this.inputValidationInfoForeground,
                inputValidationInfoBorder: this.inputValidationInfoBorder,
                inputValidationWarningBackground: this.inputValidationWarningBackground,
                inputValidationWarningForeground: this.inputValidationWarningForeground,
                inputValidationWarningBorder: this.inputValidationWarningBorder,
                inputValidationErrorBackground: this.inputValidationErrorBackground,
                inputValidationErrorForeground: this.inputValidationErrorForeground,
                inputValidationErrorBorder: this.inputValidationErrorBorder
            };
            this.inputBox.style(inputBoxStyles);
        }
    }
    select() {
        this.inputBox.select();
    }
    focus() {
        this.inputBox.focus();
    }
    getCaseSensitive() {
        return this.caseSensitive.checked;
    }
    setCaseSensitive(value) {
        this.caseSensitive.checked = value;
    }
    getWholeWords() {
        return this.wholeWords.checked;
    }
    setWholeWords(value) {
        this.wholeWords.checked = value;
    }
    getRegex() {
        return this.regex.checked;
    }
    setRegex(value) {
        this.regex.checked = value;
        this.validate();
    }
    focusOnCaseSensitive() {
        this.caseSensitive.focus();
    }
    highlightFindOptions() {
        this.domNode.classList.remove('highlight-' + (this._lastHighlightFindOptions));
        this._lastHighlightFindOptions = 1 - this._lastHighlightFindOptions;
        this.domNode.classList.add('highlight-' + (this._lastHighlightFindOptions));
    }
    validate() {
        this.inputBox.validate();
    }
    showMessage(message) {
        this.inputBox.showMessage(message);
    }
    clearMessage() {
        this.inputBox.hideMessage();
    }
}
